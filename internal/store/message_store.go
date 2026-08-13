package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// User represents a user in the database.
type User struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

// Conversation represents a support chat conversation.
type Conversation struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}

// Message represents a persisted chat message.
type Message struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversation_id"`
	SenderID       string    `json:"sender_id"`
	Content        string    `json:"content"`
	CreatedAt      time.Time `json:"created_at"`
}

// OutgoingMessage represents the structured message sent to clients and returned in history.
type OutgoingMessage struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversation_id"`
	SenderID       string    `json:"sender_id,omitempty"` // empty if anonymized
	SenderName     string    `json:"sender_name"`
	SenderRole     string    `json:"sender_role"`
	Content        string    `json:"content"`
	CreatedAt      time.Time `json:"created_at"`
}

// Store handles all database queries for users, conversations, and messages.
type Store struct {
	db *pgxpool.Pool
}

// NewStore creates a new database Store.
func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

// GetUserByID fetches a user by their UUID.
func (s *Store) GetUserByID(ctx context.Context, id string) (*User, error) {
	const query = `
		SELECT id, name, role, created_at
		FROM users
		WHERE id = $1`

	u := &User{}
	err := s.db.QueryRow(ctx, query, id).Scan(&u.ID, &u.Name, &u.Role, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

// GetOrCreateConversation finds an existing conversation for a Customer/Driver or creates one.
func (s *Store) GetOrCreateConversation(ctx context.Context, userID string) (string, error) {
	// First, check if a conversation already exists
	const checkQuery = `
		SELECT id
		FROM conversations
		WHERE user_id = $1
		LIMIT 1`

	var id string
	err := s.db.QueryRow(ctx, checkQuery, userID).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}

	// If not found, create a new one
	const insertQuery = `
		INSERT INTO conversations (user_id)
		VALUES ($1)
		RETURNING id`

	err = s.db.QueryRow(ctx, insertQuery, userID).Scan(&id)
	if err != nil {
		return "", err
	}

	return id, nil
}

// InsertMessage stores a chat message in PostgreSQL and returns the details.
func (s *Store) InsertMessage(ctx context.Context, conversationID, senderID, content string) (*Message, error) {
	const query = `
		INSERT INTO messages (conversation_id, sender_id, content)
		VALUES ($1, $2, $3)
		RETURNING id, conversation_id, sender_id, content, created_at`

	msg := &Message{}
	err := s.db.QueryRow(ctx, query, conversationID, senderID, content).
		Scan(&msg.ID, &msg.ConversationID, &msg.SenderID, &msg.Content, &msg.CreatedAt)
	if err != nil {
		return nil, err
	}
	return msg, nil
}

// GetConversationOwner retrieves the Customer/Driver ID associated with a conversation.
func (s *Store) GetConversationOwner(ctx context.Context, conversationID string) (string, error) {
	const query = `
		SELECT user_id
		FROM conversations
		WHERE id = $1`

	var userID string
	err := s.db.QueryRow(ctx, query, conversationID).Scan(&userID)
	if err != nil {
		return "", err
	}
	return userID, nil
}

// GetChatHistory returns messages for a conversation, support cursor-based pagination.
func (s *Store) GetChatHistory(ctx context.Context, conversationID string, cursorTime time.Time, limit int) ([]OutgoingMessage, error) {
	var rows pgx.Rows
	var err error

	if cursorTime.IsZero() {
		const query = `
			SELECT m.id, m.conversation_id, m.sender_id, u.name, u.role, m.content, m.created_at
			FROM messages m
			JOIN users u ON m.sender_id = u.id
			WHERE m.conversation_id = $1
			ORDER BY m.created_at DESC
			LIMIT $2`
		rows, err = s.db.Query(ctx, query, conversationID, limit)
	} else {
		const query = `
			SELECT m.id, m.conversation_id, m.sender_id, u.name, u.role, m.content, m.created_at
			FROM messages m
			JOIN users u ON m.sender_id = u.id
			WHERE m.conversation_id = $1 AND m.created_at < $2
			ORDER BY m.created_at DESC
			LIMIT $3`
		rows, err = s.db.Query(ctx, query, conversationID, cursorTime, limit)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []OutgoingMessage
	for rows.Next() {
		var m OutgoingMessage
		err := rows.Scan(&m.ID, &m.ConversationID, &m.SenderID, &m.SenderName, &m.SenderRole, &m.Content, &m.CreatedAt)
		if err != nil {
			return nil, err
		}
		messages = append(messages, m)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return messages, nil
}
