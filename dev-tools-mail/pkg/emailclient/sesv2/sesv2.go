package sesv2

import (
	"bytes"
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"
)

var GlobalEmailClient *sesv2client

const (
	DefaultEmailFrom = "verify@dev.tools"
)

type sesv2client struct {
	sesClient *sesv2.Client
}

const (
	AWSRegionFrankfurt = "eu-central-1"
)

func NewClient(accessKey, secretKey, session string) (*sesv2client, error) {
	if accessKey == "" || secretKey == "" {
		return nil, fmt.Errorf("accessKey or secretKey is empty")
	}

	cred := credentials.NewStaticCredentialsProvider(accessKey, secretKey, session)

	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithCredentialsProvider(cred), config.WithRegion("eu-central-1"))
	if err != nil {
		return nil, err
	}

	client := sesv2.NewFromConfig(cfg)

	emailClient := &sesv2client{
		sesClient: client,
	}

	GlobalEmailClient = emailClient
	return emailClient, nil
}

func (client sesv2client) SendEmailText(subject, data, from string, recipients []string) error {
	// Text body or HTML body
	body := &types.Content{
		Data: &data,
	}

	// Email content structure
	emailContent := &types.Message{
		Body: &types.Body{
			Text: body,
		},
		Subject: &types.Content{
			Data: &subject,
		},
	}

	// Send the email
	emailInput := &sesv2.SendEmailInput{
		FromEmailAddress: &from,
		Destination: &types.Destination{
			ToAddresses: recipients,
		},
		Content: &types.EmailContent{
			Simple: emailContent,
		},
	}

	output, err := client.sesClient.SendEmail(context.Background(), emailInput)
	if err != nil {
		return err
	}
	if output == nil {
		return fmt.Errorf("output is nil")
	}
	return nil
}

func (client sesv2client) SendEmailHTML(subject string, data *bytes.Buffer, from string, recipients []string) error {
	htmlBody := data.String()

	// Text body or HTML body
	body := &types.Content{
		Data: &htmlBody,
	}

	// Email content structure
	emailContent := &types.Message{
		Body: &types.Body{
			Html: body,
		},
		Subject: &types.Content{
			Data: &subject,
		},
	}

	// Send the email
	emailInput := &sesv2.SendEmailInput{
		FromEmailAddress: &from,
		Destination: &types.Destination{
			ToAddresses: recipients,
		},
		Content: &types.EmailContent{
			Simple: emailContent,
		},
	}

	output, err := client.sesClient.SendEmail(context.Background(), emailInput)
	if err != nil {
		return err
	}
	if output == nil {
		return fmt.Errorf("output is nil")
	}

	return nil
}
