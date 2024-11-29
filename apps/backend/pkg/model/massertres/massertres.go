package massertres

import "the-dev-tools/backend/pkg/idwrap"

type AssertResult struct {
	ID         idwrap.IDWrap
	ResponseID idwrap.IDWrap
	AssertID   idwrap.IDWrap
	Result     bool
}
