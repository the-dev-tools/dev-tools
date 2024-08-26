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

func SendEmailInvite(ctx context.Context, client emailclient.EmailClient, to string, data *EmailInviteTemplateData) error {
	path := os.Getenv("EMAIL_INVITE_TEMPLATE_PATH")
	if path == "" {
		path = "emailinvite.html"
	}

	template, err := template.ParseFiles(path)
	if err != nil {
		return err
	}

	// INFO: place holder data for the email template
	buf := new(bytes.Buffer)
	err = template.Execute(buf, data)
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
