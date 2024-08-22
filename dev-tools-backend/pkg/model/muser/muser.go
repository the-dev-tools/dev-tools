package muser

import (
	"github.com/oklog/ulid/v2"
)

type ProviderType int

var (
	Unknown    ProviderType = 0
	NoProvider ProviderType = 1
	MagicLink  ProviderType = 2
	Google     ProviderType = 3
)

type UserStatus int

var (
	Active  UserStatus = 0
	Pending UserStatus = 1
	Blocked UserStatus = 2
)

type User struct {
	ID           ulid.ULID
	Email        string
	Password     []byte
	ProviderType ProviderType
	ProviderID   *string
	Status       UserStatus
}
