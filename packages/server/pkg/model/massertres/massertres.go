//nolint:revive // exported
package massertres

import "the-dev-tools/server/pkg/idwrap"

type AssertResult struct {
	ID         idwrap.IDWrap
	ResponseID idwrap.IDWrap
	AssertID   idwrap.IDWrap
	Result     bool
}
