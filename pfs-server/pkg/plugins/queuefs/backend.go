package queuefs

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// QueueBackend defines the interface for queue storage backends
type QueueBackend interface {
	// Initialize initializes the backend with configuration
	Initialize(config map[string]interface{}) error

	// Close closes the backend connection
	Close() error

	// GetType returns the backend type name
	GetType() string

	// Enqueue adds a message to a queue
	Enqueue(queueName string, msg QueueMessage) error

	// Dequeue removes and returns the first message from a queue
	Dequeue(queueName string) (QueueMessage, bool, error)

	// Peek returns the first message without removing it
	Peek(queueName string) (QueueMessage, bool, error)

	// Size returns the number of messages in a queue
	Size(queueName string) (int, error)

	// Clear removes all messages from a queue
	Clear(queueName string) error

	// ListQueues returns all queue names (for directory listing)
	ListQueues(prefix string) ([]string, error)

	// GetLastEnqueueTime returns the timestamp of the last enqueued message
	GetLastEnqueueTime(queueName string) (time.Time, error)

	// RemoveQueue removes all messages for a queue and its nested queues
	RemoveQueue(queueName string) error

	// CreateQueue creates an empty queue (for mkdir support)
	CreateQueue(queueName string) error

	// QueueExists checks if a queue exists (even if empty)
	QueueExists(queueName string) (bool, error)
}

// MemoryBackend implements QueueBackend using in-memory storage
type MemoryBackend struct {
	queues map[string]*Queue
}

func NewMemoryBackend() *MemoryBackend {
	return &MemoryBackend{
		queues: make(map[string]*Queue),
	}
}

func (b *MemoryBackend) Initialize(config map[string]interface{}) error {
	// No initialization needed for memory backend
	return nil
}

func (b *MemoryBackend) Close() error {
	b.queues = nil
	return nil
}

func (b *MemoryBackend) GetType() string {
	return "memory"
}

func (b *MemoryBackend) getOrCreateQueue(queueName string) *Queue {
	if queue, exists := b.queues[queueName]; exists {
		return queue
	}
	queue := &Queue{
		messages:        []QueueMessage{},
		lastEnqueueTime: time.Time{},
	}
	b.queues[queueName] = queue
	return queue
}

func (b *MemoryBackend) Enqueue(queueName string, msg QueueMessage) error {
	queue := b.getOrCreateQueue(queueName)
	queue.mu.Lock()
	defer queue.mu.Unlock()

	queue.messages = append(queue.messages, msg)

	// Update lastEnqueueTime
	if msg.Timestamp.After(queue.lastEnqueueTime) {
		queue.lastEnqueueTime = msg.Timestamp
	} else {
		queue.lastEnqueueTime = queue.lastEnqueueTime.Add(1 * time.Nanosecond)
	}

	return nil
}

func (b *MemoryBackend) Dequeue(queueName string) (QueueMessage, bool, error) {
	queue, exists := b.queues[queueName]
	if !exists {
		return QueueMessage{}, false, nil
	}

	queue.mu.Lock()
	defer queue.mu.Unlock()

	if len(queue.messages) == 0 {
		return QueueMessage{}, false, nil
	}

	msg := queue.messages[0]
	queue.messages = queue.messages[1:]
	return msg, true, nil
}

func (b *MemoryBackend) Peek(queueName string) (QueueMessage, bool, error) {
	queue, exists := b.queues[queueName]
	if !exists {
		return QueueMessage{}, false, nil
	}

	queue.mu.Lock()
	defer queue.mu.Unlock()

	if len(queue.messages) == 0 {
		return QueueMessage{}, false, nil
	}

	return queue.messages[0], true, nil
}

func (b *MemoryBackend) Size(queueName string) (int, error) {
	queue, exists := b.queues[queueName]
	if !exists {
		return 0, nil
	}

	queue.mu.Lock()
	defer queue.mu.Unlock()

	return len(queue.messages), nil
}

func (b *MemoryBackend) Clear(queueName string) error {
	queue, exists := b.queues[queueName]
	if !exists {
		return nil
	}

	queue.mu.Lock()
	defer queue.mu.Unlock()

	queue.messages = []QueueMessage{}
	queue.lastEnqueueTime = time.Time{}
	return nil
}

func (b *MemoryBackend) ListQueues(prefix string) ([]string, error) {
	var queues []string
	for qName := range b.queues {
		if prefix == "" || qName == prefix || len(qName) > len(prefix) && qName[:len(prefix)+1] == prefix+"/" {
			queues = append(queues, qName)
		}
	}
	return queues, nil
}

func (b *MemoryBackend) GetLastEnqueueTime(queueName string) (time.Time, error) {
	queue, exists := b.queues[queueName]
	if !exists {
		return time.Time{}, nil
	}

	queue.mu.Lock()
	defer queue.mu.Unlock()

	return queue.lastEnqueueTime, nil
}

func (b *MemoryBackend) RemoveQueue(queueName string) error {
	// Remove the queue and all nested queues
	if queueName == "" {
		b.queues = make(map[string]*Queue)
		return nil
	}

	delete(b.queues, queueName)

	// Remove nested queues
	prefix := queueName + "/"
	for qName := range b.queues {
		if len(qName) > len(prefix) && qName[:len(prefix)] == prefix {
			delete(b.queues, qName)
		}
	}

	return nil
}

func (b *MemoryBackend) CreateQueue(queueName string) error {
	b.getOrCreateQueue(queueName)
	return nil
}

func (b *MemoryBackend) QueueExists(queueName string) (bool, error) {
	_, exists := b.queues[queueName]
	return exists, nil
}

// TiDBBackend implements QueueBackend using TiDB database
type TiDBBackend struct {
	db          *sql.DB
	backend     DBBackend
	backendType string
}

func NewTiDBBackend() *TiDBBackend {
	return &TiDBBackend{}
}

func (b *TiDBBackend) Initialize(config map[string]interface{}) error {
	// Store backend type from config
	backendType := "memory" // default
	if val, ok := config["backend"]; ok {
		if strVal, ok := val.(string); ok {
			backendType = strVal
		}
	}
	b.backendType = backendType

	// Create database backend
	backend, err := CreateBackend(config)
	if err != nil {
		return fmt.Errorf("failed to create backend: %w", err)
	}
	b.backend = backend

	// Open database connection
	db, err := backend.Open(config)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	b.db = db

	// Initialize schema
	for _, sqlStmt := range backend.GetInitSQL() {
		if _, err := db.Exec(sqlStmt); err != nil {
			db.Close()
			return fmt.Errorf("failed to initialize schema: %w", err)
		}
	}

	return nil
}

func (b *TiDBBackend) Close() error {
	if b.db != nil {
		return b.db.Close()
	}
	return nil
}

func (b *TiDBBackend) GetType() string {
	return b.backendType
}

func (b *TiDBBackend) Enqueue(queueName string, msg QueueMessage) error {
	msgData, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Ensure queue exists in metadata table
	_, err = b.db.Exec(
		"INSERT IGNORE INTO queue_metadata (queue_name) VALUES (?)",
		queueName,
	)
	if err != nil {
		return fmt.Errorf("failed to create queue metadata: %w", err)
	}

	// Insert message
	_, err = b.db.Exec(
		"INSERT INTO queue_messages (queue_name, message_id, data, timestamp) VALUES (?, ?, ?, ?)",
		queueName, msg.ID, string(msgData), msg.Timestamp.Unix(),
	)
	if err != nil {
		return fmt.Errorf("failed to enqueue message: %w", err)
	}

	// Update last_updated in metadata
	_, err = b.db.Exec(
		"UPDATE queue_metadata SET last_updated = CURRENT_TIMESTAMP WHERE queue_name = ?",
		queueName,
	)

	return err
}

func (b *TiDBBackend) Dequeue(queueName string) (QueueMessage, bool, error) {
	// Start transaction
	tx, err := b.db.Begin()
	if err != nil {
		return QueueMessage{}, false, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// Get the first message
	var id int64
	var data string

	err = tx.QueryRow(
		"SELECT id, data FROM queue_messages WHERE queue_name = ? ORDER BY id LIMIT 1",
		queueName,
	).Scan(&id, &data)

	if err == sql.ErrNoRows {
		return QueueMessage{}, false, nil
	} else if err != nil {
		return QueueMessage{}, false, fmt.Errorf("failed to query message: %w", err)
	}

	// Delete the message
	_, err = tx.Exec("DELETE FROM queue_messages WHERE id = ?", id)
	if err != nil {
		return QueueMessage{}, false, fmt.Errorf("failed to delete message: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return QueueMessage{}, false, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Unmarshal message
	var msg QueueMessage
	if err := json.Unmarshal([]byte(data), &msg); err != nil {
		return QueueMessage{}, false, fmt.Errorf("failed to unmarshal message: %w", err)
	}

	return msg, true, nil
}

func (b *TiDBBackend) Peek(queueName string) (QueueMessage, bool, error) {
	var data string

	err := b.db.QueryRow(
		"SELECT data FROM queue_messages WHERE queue_name = ? ORDER BY id LIMIT 1",
		queueName,
	).Scan(&data)

	if err == sql.ErrNoRows {
		return QueueMessage{}, false, nil
	} else if err != nil {
		return QueueMessage{}, false, fmt.Errorf("failed to peek message: %w", err)
	}

	// Unmarshal message
	var msg QueueMessage
	if err := json.Unmarshal([]byte(data), &msg); err != nil {
		return QueueMessage{}, false, fmt.Errorf("failed to unmarshal message: %w", err)
	}

	return msg, true, nil
}

func (b *TiDBBackend) Size(queueName string) (int, error) {
	var count int
	err := b.db.QueryRow(
		"SELECT COUNT(*) FROM queue_messages WHERE queue_name = ?",
		queueName,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get queue size: %w", err)
	}
	return count, nil
}

func (b *TiDBBackend) Clear(queueName string) error {
	_, err := b.db.Exec(
		"DELETE FROM queue_messages WHERE queue_name = ?",
		queueName,
	)
	if err != nil {
		return fmt.Errorf("failed to clear queue: %w", err)
	}
	return nil
}

func (b *TiDBBackend) ListQueues(prefix string) ([]string, error) {
	// Query from metadata table to include empty queues
	var query string
	var args []interface{}

	if prefix == "" {
		query = "SELECT queue_name FROM queue_metadata"
	} else {
		query = "SELECT queue_name FROM queue_metadata WHERE queue_name = ? OR queue_name LIKE ?"
		args = []interface{}{prefix, prefix + "/%"}
	}

	rows, err := b.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list queues: %w", err)
	}
	defer rows.Close()

	var queues []string
	for rows.Next() {
		var qName string
		if err := rows.Scan(&qName); err != nil {
			return nil, fmt.Errorf("failed to scan queue name: %w", err)
		}
		queues = append(queues, qName)
	}

	return queues, nil
}

func (b *TiDBBackend) GetLastEnqueueTime(queueName string) (time.Time, error) {
	var timestamp int64
	err := b.db.QueryRow(
		"SELECT MAX(timestamp) FROM queue_messages WHERE queue_name = ?",
		queueName,
	).Scan(&timestamp)

	if err == sql.ErrNoRows || timestamp == 0 {
		return time.Time{}, nil
	} else if err != nil {
		return time.Time{}, fmt.Errorf("failed to get last enqueue time: %w", err)
	}

	return time.Unix(timestamp, 0), nil
}

func (b *TiDBBackend) RemoveQueue(queueName string) error {
	if queueName == "" {
		// Remove all queues
		if _, err := b.db.Exec("DELETE FROM queue_messages"); err != nil {
			return err
		}
		_, err := b.db.Exec("DELETE FROM queue_metadata")
		return err
	}

	// Remove queue and nested queues
	_, err := b.db.Exec(
		"DELETE FROM queue_messages WHERE queue_name = ? OR queue_name LIKE ?",
		queueName, queueName+"/%",
	)
	if err != nil {
		return err
	}

	_, err = b.db.Exec(
		"DELETE FROM queue_metadata WHERE queue_name = ? OR queue_name LIKE ?",
		queueName, queueName+"/%",
	)
	return err
}

func (b *TiDBBackend) CreateQueue(queueName string) error {
	_, err := b.db.Exec(
		"INSERT IGNORE INTO queue_metadata (queue_name) VALUES (?)",
		queueName,
	)
	if err != nil {
		return fmt.Errorf("failed to create queue: %w", err)
	}
	return nil
}

func (b *TiDBBackend) QueueExists(queueName string) (bool, error) {
	var count int
	err := b.db.QueryRow(
		"SELECT COUNT(*) FROM queue_metadata WHERE queue_name = ?",
		queueName,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check queue existence: %w", err)
	}
	return count > 0, nil
}
