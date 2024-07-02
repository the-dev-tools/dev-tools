package flyclient

import (
	"encoding/json"
	"fmt"
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
	json.NewDecoder(resp.Body).Decode(&machines)
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
	json.NewDecoder(resp.Body).Decode(&machine)
	return &machine, nil
}
