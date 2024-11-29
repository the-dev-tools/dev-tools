package flyclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"the-dev-tools/platform/pkg/machine"
	"the-dev-tools/platform/pkg/machine/flymachine"
	"time"
)

const default_timeout = 10 * time.Second

type Fly struct {
	BaseURL url.URL
	AppName string
	token   string
	client  *http.Client
}

func New(token, appName string, public bool) *Fly {
	httpClient := http.DefaultClient
	httpClient.Timeout = default_timeout
	client := &Fly{token: token, client: http.DefaultClient, AppName: appName}
	if public {
		client.BaseURL = url.URL{
			Scheme: "https",
			Host:   "api.machines.dev:443",
		}
	} else {
		client.BaseURL = url.URL{
			Scheme: "http",
			Host:   "api.internal:4280",
		}
	}

	return client
}

func (f *Fly) GetMachines() ([]flymachine.FlyMachine, error) {
	reqURL := f.BaseURL
	reqURL.Path = fmt.Sprintf("/v1/apps/%s/machines", f.AppName)
	req, err := http.NewRequest("GET", reqURL.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", f.token))
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}

	machines := []flymachine.FlyMachine{}
	err = json.NewDecoder(resp.Body).Decode(&machines)
	if err != nil {
		return nil, err
	}
	return machines, nil
}

func (f *Fly) GetMachine(id string) (machine.Machine, error) {
	reqURL := f.BaseURL
	reqURL.Path = fmt.Sprintf("/v1/apps/%s/machines/%s", f.AppName, id)

	req, err := http.NewRequest("GET", reqURL.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", f.token))
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("machine not found")
	}

	machine := flymachine.FlyMachine{}
	err = json.NewDecoder(resp.Body).Decode(&machine)
	if err != nil {
		return nil, err
	}
	return &machine, nil
}

func (f *Fly) CreateMachine(data machine.Machine) (machine.Machine, error) {
	machineJSON, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	reqURL := f.BaseURL
	reqURL.Path = fmt.Sprintf("/v1/apps/%s/machines", f.AppName)
	req, err := http.NewRequest("POST", reqURL.String(), bytes.NewBuffer(machineJSON))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", f.token))
	req.Header.Add("Content-Type", "application/json")
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("cannot read the body")
		}
		return nil, fmt.Errorf("machine not created statuscode: %d %s", resp.StatusCode, string(body))
	}

	machine := flymachine.FlyMachine{}
	err = json.NewDecoder(resp.Body).Decode(&machine)
	if err != nil {
		return nil, err
	}
	return &machine, nil
}

// INFO: need to this way cuz currently golang cannot understand interface slice
func (f *Fly) CreateMachines(datas []*flymachine.FlyMachine) ([]machine.Machine, error) {
	var machines []machine.Machine

	for _, data := range datas {
		machine, err := f.CreateMachine(data)
		if err != nil {
			for _, m := range machines {
				_ = f.DeleteMachine(m.GetID(), true)
			}
			return nil, err
		}
		machines = append(machines, machine)
	}
	return machines, nil
}

func (f *Fly) DeleteMachine(id string, force bool) error {
	reqURL := f.BaseURL
	reqURL.Path = fmt.Sprintf("/v1/apps/%s/machines/%s", f.AppName, id)
	q := reqURL.Query()
	q.Add("force", fmt.Sprintf("%t", force))
	reqURL.RawQuery = q.Encode()
	req, err := http.NewRequest("DELETE", reqURL.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", f.token))
	resp, err := f.client.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		bodyRaw, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		str := string(bodyRaw)
		return fmt.Errorf("status code: %d body: %s", resp.StatusCode, str)
	}
	return nil
}

func (f *Fly) UpdateMachine(data machine.Machine) (machine.Machine, error) {
	machineJSON, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	reqURL := f.BaseURL
	reqURL.Path = fmt.Sprintf("/v1/apps/%s/machines/%s", f.AppName, data.GetID())
	req, err := http.NewRequest("PUT", reqURL.String(), bytes.NewBuffer(machineJSON))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", f.token))
	req.Header.Add("Content-Type", "application/json")
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("machine not updated")
	}
	machine := flymachine.FlyMachine{}
	err = json.NewDecoder(resp.Body).Decode(&machine)
	if err != nil {
		return nil, err
	}
	return &machine, nil
}

func (f *Fly) WaitMachine(id, instanceID string, timeout time.Duration, state flymachine.State) error {
	reqURL := f.BaseURL
	reqURL.Path = fmt.Sprintf("/v1/apps/%s/machines/%s/wait", f.AppName, id)
	u := reqURL.Query()
	u.Add("timeout", fmt.Sprint(timeout.Seconds()))
	u.Add("state", state.String())
	u.Add("instance_id", instanceID)
	reqURL.RawQuery = u.Encode()
	req, err := http.NewRequest("GET", reqURL.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", f.token))

	resp, err := f.client.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("machine not in state %s", state.String())
	}
	return nil
}

func (f *Fly) LeaseMachine(id string, duration time.Duration) error {
	reqURL := f.BaseURL
	reqURL.Path = fmt.Sprintf("/v1/apps/%s/machines/%s/lease", f.AppName, id)
	u := reqURL.Query()
	u.Add("duration", duration.String())
	reqURL.RawQuery = u.Encode()
	req, err := http.NewRequest("GET", reqURL.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", f.token))
	resp, err := f.client.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("machine not leased")
	}
	return nil
}

func (f *Fly) ReleaseMachine(id string) error {
	reqURL := f.BaseURL
	reqURL.Path = fmt.Sprintf("/v1/apps/%s/machines/%s/lease", f.AppName, id)
	req, err := http.NewRequest("DELETE", reqURL.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", f.token))
	resp, err := f.client.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("machine not released")
	}
	return nil
}

func (f *Fly) GetMetaDataMachine(id string) (map[string]string, error) {
	reqURL := f.BaseURL
	reqURL.Path = fmt.Sprintf("/v1/apps/%s/machines/%s/metadata", f.AppName, id)
	req, err := http.NewRequest("GET", reqURL.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", f.token))
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("machine not found")
	}
	metadata := map[string]string{}
	err = json.NewDecoder(resp.Body).Decode(&metadata)
	if err != nil {
		return nil, err
	}
	return metadata, nil
}

func (f *Fly) SetMetaDataMachine(id string, metadata map[string]string) error {
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	reqURL := f.BaseURL
	reqURL.Path = fmt.Sprintf("/v1/apps/%s/machines/%s/metadata", f.AppName, id)
	req, err := http.NewRequest("POST", reqURL.String(), bytes.NewBuffer(metadataJSON))
	if err != nil {
		return err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", f.token))
	req.Header.Add("Content-Type", "application/json")
	resp, err := f.client.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("metadata not set")
	}
	return nil
}

func (f *Fly) DeleteMetaDataMachine(id string, key string) error {
	reqURL := f.BaseURL
	reqURL.Path = fmt.Sprintf("/v1/apps/%s/machines/%s/metadata/%s", f.AppName, id, key)
	req, err := http.NewRequest("DELETE", reqURL.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", f.token))
	resp, err := f.client.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("metadata not deleted")
	}
	return nil
}

func (f *Fly) StopMachine(id string) error {
	reqURL := f.BaseURL
	reqURL.Path = fmt.Sprintf("/v1/apps/%s/machines/%s/stop", f.AppName, id)
	req, err := http.NewRequest("POST", reqURL.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", f.token))
	resp, err := f.client.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		bodyRaw, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		str := string(bodyRaw)
		return fmt.Errorf("status code: %d body: %s", resp.StatusCode, str)
	}
	return nil
}

func (f *Fly) StartMachine(id string) error {
	reqURL := f.BaseURL
	reqURL.Path = fmt.Sprintf("/v1/apps/%s/machines/%s/start", f.AppName, id)
	req, err := http.NewRequest("POST", reqURL.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", f.token))
	resp, err := f.client.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("machine not started")
	}
	return nil
}
