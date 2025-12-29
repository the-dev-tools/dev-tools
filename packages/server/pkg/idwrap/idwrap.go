//nolint:revive // exported
package idwrap

import (
	"crypto/rand"
	"database/sql/driver"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
)

var (
	monotonicEntropy   = ulid.Monotonic(rand.Reader, 10000)
	monotonicEntropyMu sync.Mutex
)

type IDWrap struct {
	ulid ulid.ULID `yaml:"binary_data"`
}

func New(ulid ulid.ULID) IDWrap {
	return IDWrap{ulid: ulid}
}

func NewNow() IDWrap {
	return IDWrap{ulid: ulid.Make()}
}

// NewMonotonic generates a ULID that is guaranteed to be greater than the previous one
// if generated within the same monotonic horizon (1 second).
func NewMonotonic() IDWrap {
	monotonicEntropyMu.Lock()
	defer monotonicEntropyMu.Unlock()
	id := ulid.MustNew(ulid.Timestamp(time.Now()), monotonicEntropy)
	return IDWrap{ulid: id}
}

// MarshalYAML implements the yaml.Marshaler interface.
func (id IDWrap) MarshalYAML() (interface{}, error) {
	return id.ulid.String(), nil
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (id *IDWrap) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var value string
	if err := unmarshal(&value); err != nil {
		return err
	}

	parsed, err := ulid.Parse(value)
	if err != nil {
		return err
	}

	id.ulid = parsed
	return nil
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
	return time.UnixMilli(int64(idwrap.ulid.Time())) // nolint:gosec // G115
}

func GetUnixMilliFromULID(idwrap IDWrap) int64 {
	return int64(idwrap.ulid.Time()) // nolint:gosec // G115
}

func GetUlid(id IDWrap) ulid.ULID {
	return id.ulid
}
