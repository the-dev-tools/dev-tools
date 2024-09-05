package ulidwrap

import (
	"time"

	"github.com/oklog/ulid/v2"
)

type ULIDWrap struct {
	ulid *ulid.ULID
}

func GetTimeFromULID(ulid ulid.ULID) time.Time {
	// Get the time from the ULID
	return time.UnixMilli(int64(ulid.Time()))
}
