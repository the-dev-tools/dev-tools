package flyclient_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/DevToolsGit/devtools-platform/pkg/client/flyclient"
)

func TestFlyClient(t *testing.T) {
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

	machines, err := flyClient.GetMachines()
	if err != nil {
		t.Errorf("GetMachines() returned error: %v", err)
	}

	for _, machine := range machines {
		machine, err := flyClient.GetMachine(machine.ID)
		if err != nil {
			t.Errorf("GetMachine() returned error: %v", err)
		}
		fmt.Println("machine:", machine)
	}

	fmt.Println("machines:", machines)
}
