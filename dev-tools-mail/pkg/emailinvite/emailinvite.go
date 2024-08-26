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
	template *template.Template
}

func NewEmailTemplateFile(path string) (*EmailTemplateManager, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	str := string(data)

	tmpl, err := template.New("emailinvite").Parse(str)
	if err != nil {
		return nil, err
	}

	return &EmailTemplateManager{template: tmpl}, nil
}

func (f EmailTemplateManager) SendEmailInvite(ctx context.Context, client emailclient.EmailClient, to string, data *EmailInviteTemplateData) error {
	// INFO: place holder data for the email template
	buf := new(bytes.Buffer)
	err := f.template.Execute(buf, data)
	if err != nil {
		return err
	}

	output, err := emailclient.SendEmailHTML(client, "Invitation to DevTools", buf, emailclient.DefaultEmailFrom, []string{to})
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
