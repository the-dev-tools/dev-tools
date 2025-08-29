package movable

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"the-dev-tools/server/pkg/idwrap"
)

// mockMovableRepository implements MovableRepository for testing
type mockMovableRepository struct {
	mu            sync.RWMutex
	items         map[idwrap.IDWrap]MovableItem
	updateCounter int
}

func newMockMovableRepository() *mockMovableRepository {
	return &mockMovableRepository{
		items: make(map[idwrap.IDWrap]MovableItem),
	}
}

func (m *mockMovableRepository) UpdatePosition(ctx context.Context, tx *sql.Tx, itemID idwrap.IDWrap, listType ListType, position int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	item, exists := m.items[itemID]
	if !exists {
		return fmt.Errorf("item not found: %s", itemID.String())
	}
	
	item.Position = position
	m.items[itemID] = item
	m.updateCounter++
	
	return nil
}

func (m *mockMovableRepository) UpdatePositions(ctx context.Context, tx *sql.Tx, updates []PositionUpdate) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	for _, update := range updates {
		item, exists := m.items[update.ItemID]
		if !exists {
			return fmt.Errorf("item not found: %s", update.ItemID.String())
		}
		
		item.Position = update.Position
		m.items[update.ItemID] = item
	}
	
	m.updateCounter += len(updates)
	return nil
}

func (m *mockMovableRepository) GetMaxPosition(ctx context.Context, parentID idwrap.IDWrap, listType ListType) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	maxPos := -1
	for _, item := range m.items {
		if item.ParentID != nil && *item.ParentID == parentID && item.Position > maxPos {
			maxPos = item.Position
		}
	}
	
	return maxPos, nil
}

func (m *mockMovableRepository) GetItemsByParent(ctx context.Context, parentID idwrap.IDWrap, listType ListType) ([]MovableItem, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	var items []MovableItem
	for _, item := range m.items {
		if item.ParentID != nil && *item.ParentID == parentID {
			items = append(items, item)
		}
	}
	
	return items, nil
}

// addTestItem adds a test item to the mock repository
func (m *mockMovableRepository) addTestItem(id, parentID idwrap.IDWrap, position int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.items[id] = MovableItem{
		ID:       id,
		ParentID: &parentID,
		Position: position,
		ListType: CollectionListTypeItems,
	}
}

// getItemByID gets an item by ID (helper method for delta manager)
func (m *mockMovableRepository) getItemByID(id idwrap.IDWrap) (MovableItem, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	item, exists := m.items[id]
	return item, exists
}