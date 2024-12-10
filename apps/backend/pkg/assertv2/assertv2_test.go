package assertv2_test

import (
	"context"
	"testing"
	"the-dev-tools/backend/pkg/assertv2"
	"the-dev-tools/backend/pkg/assertv2/leafs/leafmock"
	"the-dev-tools/backend/pkg/testutil"
)

func TestAssertSys_Eval_EqualTrue(t *testing.T) {
	testAssertValue := 15
	castedAssertValue := interface{}(testAssertValue)

	rootLeaf := &leafmock.LeafMock{}
	subLeaf1 := &leafmock.LeafMock{}
	subLeaf2 := &leafmock.LeafMock{}
	rootLeaf.Leafs = map[string]interface{}{
		"abc": subLeaf1,
	}
	subLeaf1.Leafs = map[string]interface{}{
		"def": subLeaf2,
	}
	subLeaf2.Leafs = map[string]interface{}{
		"ghi": castedAssertValue,
	}

	root := assertv2.NewAssertRoot(rootLeaf)
	assertSys := assertv2.NewAssertSystem(root)
	ctx := context.Background()

	ok, err := assertSys.EvalBool(ctx, "abc.def.ghi == 15")
	testutil.Assert(t, nil, err)
	testutil.Assert(t, true, ok)
}

func TestAssertSys_Eval_EqualFalse(t *testing.T) {
	testAssertValue := 15
	castedAssertValue := interface{}(testAssertValue)

	rootLeaf := &leafmock.LeafMock{}
	subLeaf1 := &leafmock.LeafMock{}
	subLeaf2 := &leafmock.LeafMock{}
	rootLeaf.Leafs = map[string]interface{}{
		"abc": subLeaf1,
	}
	subLeaf1.Leafs = map[string]interface{}{
		"def": subLeaf2,
	}
	subLeaf2.Leafs = map[string]interface{}{
		"ghi": castedAssertValue,
	}

	root := assertv2.NewAssertRoot(rootLeaf)
	assertSys := assertv2.NewAssertSystem(root)
	ctx := context.Background()

	ok, err := assertSys.EvalBool(ctx, "abc.def.ghi == 14")
	testutil.Assert(t, nil, err)
	testutil.Assert(t, false, ok)
}

func TestAssertSys_Eval_InTrue(t *testing.T) {
	testAssertValue := 15
	castedAssertValue := interface{}(testAssertValue)
	rootLeaf := &leafmock.LeafMock{}
	rootLeaf.Leafs = map[string]interface{}{
		"a":     castedAssertValue,
		"array": []interface{}{15, 16, 17},
	}

	root := assertv2.NewAssertRoot(rootLeaf)
	assertSys := assertv2.NewAssertSystem(root)
	ctx := context.Background()

	ok, err := assertSys.EvalBool(ctx, "a in array")
	testutil.Assert(t, nil, err)
	testutil.Assert(t, true, ok)
}

func TestAssertSys_Eval_InFalse(t *testing.T) {
	testAssertValue := 14
	castedAssertValue := interface{}(testAssertValue)
	rootLeaf := &leafmock.LeafMock{}
	rootLeaf.Leafs = map[string]interface{}{
		"a":     castedAssertValue,
		"array": []interface{}{15, 16, 17},
	}

	root := assertv2.NewAssertRoot(rootLeaf)
	assertSys := assertv2.NewAssertSystem(root)
	ctx := context.Background()

	ok, err := assertSys.EvalBool(ctx, "a in array")
	testutil.Assert(t, nil, err)
	testutil.Assert(t, false, ok)
}

func TestAssert_Simple_Eval_InTrue(t *testing.T) {
	testAssertValue := 15
	castedAssertValue := interface{}(testAssertValue)
	rootLeaf := &leafmock.LeafMock{}
	rootLeaf.Leafs = map[string]interface{}{
		"a":     castedAssertValue,
		"array": []interface{}{15, 16, 17},
	}

	root := assertv2.NewAssertRoot(rootLeaf)
	assertSys := assertv2.NewAssertSystem(root)
	ctx := context.Background()

	ok, err := assertSys.AssertSimple(ctx, assertv2.AssertTypeContains, "array", castedAssertValue)
	testutil.Assert(t, nil, err)
	testutil.Assert(t, true, ok)
}

func TestAssert_Simple_Eval_InFalse(t *testing.T) {
	testAssertValue := 14
	castedAssertValue := interface{}(testAssertValue)
	rootLeaf := &leafmock.LeafMock{}
	rootLeaf.Leafs = map[string]interface{}{
		"a":     castedAssertValue,
		"array": []interface{}{15, 16, 17},
	}

	root := assertv2.NewAssertRoot(rootLeaf)
	assertSys := assertv2.NewAssertSystem(root)
	ctx := context.Background()

	ok, err := assertSys.AssertSimple(ctx, assertv2.AssertTypeContains, "array", castedAssertValue)
	testutil.Assert(t, nil, err)
	testutil.Assert(t, false, ok)
}

func TestAssert_Simple_Eval_NotInTrue(t *testing.T) {
	testAssertValue := 14
	castedAssertValue := interface{}(testAssertValue)
	rootLeaf := &leafmock.LeafMock{}
	rootLeaf.Leafs = map[string]interface{}{
		"a":     castedAssertValue,
		"array": []interface{}{15, 16, 17},
	}

	root := assertv2.NewAssertRoot(rootLeaf)
	assertSys := assertv2.NewAssertSystem(root)
	ctx := context.Background()

	ok, err := assertSys.AssertSimple(ctx, assertv2.AssertTypeNotContains, "array", castedAssertValue)
	testutil.Assert(t, nil, err)
	testutil.Assert(t, true, ok)
}

func TestAssert_Simple_Eval_NotInFalse(t *testing.T) {
	testAssertValue := 15
	castedAssertValue := interface{}(testAssertValue)
	rootLeaf := &leafmock.LeafMock{}
	rootLeaf.Leafs = map[string]interface{}{
		"a":     castedAssertValue,
		"array": []interface{}{15, 16, 17},
	}

	root := assertv2.NewAssertRoot(rootLeaf)
	assertSys := assertv2.NewAssertSystem(root)
	ctx := context.Background()

	ok, err := assertSys.AssertSimple(ctx, assertv2.AssertTypeNotContains, "array", castedAssertValue)
	testutil.Assert(t, nil, err)
	testutil.Assert(t, false, ok)
}
