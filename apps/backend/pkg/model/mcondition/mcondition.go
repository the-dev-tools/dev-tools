package mcondition

type Condition struct {
	Comparisons Comparison
}

type ComparisonKind int8

const (
	COMPARISON_KIND_UNSPECIFIED ComparisonKind = iota
	COMPARISON_KIND_EQUAL
	COMPARISON_KIND_NOT_EQUAL
	COMPARISON_KIND_CONTAINS
	COMPARISON_KIND_NOT_CONTAINS
	COMPARISON_KIND_GREATER
	COMPARISON_KIND_LESS
	COMPARISON_KIND_GREATER_OR_EQUAL
	COMPARISON_KIND_LESS_OR_EQUAL
)

type Comparison struct {
	Kind  ComparisonKind
	Path  string
	Value string
}
