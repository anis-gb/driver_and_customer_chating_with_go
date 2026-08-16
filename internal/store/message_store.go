package store

import (
	"context"
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

// Message represents a persisted chat message.
type Message struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	AdminID   *string   `json:"admin_id"`
	SendedBy  string    `json:"sended_by"`
	Content   string    `json:"content"`
	Seen      bool      `json:"seen"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// OutgoingMessage represents the structured message sent to clients and returned in history.
type OutgoingMessage struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	AdminID    *string   `json:"admin_id,omitempty"` // empty if anonymized or not an admin
	SendedBy   string    `json:"sended_by"`
	SenderName string    `json:"sender_name"`
	Content    string    `json:"content"`
	Seen       bool      `json:"seen"`
	CreatedAt  time.Time `json:"created_at"`
}

// AdminConversation represents an active chat thread summary for admin dashboard.
type AdminConversation struct {
	UserID       string    `json:"user_id"`
	CustomerName string    `json:"customer_name"`
	Role         string    `json:"role"`
	LastMessage  string    `json:"last_message"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Store handles all database queries for users and messages.
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

// InsertMessage stores a chat message in PostgreSQL and returns the details.
func (s *Store) InsertMessage(ctx context.Context, userID string, adminID *string, sendedBy string, content string) (*Message, error) {
	const query = `
		INSERT INTO messages (user_id, admin_id, sended_by, content)
		VALUES ($1, $2, $3, $4)
		RETURNING id, user_id, admin_id, sended_by, content, seen, created_at, updated_at`

	msg := &Message{}
	err := s.db.QueryRow(ctx, query, userID, adminID, sendedBy, content).
		Scan(&msg.ID, &msg.UserID, &msg.AdminID, &msg.SendedBy, &msg.Content, &msg.Seen, &msg.CreatedAt, &msg.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return msg, nil
}

// GetChatHistory returns messages for a specific customer/driver user, supports cursor-based pagination.
func (s *Store) GetChatHistory(ctx context.Context, userID string, cursorTime time.Time, limit int) ([]OutgoingMessage, error) {
	var rows pgx.Rows
	var err error

	queryLimit := limit + 1

	if cursorTime.IsZero() {
		const query = `
			SELECT m.id, m.user_id, m.admin_id, m.sended_by, 
			       CASE WHEN m.sended_by = 'ADMIN' THEN COALESCE(a.name, 'Admin') ELSE u.name END as sender_name,
			       m.content, m.seen, m.created_at
			FROM messages m
			JOIN users u ON m.user_id = u.id
			LEFT JOIN users a ON m.admin_id = a.id
			WHERE m.user_id = $1
			ORDER BY m.created_at DESC
			LIMIT $2`
		rows, err = s.db.Query(ctx, query, userID, queryLimit)
	} else {
		const query = `
			SELECT m.id, m.user_id, m.admin_id, m.sended_by, 
			       CASE WHEN m.sended_by = 'ADMIN' THEN COALESCE(a.name, 'Admin') ELSE u.name END as sender_name,
			       m.content, m.seen, m.created_at
			FROM messages m
			JOIN users u ON m.user_id = u.id
			LEFT JOIN users a ON m.admin_id = a.id
			WHERE m.user_id = $1 AND m.created_at < $2
			ORDER BY m.created_at DESC
			LIMIT $3`
		rows, err = s.db.Query(ctx, query, userID, cursorTime, queryLimit)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []OutgoingMessage
	for rows.Next() {
		var m OutgoingMessage
		err := rows.Scan(&m.ID, &m.UserID, &m.AdminID, &m.SendedBy, &m.SenderName, &m.Content, &m.Seen, &m.CreatedAt)
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

// GetAdminConversations fetches all active users (customers/drivers) with the latest message details for admins.
func (s *Store) GetAdminConversations(ctx context.Context) ([]AdminConversation, error) {
	const query = `
		SELECT 
			u.id AS user_id,
			u.name AS customer_name,
            u.role AS role,
			COALESCE(m.content, '') AS last_message,
			COALESCE(m.created_at, u.created_at) AS updated_at
		FROM users u
		LEFT JOIN LATERAL (
			SELECT content, created_at
			FROM messages
			WHERE user_id = u.id
			ORDER BY created_at DESC
			LIMIT 1
		) m ON true
        WHERE u.role IN ('CUSTOMER', 'DRIVER')
		ORDER BY updated_at DESC`

	rows, err := s.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var conversations []AdminConversation
	for rows.Next() {
		var ac AdminConversation
		if err := rows.Scan(&ac.UserID, &ac.CustomerName, &ac.Role, &ac.LastMessage, &ac.UpdatedAt); err != nil {
			return nil, err
		}
		conversations = append(conversations, ac)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return conversations, nil
}

// MarkMessagesAsSeen updates the 'seen' status to true for messages in a chat thread.
func (s *Store) MarkMessagesAsSeen(ctx context.Context, targetUserID string, viewerRole string) error {
	// If an Admin views the chat, mark all messages NOT from an Admin as seen
	// If a Customer/Driver views the chat, mark all messages FROM an Admin as seen
	var query string
	
	if viewerRole == "ADMIN" {
		query = `
			UPDATE messages 
			SET seen = true, updated_at = NOW() 
			WHERE user_id = $1 AND sended_by != 'ADMIN' AND seen = false`
	} else {
		query = `
			UPDATE messages 
			SET seen = true, updated_at = NOW() 
			WHERE user_id = $1 AND sended_by = 'ADMIN' AND seen = false`
	}

	_, err := s.db.Exec(ctx, query, targetUserID)
	return err
}
