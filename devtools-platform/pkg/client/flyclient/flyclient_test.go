package flyclient_test

import (
	"devtools-platform/pkg/client/flyclient"
	"devtools-platform/pkg/machine/flymachine"
	"fmt"
	"os"
	"testing"
	"time"
)

func SetupCreateClient(t testing.TB) *flyclient.Fly {
	t.Helper()

	token := os.Getenv("FLY_API_TOKEN")
	if token == "" {
		t.Skip("FLY_API_TOKEN is not set")
	}

	appName := os.Getenv("FLY_APP_NAME")
	if appName == "" {
		t.Skip("FLY_APP_NAME is not set")
	}

	flyClient := flyclient.New(token, appName, true)
	if flyClient == nil {
		t.Errorf("New() returned nil")
	}

	return flyClient
}

func TestFlyClient(t *testing.T) {
	client := SetupCreateClient(t)

	machines, err := client.GetMachines()
	if err != nil {
		t.Errorf("GetMachines() returned error: %v", err)
	}

	for _, machine := range machines {
		machine, err := client.GetMachine(machine.ID)
		if err != nil {
			t.Errorf("GetMachine() returned error: %v", err)
		}
		t.Logf("machine: %v", machine)
	}
}

func TestFlyClientCreate(t *testing.T) {
	client := SetupCreateClient(t)

	formatedText := time.Now().Format("2006-01-02 15:04:05")

	flymachine := &flymachine.FlyMachine{
		Name:   fmt.Sprintf("test-%s", formatedText),
		Region: flymachine.RegionAmsterdam,
		Config: flymachine.FlyMachineCreateConfig{
			Image: "alpine",
			Env: map[string]string{
				"FOO": "bar",
			},
			Services: []flymachine.FlyMachineService{
				{
					Ports: []flymachine.FlyMachinePortPair{
						{Port: 8090, Handlers: []string{"http"}},
						{Port: 8080, Handlers: []string{"http"}},
					},
					Protocol:     "tcp",
					InternalPort: 80,
				},
			},
		},
	}

	machine, err := client.CreateMachine(flymachine)
	if err != nil {
		t.Errorf("CreateMachine() returned error: %v", err)
	}
	t.Logf("machine: %v", machine)

	err = client.DeleteMachine(machine.GetID(), true)
	if err != nil {
		t.Errorf("DeleteMachine() returned error: %v", err)
	}
}
