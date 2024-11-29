package idwrap

import (
	"database/sql/driver"
	"time"

	"github.com/oklog/ulid/v2"
)

type IDWrap struct {
	ulid ulid.ULID
}

func New(ulid ulid.ULID) IDWrap {
	return IDWrap{ulid: ulid}
}

func NewNow() IDWrap {
	return IDWrap{ulid: ulid.Make()}
}

func NewText(ulidString string) (IDWrap, error) {
	ulid, err := ulid.Parse(ulidString)
	if err != nil {
		return IDWrap{}, err
	}
	return IDWrap{ulid: ulid}, nil
}

func NewTextMust(ulidString string) IDWrap {
	ulid, err := ulid.Parse(ulidString)
	if err != nil {
		panic(err)
	}
	return IDWrap{ulid: ulid}
}

func NewFromBytes(data []byte) (IDWrap, error) {
	ulidData := ulid.ULID{}
	err := ulidData.UnmarshalBinary(data)
	return IDWrap{ulid: ulidData}, err
}

func NewFromBytesMust(data []byte) IDWrap {
	ulidData := ulid.ULID{}
	err := ulidData.UnmarshalBinary(data)
	if err != nil {
		panic(err)
	}
	return IDWrap{ulid: ulidData}
}

func (u IDWrap) String() string {
	return u.ulid.String()
}

func (u IDWrap) GetUlid() ulid.ULID {
	return u.ulid
}

func (u IDWrap) Bytes() []byte {
	return u.ulid[:]
}

func (u IDWrap) Compare(id IDWrap) int {
	return u.ulid.Compare(id.ulid)
}

func (u IDWrap) Time() time.Time {
	return GetTimeFromULID(u)
}

// SQL driver value
func (u IDWrap) Value() (driver.Value, error) {
	return u.ulid.Value()
}

func (u *IDWrap) Scan(value interface{}) error {
	return u.ulid.UnmarshalBinary(value.([]byte))
}

func GetTimeFromULID(idwrap IDWrap) time.Time {
	// Get the time from the ULID
	return time.UnixMilli(int64(idwrap.ulid.Time()))
}

func GetUnixMilliFromULID(idwrap IDWrap) int64 {
	return int64(idwrap.ulid.Time())
}

func GetUlid(Ulid IDWrap) ulid.ULID {
	return Ulid.ulid
}
