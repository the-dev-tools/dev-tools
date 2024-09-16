package muser

import "dev-tools-backend/pkg/idwrap"

type ProviderType int8

var (
	Unknown    ProviderType = 0
	NoProvider ProviderType = 1
	MagicLink  ProviderType = 2
	Google     ProviderType = 3
)

type UserStatus int8

var (
	Active  UserStatus = 0
	Pending UserStatus = 1
	Blocked UserStatus = 2
)

type User struct {
	ID           idwrap.IDWrap
	Email        string
	Password     []byte
	ProviderType ProviderType
	ProviderID   *string
	Status       UserStatus
}
