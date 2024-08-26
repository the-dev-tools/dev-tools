package emailclient

import (
	"bytes"
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"
)

var GlobalEmailClient *EmailClient

const (
	DefaultEmailFrom = "verify@dev.tools"
)

type EmailClient struct {
	sesClient *sesv2.Client
}

const (
	AWSRegionFrankfurt = "eu-central-1"
)

func NewClient(accessKey, secretKey, session string) (*EmailClient, error) {
	if accessKey == "" || secretKey == "" {
		return nil, fmt.Errorf("accessKey or secretKey is empty")
	}

	cred := credentials.NewStaticCredentialsProvider(accessKey, secretKey, session)

	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithCredentialsProvider(cred), config.WithRegion("eu-central-1"))
	if err != nil {
		return nil, err
	}

	client := sesv2.NewFromConfig(cfg)

	emailClient := &EmailClient{
		sesClient: client,
	}

	GlobalEmailClient = emailClient
	return emailClient, nil
}

func SendEmailText(client EmailClient, subject, data, from string, recipients []string) (*sesv2.SendEmailOutput, error) {
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
		return nil, err
	}

	return output, nil
}

func SendEmailHTML(client EmailClient, subject string, data *bytes.Buffer, from string, recipients []string) (*sesv2.SendEmailOutput, error) {
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
		return nil, err
	}

	return output, nil
}
