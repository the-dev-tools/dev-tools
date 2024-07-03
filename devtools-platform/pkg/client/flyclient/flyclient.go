package flyclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/DevToolsGit/devtools-platform/pkg/machine"
	"github.com/DevToolsGit/devtools-platform/pkg/machine/flymachine"
)

const default_timeout = 10 * time.Second

type Fly struct {
	BaseURL string
	AppName string
	token   string
	client  *http.Client
}

func New(token, appName string, public bool) *Fly {
	httpClient := http.DefaultClient
	httpClient.Timeout = default_timeout
	client := &Fly{token: token, client: http.DefaultClient, AppName: appName}
	if public {
		client.BaseURL = "https://api.machines.dev"
	} else {
		client.BaseURL = "http://_api.internal:4280"
	}

	return client
}

func (f *Fly) GetMachines() ([]flymachine.FlyMachine, error) {
	url := fmt.Sprintf("%s/v1/apps/%s/machines", f.BaseURL, f.AppName)
	req, err := http.NewRequest("GET", url, nil)
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
	url := fmt.Sprintf("%s/v1/apps/%s/machines/%s", f.BaseURL, f.AppName, id)
	req, err := http.NewRequest("GET", url, nil)
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

func (f *Fly) CreateMachine(data interface{}) (machine.Machine, error) {
	createMachineReqData, ok := data.(*flymachine.FlyMachineCreateRequest)
	if !ok {
		return nil, fmt.Errorf("invalid machine type")
	}

	machineJSON, err := json.Marshal(createMachineReqData)
	if err != nil {
		return nil, err
	}

	fmt.Println("machineJSON: ", string(machineJSON))

	url := fmt.Sprintf("%s/v1/apps/%s/machines", f.BaseURL, f.AppName)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(machineJSON))
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

func (f *Fly) DeleteMachine(id string, force bool) error {
	url := fmt.Sprintf("%s/v1/apps/%s/machines/%s?force=%t", f.BaseURL, f.AppName, id, force)
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", f.token))
	resp, err := f.client.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("machine not deleted")
	}
	return nil
}
