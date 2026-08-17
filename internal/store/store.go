package store

import (
	"time"

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
	ID            string    `json:"id"`
	UserID        string    `json:"user_id"`
	AdminID       *string   `json:"admin_id"`
	SendedBy      string    `json:"sended_by"`
	Content       string    `json:"content"`
	Seen          bool      `json:"seen"`
	VoiceMessages string    `json:"voice_messages,omitempty"`
	Photo         string    `json:"photo,omitempty"`
	File          string    `json:"file,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// OutgoingMessage represents the structured message sent to clients and returned in history.
type OutgoingMessage struct {
	Type          string    `json:"type,omitempty"` // NEW_MESSAGE, EDIT_MESSAGE, DELETE_MESSAGE, READ_STATUS
	ID            string    `json:"id,omitempty"`
	UserID        string    `json:"user_id"`
	AdminID       *string   `json:"admin_id,omitempty"` // empty if anonymized or not an admin
	SendedBy      string    `json:"sended_by,omitempty"`
	SenderName    string    `json:"sender_name,omitempty"`
	Content       string    `json:"content,omitempty"`
	Seen          bool      `json:"seen,omitempty"`
	VoiceMessages string    `json:"voice_messages,omitempty"`
	Photo         string    `json:"photo,omitempty"`
	File          string    `json:"file,omitempty"`
	CreatedAt     time.Time `json:"created_at,omitempty"`
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
