package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Message represents a single chat message.
type Message struct {
	ID        int64     `json:"id"`
	SenderID  int64     `json:"sender_id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateMessageParams holds the fields required to insert a new message.
type CreateMessageParams struct {
	SenderID int64  `json:"sender_id"`
	Content  string `json:"content"`
}

// MessageStore handles database operations for messages.
type MessageStore struct {
	db *pgxpool.Pool
}

// NewMessageStore creates a new MessageStore.
func NewMessageStore(db *pgxpool.Pool) *MessageStore {
	return &MessageStore{db: db}
}

// Create inserts a new message and returns the persisted record.
func (s *MessageStore) Create(ctx context.Context, p CreateMessageParams) (*Message, error) {
	const query = `
		INSERT INTO messages (sender_id, content)
		VALUES ($1, $2)
		RETURNING id, sender_id, content, created_at`

	msg := &Message{}
	err := s.db.QueryRow(ctx, query, p.SenderID, p.Content).
		Scan(&msg.ID, &msg.SenderID, &msg.Content, &msg.CreatedAt)
	if err != nil {
		return nil, err
	}
	return msg, nil
}

// GetByID fetches a single message by its ID.
// Returns nil, nil when no row is found.
func (s *MessageStore) GetByID(ctx context.Context, id int64) (*Message, error) {
	const query = `
		SELECT id, sender_id, content, created_at
		FROM messages
		WHERE id = $1`

	msg := &Message{}
	err := s.db.QueryRow(ctx, query, id).
		Scan(&msg.ID, &msg.SenderID, &msg.Content, &msg.CreatedAt)
	if err != nil {
		return nil, err
	}
	return msg, nil
}

// List returns all messages ordered by newest first.
func (s *MessageStore) List(ctx context.Context) ([]Message, error) {
	const query = `
		SELECT id, sender_id, content, created_at
		FROM messages
		ORDER BY created_at DESC`

	rows, err := s.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var msg Message
		if err := rows.Scan(&msg.ID, &msg.SenderID, &msg.Content, &msg.CreatedAt); err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}
	return messages, rows.Err()
}

