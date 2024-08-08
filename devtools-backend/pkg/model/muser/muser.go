package muser

import "github.com/oklog/ulid/v2"

type OAuthType int

var (
	NoOauth   OAuthType = 0
	MagicLink OAuthType = 1
	Google    OAuthType = 2
)

type User struct {
	ID        ulid.ULID
	OrgID     ulid.ULID
	Email     string
	Password  []byte
	OAuthType OAuthType
	OAuthID   string
}
