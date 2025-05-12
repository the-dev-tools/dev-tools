package mcondition

type Condition struct {
	Comparisons Comparison
}

type ComparisonKind int8

type Comparison struct {
	Expression string
}
