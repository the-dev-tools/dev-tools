package mailclient

type EmailData interface {
	SendEmailTo(to string) error
}
