package txutil

import (
	"context"
	"database/sql"
)

// TopicExtractor is a function that extracts a topic from an item.
// Used for auto-grouping items by topic before bulk publishing.
type TopicExtractor[T any, Topic any] func(item T) Topic

// BulkSyncTxInsert wraps a SQL transaction and tracks items to publish bulk sync events
// grouped by topic after successful commit. This eliminates the need for manual loops
// and ensures events are never forgotten.
//
// Usage:
//
//	tx, _ := db.BeginTx(ctx, nil)
//	syncTx := txutil.NewBulkInsertTx[ItemType, TopicType](tx, extractTopicFn)
//	defer devtoolsdb.TxnRollback(tx)
//
//	for _, item := range items {
//	    service.Create(ctx, item)
//	    syncTx.Track(item)
//	}
//
//	err := syncTx.CommitAndPublish(ctx, publishBulkInsertFn)
type BulkSyncTxInsert[T any, Topic comparable] struct {
	tx             *sql.Tx
	tracked        []T
	topicExtractor TopicExtractor[T, Topic]
}

// NewBulkInsertTx creates a new bulk transaction wrapper for insert operations.
// The topicExtractor function is used to extract the topic from each item for grouping.
func NewBulkInsertTx[T any, Topic comparable](
	tx *sql.Tx,
	topicExtractor TopicExtractor[T, Topic],
) *BulkSyncTxInsert[T, Topic] {
	return &BulkSyncTxInsert[T, Topic]{
		tx:             tx,
		tracked:        make([]T, 0),
		topicExtractor: topicExtractor,
	}
}

// Track adds an item to be published after successful commit.
func (s *BulkSyncTxInsert[T, Topic]) Track(item T) {
	s.tracked = append(s.tracked, item)
}

// CommitAndPublish commits the transaction and publishes all tracked items
// grouped by topic in bulk. If commit fails, no events are published.
// The publishFn receives a topic and slice of items for that topic.
//
// Items are automatically grouped by topic using the topicExtractor function,
// and publishFn is called once per unique topic.
func (s *BulkSyncTxInsert[T, Topic]) CommitAndPublish(
	ctx context.Context,
	publishFn func(Topic, []T),
) error {
	if err := s.tx.Commit(); err != nil {
		return err
	}

	// Group items by topic
	grouped := make(map[Topic][]T)
	for _, item := range s.tracked {
		topic := s.topicExtractor(item)
		grouped[topic] = append(grouped[topic], item)
	}

	// Publish each topic's batch
	for topic, items := range grouped {
		publishFn(topic, items)
	}

	return nil
}

// BulkSyncTxUpdate wraps a SQL transaction and tracks update events to publish
// grouped by topic after successful commit.
//
// Usage:
//
//	tx, _ := db.BeginTx(ctx, nil)
//	syncTx := txutil.NewBulkUpdateTx[ItemType, PatchType, TopicType](tx, extractTopicFn)
//	defer devtoolsdb.TxnRollback(tx)
//
//	for _, update := range updates {
//	    service.Update(ctx, update.Item)
//	    syncTx.Track(update.Item, update.Patch)
//	}
//
//	err := syncTx.CommitAndPublish(ctx, publishBulkUpdateFn)
type BulkSyncTxUpdate[T any, P any, Topic comparable] struct {
	tx             *sql.Tx
	tracked        []UpdateEvent[T, P]
	topicExtractor TopicExtractor[T, Topic]
}

// NewBulkUpdateTx creates a new bulk transaction wrapper for update operations.
// The topicExtractor function is used to extract the topic from each item for grouping.
func NewBulkUpdateTx[T any, P any, Topic comparable](
	tx *sql.Tx,
	topicExtractor TopicExtractor[T, Topic],
) *BulkSyncTxUpdate[T, P, Topic] {
	return &BulkSyncTxUpdate[T, P, Topic]{
		tx:             tx,
		tracked:        make([]UpdateEvent[T, P], 0),
		topicExtractor: topicExtractor,
	}
}

// Track adds an update event (item + patch) to be published after successful commit.
func (s *BulkSyncTxUpdate[T, P, Topic]) Track(item T, patch P) {
	s.tracked = append(s.tracked, UpdateEvent[T, P]{
		Item:  item,
		Patch: patch,
	})
}

// CommitAndPublish commits the transaction and publishes all tracked update events
// grouped by topic. If commit fails, no events are published.
// The publishFn receives a topic and slice of UpdateEvents for that topic.
//
// Items are automatically grouped by topic using the topicExtractor function,
// and publishFn is called once per unique topic.
func (s *BulkSyncTxUpdate[T, P, Topic]) CommitAndPublish(
	ctx context.Context,
	publishFn func(Topic, []UpdateEvent[T, P]),
) error {
	if err := s.tx.Commit(); err != nil {
		return err
	}

	// Group events by topic
	grouped := make(map[Topic][]UpdateEvent[T, P])
	for _, event := range s.tracked {
		topic := s.topicExtractor(event.Item)
		grouped[topic] = append(grouped[topic], event)
	}

	// Publish each topic's batch
	for topic, events := range grouped {
		publishFn(topic, events)
	}

	return nil
}

// BulkSyncTxDelete wraps a SQL transaction and tracks delete events to publish
// grouped by topic after successful commit.
//
// Usage:
//
//	tx, _ := db.BeginTx(ctx, nil)
//	syncTx := txutil.NewBulkDeleteTx[IDType, TopicType](tx, extractDeleteTopicFn)
//	defer devtoolsdb.TxnRollback(tx)
//
//	for _, id := range ids {
//	    service.Delete(ctx, id)
//	    syncTx.Track(id, workspaceID, isDelta)
//	}
//
//	err := syncTx.CommitAndPublish(ctx, publishBulkDeleteFn)
type BulkSyncTxDelete[ID any, Topic comparable] struct {
	tx             *sql.Tx
	tracked        []DeleteEvent[ID]
	topicExtractor func(DeleteEvent[ID]) Topic
}

// NewBulkDeleteTx creates a new bulk transaction wrapper for delete operations.
// The topicExtractor function is used to extract the topic from each DeleteEvent for grouping.
func NewBulkDeleteTx[ID any, Topic comparable](
	tx *sql.Tx,
	topicExtractor func(DeleteEvent[ID]) Topic,
) *BulkSyncTxDelete[ID, Topic] {
	return &BulkSyncTxDelete[ID, Topic]{
		tx:             tx,
		tracked:        make([]DeleteEvent[ID], 0),
		topicExtractor: topicExtractor,
	}
}

// Track adds a delete event to be published after successful commit.
func (s *BulkSyncTxDelete[ID, Topic]) Track(id ID, workspaceID ID, isDelta bool) {
	s.tracked = append(s.tracked, DeleteEvent[ID]{
		ID:          id,
		WorkspaceID: workspaceID,
		IsDelta:     isDelta,
	})
}

// CommitAndPublish commits the transaction and publishes all tracked delete events
// grouped by topic. If commit fails, no events are published.
// The publishFn receives a topic and slice of DeleteEvents for that topic.
//
// Events are automatically grouped by topic using the topicExtractor function,
// and publishFn is called once per unique topic.
func (s *BulkSyncTxDelete[ID, Topic]) CommitAndPublish(
	ctx context.Context,
	publishFn func(Topic, []DeleteEvent[ID]),
) error {
	if err := s.tx.Commit(); err != nil {
		return err
	}

	// Group events by topic
	grouped := make(map[Topic][]DeleteEvent[ID])
	for _, event := range s.tracked {
		topic := s.topicExtractor(event)
		grouped[topic] = append(grouped[topic], event)
	}

	// Publish each topic's batch
	for topic, events := range grouped {
		publishFn(topic, events)
	}

	return nil
}
