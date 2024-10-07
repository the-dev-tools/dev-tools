package massertres

import "dev-tools-backend/pkg/idwrap"

type AssertResult struct {
	ID       idwrap.IDWrap
	AssertID idwrap.IDWrap
	Result   bool
	Value    string
}
