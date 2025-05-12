package tassert_test

/*
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
	testutil.Assert(t, "testValue", result.Condition.Comparison.Right)
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
	testutil.Assert(t, "testValue", result.Condition.Comparison.Right)
}

func TestSerializeAssertModelToRPCEmpty(t *testing.T) {
	a := massert.Assert{
		ID:    idwrap.NewNow(),
		Path:  "",
		Value: "testValue",
		Type:  massert.AssertTypeEqual,
	}

	result, err := tassert.SerializeAssertModelToRPC(a)
	testutil.Assert(t, nil, err)
	testutil.AssertNot(t, nil, result)
	testutil.Assert(t, true, bytes.Equal(a.ID.Bytes(), result.AssertId))
	pathSize := len(result.Condition.Comparison.Left)
	testutil.Assert(t, pathSize, 0)
	testutil.Assert(t, "testValue", result.Condition.Comparison.Right)
}
*/
