package ulidwrap

import (
	"time"

	"github.com/oklog/ulid/v2"
)

type ULIDWrap struct {
	ulid *ulid.ULID
}

func New(ulid ulid.ULID) ULIDWrap {
	return ULIDWrap{ulid: &ulid}
}

func NewWithParse(ulidString string) (ULIDWrap, error) {
	ulid, err := ulid.Parse(ulidString)
	if err != nil {
		return ULIDWrap{}, err
	}
	return ULIDWrap{ulid: &ulid}, nil
}

func (u ULIDWrap) String() string {
	return u.ulid.String()
}

func (u ULIDWrap) Time() time.Time {
	return GetTimeFromULID(*u.ulid)
}

func (u ULIDWrap) GetUlid() ulid.ULID {
	return *u.ulid
}

func GetTimeFromULID(ulid ulid.ULID) time.Time {
	// Get the time from the ULID
	return time.UnixMilli(int64(ulid.Time()))
}

func GetUlid(Ulid ULIDWrap) ulid.ULID {
	return *Ulid.ulid
}
