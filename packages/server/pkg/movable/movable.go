package movable

import (
	"context"
	"database/sql"
	"the-dev-tools/server/pkg/idwrap"
)

// MovePosition represents the position relative to a target item
type MovePosition int

const (
	MovePositionUnspecified MovePosition = iota
	MovePositionAfter
	MovePositionBefore
)

// ListType defines the interface for different list type enums
type ListType interface {
	String() string
	Value() int
}

// Movable defines the interface for items that can be moved in linked lists
type Movable interface {
	// GetID returns the unique identifier of the item
	GetID() idwrap.IDWrap
	
	// GetListTypes returns all list types this item can participate in
	GetListTypes() []ListType
	
	// GetPosition returns the current position of the item in the specified list type
	GetPosition(listType ListType) (int, error)
	
	// SetPosition updates the position of the item in the specified list type
	SetPosition(listType ListType, position int) error
	
	// GetParentID returns the parent container ID for the specified list type
	GetParentID(listType ListType) (*idwrap.IDWrap, error)
}

// MoveOperation represents a move operation request
type MoveOperation struct {
	ItemID       idwrap.IDWrap
	ListType     ListType
	Position     MovePosition
	TargetID     *idwrap.IDWrap
	NewParentID  *idwrap.IDWrap
}

// MoveResult represents the result of a move operation
type MoveResult struct {
	Success      bool
	NewPosition  int
	AffectedIDs  []idwrap.IDWrap
	Error        error
}

// LinkedListManager defines the interface for managing linked list operations
type LinkedListManager interface {
	// Move performs a move operation on an item
	Move(ctx context.Context, tx *sql.Tx, operation MoveOperation) (*MoveResult, error)
	
	// ReorderAfter moves an item to be positioned after the target item
	ReorderAfter(ctx context.Context, tx *sql.Tx, itemID, targetID idwrap.IDWrap, listType ListType) (*MoveResult, error)
	
	// ReorderBefore moves an item to be positioned before the target item
	ReorderBefore(ctx context.Context, tx *sql.Tx, itemID, targetID idwrap.IDWrap, listType ListType) (*MoveResult, error)
	
	// GetItemsInList returns all items in the specified list, ordered by position
	GetItemsInList(ctx context.Context, parentID idwrap.IDWrap, listType ListType) ([]Movable, error)
	
	// ValidateMove checks if a move operation is valid
	ValidateMove(ctx context.Context, operation MoveOperation) error
	
	// CompactPositions recalculates and compacts position values to eliminate gaps
	CompactPositions(ctx context.Context, tx *sql.Tx, parentID idwrap.IDWrap, listType ListType) error
}

// MovableRepository defines the interface for database operations on movable items
type MovableRepository interface {
	// UpdatePosition updates the position of an item in a specific list type
	UpdatePosition(ctx context.Context, tx *sql.Tx, itemID idwrap.IDWrap, listType ListType, position int) error
	
	// UpdatePositions updates positions for multiple items in batch
	UpdatePositions(ctx context.Context, tx *sql.Tx, updates []PositionUpdate) error
	
	// GetMaxPosition returns the maximum position value for a list
	GetMaxPosition(ctx context.Context, parentID idwrap.IDWrap, listType ListType) (int, error)
	
	// GetItemsByParent returns all items under a parent, ordered by position
	GetItemsByParent(ctx context.Context, parentID idwrap.IDWrap, listType ListType) ([]MovableItem, error)
}

// TransactionAwareRepository extends MovableRepository with transaction support
type TransactionAwareRepository interface {
	MovableRepository
	// TX returns a new repository instance with transaction support
	TX(tx *sql.Tx) MovableRepository
}

// PositionUpdate represents a position update operation
type PositionUpdate struct {
	ItemID   idwrap.IDWrap
	ListType ListType
	Position int
}

// MovableItem represents the basic structure of a movable item in the database
type MovableItem struct {
	ID        idwrap.IDWrap
	ParentID  *idwrap.IDWrap
	Position  int
	ListType  ListType
}