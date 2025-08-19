package movable

import "errors"

// Common errors for movable operations
var (
	ErrItemNotFound       = errors.New("item not found")
	ErrTargetNotFound     = errors.New("target item not found")
	ErrInvalidPosition    = errors.New("invalid position")
	ErrCircularReference  = errors.New("circular reference detected")
	ErrIncompatibleType   = errors.New("incompatible list type")
	ErrEmptyItemID        = errors.New("item ID cannot be empty")
	ErrEmptyTargetID      = errors.New("target ID cannot be empty")
	ErrSelfReference      = errors.New("cannot move item to itself")
	ErrNoParent           = errors.New("item has no parent")
	ErrInvalidListType    = errors.New("invalid list type")
	ErrPositionOutOfRange = errors.New("position out of range")
	ErrDatabaseTransaction = errors.New("database transaction failed")
)