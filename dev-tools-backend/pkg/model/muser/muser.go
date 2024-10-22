package muser

import "dev-tools-backend/pkg/idwrap"

type ProviderType int8

var (
	Unknown    ProviderType = 0
	NoProvider ProviderType = 1
	MagicLink  ProviderType = 2
	Google     ProviderType = 3
	Local      ProviderType = 16
)

type UserStatus int8

var (
	Active  UserStatus = 0
	Pending UserStatus = 1
	Blocked UserStatus = 2
)

type User struct {
	Email        string
	ProviderID   *string
	Password     []byte
	ProviderType ProviderType
	Status       UserStatus
	ID           idwrap.IDWrap
}
