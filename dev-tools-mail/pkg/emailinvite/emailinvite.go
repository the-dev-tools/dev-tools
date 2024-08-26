package emailinvite

import (
	"bytes"
	"context"
	"dev-tools-mail/pkg/emailclient"
	"fmt"
	"html/template"
	"os"
)

type EmailInviteTemplateData struct {
	WorkspaceName     string
	InviteLink        string
	InvitedByUsername string
	Username          string
}

type EmailTemplateManager struct {
	template    *template.Template
	emailClient *emailclient.EmailClient
}

func NewEmailTemplateFile(path string, client *emailclient.EmailClient) (*EmailTemplateManager, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	str := string(data)
	if client == nil {
		return nil, fmt.Errorf("client is nil")
	}

	tmpl, err := template.New("emailinvite").Parse(str)
	if err != nil {
		return nil, err
	}

	return &EmailTemplateManager{template: tmpl, emailClient: client}, nil
}

func (f EmailTemplateManager) SendEmailInvite(ctx context.Context, to string, data *EmailInviteTemplateData) error {
	// INFO: place holder data for the email template
	buf := new(bytes.Buffer)
	err := f.template.Execute(buf, data)
	if err != nil {
		return err
	}

	output, err := emailclient.SendEmailHTML(f.emailClient, "Invitation to DevTools", buf, emailclient.DefaultEmailFrom, []string{to})
	if err != nil {
		return err
	}
	if output == nil {
		return fmt.Errorf("output is nil")
	}
	if output.MessageId == nil {
		return fmt.Errorf("output.MessageId is nil")
	}

	return nil
}
