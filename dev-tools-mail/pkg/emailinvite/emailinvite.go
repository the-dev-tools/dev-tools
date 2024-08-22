package emailinvite

import (
	"bufio"
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

func SendEmailInvite(ctx context.Context, client emailclient.EmailClient, to string, inviteLink string) error {
	path := os.Getenv("EMAIL_INVITE_TEMPLATE_PATH")
	if path == "" {
		path = "emailinvite.html"
	}

	template, err := template.ParseFiles(path)
	if err != nil {
		return err
	}
	// INFO: place holder data for the email template
	data := EmailInviteTemplateData{
		WorkspaceName:     "DevTools",
		InviteLink:        inviteLink,
		InvitedByUsername: "Ege",
		Username:          "Mustafa",
	}

	var buffer bytes.Buffer
	writer := bufio.NewWriter(&buffer)
	err = template.Execute(writer, data)
	if err != nil {
		return err
	}

	output, err := emailclient.SendEmailHTML(client, "Invitation to DevTools", buffer.String(), emailclient.DefaultEmailFrom, []string{to})
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
