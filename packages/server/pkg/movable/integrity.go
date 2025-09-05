package movable

import "the-dev-tools/server/pkg/idwrap"

// Note: The primary integrity checker lives in end.go (CheckListIntegrity) so callers
// have a single import location. This file reserves space for future, more detailed
// integrity utilities if we later add optional pointer-based validations.

// HasID reports whether the given slice contains the id.
func HasID(items []MovableItem, id idwrap.IDWrap) bool {
    for _, it := range items {
        if it.ID.Compare(id) == 0 { return true }
    }
    return false
}

