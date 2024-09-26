package svar_test

import (
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/mvar"
	"dev-tools-backend/pkg/service/svar"
	"fmt"
	"testing"
)

func TestMergeVars(t *testing.T) {
	// TestMergeVars tests the mergeVars function
	// when the input is a slice of variables with no duplicates
	a := []mvar.Var{}
	const aSize = 10

	for i := 0; i < aSize; i++ {
		a = append(a, mvar.Var{
			ID:     idwrap.NewNow(),
			VarKey: fmt.Sprintf("key_%d", i),
			EnvID:  idwrap.NewNow(),
			Value:  fmt.Sprintf("value_%d", i),
		})
	}

	b := []mvar.Var{}
	const bNonDupe = 10
	const bSize = bNonDupe + aSize

	for i := aSize; i < bSize; i++ {
		b = append(b, mvar.Var{
			ID:     idwrap.NewNow(),
			VarKey: fmt.Sprintf("key_%d", i),
			EnvID:  idwrap.NewNow(),
			Value:  fmt.Sprintf("value_%d", i),
		})
	}

	c := svar.MergeVars(a, b)
	const expectedSize = aSize + bNonDupe
	if len(c) != expectedSize {
		t.Errorf("Expected size of %d, got %d", expectedSize, len(c))
	}
}
