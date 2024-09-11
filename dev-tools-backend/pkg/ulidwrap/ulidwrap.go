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

func GetTimeFromULID(ulid ulid.ULID) time.Time {
	// Get the time from the ULID
	return time.UnixMilli(int64(ulid.Time()))
}

func GetUlid(Ulid ULIDWrap) ulid.ULID {
	return *Ulid.ulid
}
