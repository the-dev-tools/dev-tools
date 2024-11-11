package emailclient

import (
	"bytes"
)

var GlobalEmailClient *EmailClient

// DEFAULTS
const (
	DefaultEmailFrom = "verify@dev.tools"
)

// REGION
const (
	AWSRegionFrankfurt = "eu-central-1"
)

type EmailClient interface {
	SendEmailText(subject, data, from string, recipients []string) error
	SendEmailHTML(subject string, data *bytes.Buffer, from string, recipients []string) error
}
