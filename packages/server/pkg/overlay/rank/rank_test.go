package rank_test

import (
	"testing"
	rank "the-dev-tools/server/pkg/overlay/rank"
)

func TestBetween_MonotonicGrowth(t *testing.T) {
	a := rank.First()
	b := rank.Between(a, "")
	c := rank.Between(b, "")
	if !(a < b && b < c) {
		t.Fatalf("expected a < b < c, got a=%q b=%q c=%q", a, b, c)
	}
}

func TestBetween_MiddleBetweenBounds(t *testing.T) {
	left := rank.Between("", "")
	right := rank.Between(left, "")
	mid := rank.Between(left, right)
	if !(left < mid && mid < right) {
		t.Fatalf("expected left < mid < right, got left=%q mid=%q right=%q", left, mid, right)
	}
}
