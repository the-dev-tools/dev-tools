package nodecom_test

import (
	"testing"
	"the-dev-tools/nodes/pkg/mailclient/mockemailclient"
)

func TestSendEmail(t *testing.T) {
	emailAdress := "ege@dev.tools"
	emailClient := mockemailclient.MockEmailClient{}
	err := emailClient.SendEmailTo(emailAdress)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
