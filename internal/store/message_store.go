package store

import (
	"context"
	"fmt"
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
	UserID            string    `json:"user_id"`
	CustomerName      string    `json:"customer_name"`
	Role              string    `json:"role"`
	LastMessage       string    `json:"last_message"`
	LastMessageSender string    `json:"last_message_sender"`
	IsSeen            bool      `json:"is_seen"`
	ProfilePicture    string    `json:"profile_picture"`
	Gender            string    `json:"gender"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// Store handles all database queries for users and messages.
type Store struct {
	db *pgxpool.Pool
}

// NewStore creates a new database Store.
func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

// InsertMessage stores a chat message in PostgreSQL and returns the details.
func (s *Store) InsertMessage(ctx context.Context, userID string, adminID *string, sendedBy string, content string, targetUserType string) (*Message, error) {
	var table string
	if sendedBy == "CUSTOMER" {
		table = "customer_messages"
	} else if sendedBy == "DRIVER" {
		table = "driver_messages"
	} else if sendedBy == "ADMIN" {
		if targetUserType == "CUSTOMER" {
			table = "customer_messages"
		} else if targetUserType == "DRIVER" {
			table = "driver_messages"
		} else {
			return nil, fmt.Errorf("invalid target user type for admin reply: %s", targetUserType)
		}
	} else {
		return nil, fmt.Errorf("invalid sender role: %s", sendedBy)
	}

	query := fmt.Sprintf(`
		INSERT INTO %s (user_id, admin_id, sended_by, content)
		VALUES ($1, $2, $3, $4)
		RETURNING id, user_id, admin_id, sended_by, content, seen, created_at, updated_at`, table)

	msg := &Message{}
	err := s.db.QueryRow(ctx, query, userID, adminID, sendedBy, content).
		Scan(&msg.ID, &msg.UserID, &msg.AdminID, &msg.SendedBy, &msg.Content, &msg.Seen, &msg.CreatedAt, &msg.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return msg, nil
}

// GetChatHistory returns messages for a specific customer/driver user, supports cursor-based pagination.
func (s *Store) GetChatHistory(ctx context.Context, userID string, userType string, cursorTime time.Time, limit int) ([]OutgoingMessage, error) {
	var rows pgx.Rows
	var err error

	queryLimit := limit + 1

	var table string
	var clientName string
	if userType == "CUSTOMER" {
		table = "customer_messages"
		clientName = "Customer"
	} else if userType == "DRIVER" {
		table = "driver_messages"
		clientName = "Driver"
	} else {
		return nil, fmt.Errorf("invalid user type for chat history: %s", userType)
	}

	if cursorTime.IsZero() {
		query := fmt.Sprintf(`
			SELECT id, user_id, admin_id, sended_by, 
			       CASE WHEN sended_by = 'ADMIN' THEN 'Support Admin' ELSE COALESCE(full_name, '%s') END as sender_name,
			       content, seen, created_at
			FROM %s
			WHERE user_id = $1
			ORDER BY created_at DESC
			LIMIT $2`, clientName, table)
		rows, err = s.db.Query(ctx, query, userID, queryLimit)
	} else {
		query := fmt.Sprintf(`
			SELECT id, user_id, admin_id, sended_by, 
			       CASE WHEN sended_by = 'ADMIN' THEN 'Support Admin' ELSE COALESCE(full_name, '%s') END as sender_name,
			       content, seen, created_at
			FROM %s
			WHERE user_id = $1 AND created_at < $2
			ORDER BY created_at DESC
			LIMIT $3`, clientName, table)
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
		WITH latest_customer_msgs AS (
			SELECT DISTINCT ON (user_id)
				user_id,
				'CUSTOMER' AS role,
				content AS last_message,
				sended_by AS last_message_sender,
				seen AS is_seen,
				created_at AS updated_at,
				COALESCE(
					(SELECT full_name FROM customer_messages WHERE user_id = cm.user_id AND full_name IS NOT NULL AND full_name <> '' ORDER BY created_at DESC LIMIT 1),
					'Customer'
				) AS full_name,
				(SELECT profile_picture FROM customer_messages WHERE user_id = cm.user_id AND profile_picture IS NOT NULL AND profile_picture <> '' ORDER BY created_at DESC LIMIT 1) AS profile_picture,
				(SELECT gender FROM customer_messages WHERE user_id = cm.user_id AND gender IS NOT NULL AND gender <> '' ORDER BY created_at DESC LIMIT 1) AS gender
			FROM customer_messages cm
			ORDER BY user_id, created_at DESC
		),
		latest_driver_msgs AS (
			SELECT DISTINCT ON (user_id)
				user_id,
				'DRIVER' AS role,
				content AS last_message,
				sended_by AS last_message_sender,
				seen AS is_seen,
				created_at AS updated_at,
				COALESCE(
					(SELECT full_name FROM driver_messages WHERE user_id = dm.user_id AND full_name IS NOT NULL AND full_name <> '' ORDER BY created_at DESC LIMIT 1),
					'Driver'
				) AS full_name,
				(SELECT profile_picture FROM driver_messages WHERE user_id = dm.user_id AND profile_picture IS NOT NULL AND profile_picture <> '' ORDER BY created_at DESC LIMIT 1) AS profile_picture,
				(SELECT gender FROM driver_messages WHERE user_id = dm.user_id AND gender IS NOT NULL AND gender <> '' ORDER BY created_at DESC LIMIT 1) AS gender
			FROM driver_messages dm
			ORDER BY user_id, created_at DESC
		),
		all_conversations AS (
			SELECT user_id, role, last_message, last_message_sender, is_seen, full_name, profile_picture, gender, updated_at FROM latest_customer_msgs
			UNION ALL
			SELECT user_id, role, last_message, last_message_sender, is_seen, full_name, profile_picture, gender, updated_at FROM latest_driver_msgs
		)
		SELECT 
			user_id,
			full_name AS customer_name,
			role,
			last_message,
			last_message_sender,
			is_seen,
			COALESCE(profile_picture, '') AS profile_picture,
			COALESCE(gender, 'Male') AS gender,
			updated_at
		FROM all_conversations
		ORDER BY updated_at DESC`

	rows, err := s.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var conversations []AdminConversation
	for rows.Next() {
		var ac AdminConversation
		if err := rows.Scan(&ac.UserID, &ac.CustomerName, &ac.Role, &ac.LastMessage, &ac.LastMessageSender, &ac.IsSeen, &ac.ProfilePicture, &ac.Gender, &ac.UpdatedAt); err != nil {
			return nil, err
		}
		conversations = append(conversations, ac)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return conversations, nil
}

// GetAdminConversationsForType fetches active conversations filtered by user role (CUSTOMER or DRIVER).
func (s *Store) GetAdminConversationsForType(ctx context.Context, role string) ([]AdminConversation, error) {
	var table string
	var defaultName string
	if role == "CUSTOMER" {
		table = "customer_messages"
		defaultName = "Customer"
	} else if role == "DRIVER" {
		table = "driver_messages"
		defaultName = "Driver"
	} else {
		return nil, fmt.Errorf("invalid role: %s", role)
	}

	query := fmt.Sprintf(`
		WITH latest_msgs AS (
			SELECT DISTINCT ON (user_id)
				user_id,
				'%s' AS role,
				content AS last_message,
				sended_by AS last_message_sender,
				seen AS is_seen,
				created_at AS updated_at,
				COALESCE(
					(SELECT full_name FROM %s WHERE user_id = m.user_id AND full_name IS NOT NULL AND full_name <> '' ORDER BY created_at DESC LIMIT 1),
					'%s'
				) AS full_name,
				(SELECT profile_picture FROM %s WHERE user_id = m.user_id AND profile_picture IS NOT NULL AND profile_picture <> '' ORDER BY created_at DESC LIMIT 1) AS profile_picture,
				(SELECT gender FROM %s WHERE user_id = m.user_id AND gender IS NOT NULL AND gender <> '' ORDER BY created_at DESC LIMIT 1) AS gender
			FROM %s m
			ORDER BY user_id, created_at DESC
		)
		SELECT 
			user_id,
			full_name AS customer_name,
			role,
			last_message,
			last_message_sender,
			is_seen,
			COALESCE(profile_picture, '') AS profile_picture,
			COALESCE(gender, 'Male') AS gender,
			updated_at
		FROM latest_msgs
		ORDER BY updated_at DESC`, role, table, defaultName, table, table, table)

	rows, err := s.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var conversations []AdminConversation
	for rows.Next() {
		var ac AdminConversation
		if err := rows.Scan(&ac.UserID, &ac.CustomerName, &ac.Role, &ac.LastMessage, &ac.LastMessageSender, &ac.IsSeen, &ac.ProfilePicture, &ac.Gender, &ac.UpdatedAt); err != nil {
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
func (s *Store) MarkMessagesAsSeen(ctx context.Context, targetUserID string, targetUserType string, viewerType string) error {
	var table string
	if targetUserType == "CUSTOMER" {
		table = "customer_messages"
	} else if targetUserType == "DRIVER" {
		table = "driver_messages"
	} else {
		return fmt.Errorf("invalid target user type for marking seen: %s", targetUserType)
	}

	// If an Admin views the chat, mark all messages NOT from an Admin as seen
	// If a Customer/Driver views the chat, mark all messages FROM an Admin as seen
	var query string
	if viewerType == "ADMIN" {
		query = fmt.Sprintf(`
			UPDATE %s 
			SET seen = true, updated_at = NOW() 
			WHERE user_id = $1 AND sended_by != 'ADMIN' AND seen = false`, table)
	} else {
		query = fmt.Sprintf(`
			UPDATE %s 
			SET seen = true, updated_at = NOW() 
			WHERE user_id = $1 AND sended_by = 'ADMIN' AND seen = false`, table)
	}

	_, err := s.db.Exec(ctx, query, targetUserID)
	return err
}
