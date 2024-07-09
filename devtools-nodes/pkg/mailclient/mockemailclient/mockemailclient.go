package mockemailclient

type MockEmailClient struct {
	ReturnError error
}

func (m *MockEmailClient) SendEmailTo(to string) error {
	return m.ReturnError
}
