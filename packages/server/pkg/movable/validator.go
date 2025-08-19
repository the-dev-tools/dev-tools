package movable

import (
	"context"
	"fmt"
	"the-dev-tools/server/pkg/idwrap"
)

// MoveValidator provides enhanced validation for move operations
type MoveValidator struct {
	repo MovableRepository
}

// NewMoveValidator creates a new MoveValidator
func NewMoveValidator(repo MovableRepository) *MoveValidator {
	return &MoveValidator{
		repo: repo,
	}
}

// ValidateComplexMove performs comprehensive validation for move operations
func (v *MoveValidator) ValidateComplexMove(ctx context.Context, operation MoveOperation) error {
	// Basic validation
	if err := v.validateBasicOperation(operation); err != nil {
		return err
	}

	// Check if item exists and can be moved
	if err := v.validateItemExists(ctx, operation.ItemID, operation.ListType); err != nil {
		return err
	}

	// Validate target if specified
	if operation.TargetID != nil {
		if err := v.validateTargetExists(ctx, *operation.TargetID, operation.ListType); err != nil {
			return err
		}

		// Check for self-reference
		if operation.ItemID == *operation.TargetID {
			return ErrSelfReference
		}

		// Check for circular references if moving between different parents
		if operation.NewParentID != nil {
			if err := v.validateNoCircularReference(ctx, operation.ItemID, *operation.NewParentID, operation.ListType); err != nil {
				return err
			}
		}
	}

	// Validate parent change if specified
	if operation.NewParentID != nil {
		if err := v.validateParentChange(ctx, operation); err != nil {
			return err
		}
	}

	return nil
}

// validateBasicOperation performs basic validation of the operation structure
func (v *MoveValidator) validateBasicOperation(operation MoveOperation) error {
	if operation.ItemID == (idwrap.IDWrap{}) {
		return ErrEmptyItemID
	}

	if operation.ListType == nil {
		return ErrInvalidListType
	}

	switch operation.Position {
	case MovePositionAfter, MovePositionBefore:
		if operation.TargetID == nil {
			return ErrEmptyTargetID
		}
	case MovePositionUnspecified:
		return ErrInvalidPosition
	default:
		return fmt.Errorf("%w: %v", ErrInvalidPosition, operation.Position)
	}

	return nil
}

// validateItemExists checks if the item exists and supports the specified list type
func (v *MoveValidator) validateItemExists(ctx context.Context, itemID idwrap.IDWrap, listType ListType) error {
	items, err := v.repo.GetItemsByParent(ctx, itemID, listType)
	if err != nil {
		return fmt.Errorf("failed to check item existence: %w", err)
	}

	if len(items) == 0 {
		return ErrItemNotFound
	}

	return nil
}

// validateTargetExists checks if the target item exists
func (v *MoveValidator) validateTargetExists(ctx context.Context, targetID idwrap.IDWrap, listType ListType) error {
	items, err := v.repo.GetItemsByParent(ctx, targetID, listType)
	if err != nil {
		return fmt.Errorf("failed to check target existence: %w", err)
	}

	if len(items) == 0 {
		return ErrTargetNotFound
	}

	return nil
}

// validateNoCircularReference checks for circular references when changing parents
func (v *MoveValidator) validateNoCircularReference(ctx context.Context, itemID, newParentID idwrap.IDWrap, listType ListType) error {
	// Simple check: ensure new parent is not a descendant of the item
	// This would need to be implemented based on the specific hierarchy structure
	// For now, we perform a basic check
	
	if itemID == newParentID {
		return ErrCircularReference
	}

	// Additional circular reference checks would go here
	// This would involve traversing the hierarchy to ensure the new parent
	// is not a child/descendant of the item being moved

	return nil
}

// validateParentChange validates that a parent change operation is valid
func (v *MoveValidator) validateParentChange(ctx context.Context, operation MoveOperation) error {
	// Check if the new parent exists and can contain items of this list type
	if operation.NewParentID == nil {
		return nil // No parent change
	}

	// Validate that the new parent can contain items of the specified list type
	// This validation would be specific to the domain model
	// For example, a folder can contain endpoints, but not other types

	return nil
}

// ValidateListTypeCompatibility checks if an item can participate in a specific list type
func (v *MoveValidator) ValidateListTypeCompatibility(itemType string, listType ListType) error {
	// Define compatibility rules based on item type and list type
	switch listType := listType.(type) {
	case CollectionListType:
		return v.validateCollectionListTypeCompatibility(itemType, listType)
	case RequestListType:
		return v.validateRequestListTypeCompatibility(itemType, listType)
	case FlowListType:
		return v.validateFlowListTypeCompatibility(itemType, listType)
	case WorkspaceListType:
		return v.validateWorkspaceListTypeCompatibility(itemType, listType)
	default:
		return ErrIncompatibleType
	}
}

func (v *MoveValidator) validateCollectionListTypeCompatibility(itemType string, listType CollectionListType) error {
	compatibilityMap := map[string][]CollectionListType{
		"folder":     {CollectionListTypeFolders},
		"endpoint":   {CollectionListTypeEndpoints},
		"example":    {CollectionListTypeExamples},
		"collection": {CollectionListTypeCollections},
	}

	allowedTypes, exists := compatibilityMap[itemType]
	if !exists {
		return ErrIncompatibleType
	}

	for _, allowedType := range allowedTypes {
		if allowedType == listType {
			return nil
		}
	}

	return ErrIncompatibleType
}

func (v *MoveValidator) validateRequestListTypeCompatibility(itemType string, listType RequestListType) error {
	compatibilityMap := map[string][]RequestListType{
		"header": {RequestListTypeHeaders, RequestListTypeHeadersDeltas},
		"query":  {RequestListTypeQueries, RequestListTypeQueriesDeltas},
		"body_form": {RequestListTypeBodyForm, RequestListTypeBodyFormDeltas},
		"body_url_encoded": {RequestListTypeBodyUrlEncoded, RequestListTypeBodyUrlEncodedDeltas},
	}

	allowedTypes, exists := compatibilityMap[itemType]
	if !exists {
		return ErrIncompatibleType
	}

	for _, allowedType := range allowedTypes {
		if allowedType == listType {
			return nil
		}
	}

	return ErrIncompatibleType
}

func (v *MoveValidator) validateFlowListTypeCompatibility(itemType string, listType FlowListType) error {
	compatibilityMap := map[string][]FlowListType{
		"node":     {FlowListTypeNodes},
		"edge":     {FlowListTypeEdges},
		"variable": {FlowListTypeVariables},
	}

	allowedTypes, exists := compatibilityMap[itemType]
	if !exists {
		return ErrIncompatibleType
	}

	for _, allowedType := range allowedTypes {
		if allowedType == listType {
			return nil
		}
	}

	return ErrIncompatibleType
}

func (v *MoveValidator) validateWorkspaceListTypeCompatibility(itemType string, listType WorkspaceListType) error {
	compatibilityMap := map[string][]WorkspaceListType{
		"workspace":   {WorkspaceListTypeWorkspaces},
		"environment": {WorkspaceListTypeEnvironments},
		"variable":    {WorkspaceListTypeVariables},
		"tag":         {WorkspaceListTypeTags},
	}

	allowedTypes, exists := compatibilityMap[itemType]
	if !exists {
		return ErrIncompatibleType
	}

	for _, allowedType := range allowedTypes {
		if allowedType == listType {
			return nil
		}
	}

	return ErrIncompatibleType
}