package idwrap_test

import (
	"dev-tools-backend/pkg/idwrap"
	"testing"
)

func TestNew(t *testing.T) {
	a := idwrap.NewNow()
	aInterface, err := a.Value()
	if err != nil {
		t.Error(err)
	}

	aBytes, ok := aInterface.([]byte)
	if !ok {
		t.Error("Value is not []byte")
	}
	if len(aBytes) != 16 {
		t.Error("Value is not 16 bytes")
	}
	a2, err := idwrap.NewFromBytes(aBytes)
	if err != nil {
		t.Error(err)
	}

	if a.Compare(a2) != 0 {
		t.Error("Compare failed")
	}

	a3 := idwrap.NewNow()
	if a.Compare(a3) == 0 {
		t.Error("Compare failed")
	}

	err = a3.Scan(aBytes)
	if err != nil {
		t.Error(err)
	}

	if a.Compare(a3) != 0 {
		t.Error("Compare failed")
	}
}
