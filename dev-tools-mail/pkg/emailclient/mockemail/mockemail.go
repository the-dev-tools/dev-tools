package mockemail

import "bytes"

type MockEmailClient struct{}

func NewMockEmailClient() *MockEmailClient {
	return &MockEmailClient{}
}

func (m MockEmailClient) SendEmailText(subject, data, from string, recipients []string) error {
	return nil
}

func (m MockEmailClient) SendEmailHTML(subject string, data *bytes.Buffer, from string, recipients []string) error {
	return nil
}
