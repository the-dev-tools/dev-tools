package idwrap_test

import (
	"testing"
	"the-dev-tools/server/pkg/idwrap"
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

func TestNewMonotonic(t *testing.T) {
	const count = 1000
	ids := make([]idwrap.IDWrap, count)

	for i := 0; i < count; i++ {
		ids[i] = idwrap.NewMonotonic()
	}

	for i := 1; i < count; i++ {
		if ids[i].Compare(ids[i-1]) <= 0 {
			t.Errorf("ID %d (%s) is not greater than ID %d (%s)", i, ids[i], i-1, ids[i-1])
		}
	}
}

func TestNewMonotonicConcurrency(t *testing.T) {
	const goroutines = 10
	const countPerGoroutine = 100
	idsChan := make(chan idwrap.IDWrap, goroutines*countPerGoroutine)

	for i := 0; i < goroutines; i++ {
		go func() {
			for j := 0; j < countPerGoroutine; j++ {
				idsChan <- idwrap.NewMonotonic()
			}
		}()
	}

	uniqueIDs := make(map[string]bool)
	for i := 0; i < goroutines*countPerGoroutine; i++ {
		id := (<-idsChan).String()
		if uniqueIDs[id] {
			t.Errorf("Duplicate ID generated: %s", id)
		}
		uniqueIDs[id] = true
	}
}

func BenchmarkNewNow(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = idwrap.NewNow()
	}
}
