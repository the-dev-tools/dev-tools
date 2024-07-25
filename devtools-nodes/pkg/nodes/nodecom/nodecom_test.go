package nodecom_test

import (
	"devtools-nodes/pkg/mailclient/mockemailclient"
	"testing"
)

func TestSendEmail(t *testing.T) {
	emailAdress := "ege@dev.tools"
	emailClient := mockemailclient.MockEmailClient{}
	err := emailClient.SendEmailTo(emailAdress)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
