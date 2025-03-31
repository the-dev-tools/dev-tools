package tassert_test

import (
	"bytes"
	"testing"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/massert"
	"the-dev-tools/server/pkg/testutil"
	"the-dev-tools/server/pkg/translate/tassert"
)

func TestSerializeAssertModelToRPC(t *testing.T) {
	a := massert.Assert{
		ID:    idwrap.NewNow(),
		Path:  "key1.key2[0]",
		Value: "testValue",
		Type:  massert.AssertTypeEqual,
	}

	result, err := tassert.SerializeAssertModelToRPC(a)
	testutil.Assert(t, nil, err)
	testutil.AssertNot(t, nil, result)
	testutil.Assert(t, true, bytes.Equal(a.ID.Bytes(), result.AssertId))
	testutil.Assert(t, "testValue", result.Condition.Comparison.Value)
}

func TestSerializeAssertModelToRPCItem(t *testing.T) {
	a := massert.Assert{
		ID:    idwrap.NewNow(),
		Path:  "key1.key2[0]",
		Value: "testValue",
		Type:  massert.AssertTypeEqual,
	}

	result, err := tassert.SerializeAssertModelToRPC(a)
	testutil.Assert(t, nil, err)
	testutil.AssertNot(t, nil, result)
	testutil.Assert(t, true, bytes.Equal(a.ID.Bytes(), result.AssertId))
	testutil.Assert(t, "testValue", result.Condition.Comparison.Value)
}
