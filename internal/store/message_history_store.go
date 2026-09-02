package store

import (
	"context"
	"time"
)

// MessageHistory represents an audit trail record for message edits.
type MessageHistory struct {
	ID              string    `json:"id"`
	MessageID       string    `json:"message_id"`
	MessageType     string    `json:"message_type"`
	EditedByUserID  string    `json:"edited_by_user_id"`
	EditedByUserName string   `json:"edited_by_user_name"`
	EditedByType    string    `json:"edited_by_user_type"`
	EditTime        time.Time `json:"edit_time"`
	OldValue        string    `json:"old_value"`
	NewValue        string    `json:"new_value"`
	CreatedAt       time.Time `json:"created_at"`
}

// GetCustomerMessageByID retrieves a single customer message by ID.
func (s *Store) GetCustomerMessageByID(ctx context.Context, messageID string) (*Message, error) {
	query := `
		SELECT id, user_id, admin_id, sended_by, content, seen, 
		       COALESCE(voice_messages, ''), COALESCE(photo, ''), COALESCE(file, ''), 
		       COALESCE(user_phone, ''), COALESCE(full_name, ''), COALESCE(profile_picture, ''), 
		       COALESCE(gender, ''), created_at, updated_at
		FROM customer_messages
		WHERE id = $1
	`
	msg := &Message{}
	err := s.db.QueryRow(ctx, query, messageID).
		Scan(
			&msg.ID, &msg.UserID, &msg.AdminID, &msg.SendedBy, &msg.Content, &msg.Seen,
			&msg.VoiceMessages, &msg.Photo, &msg.File, &msg.UserPhone, &msg.FullName,
			&msg.ProfilePicture, &msg.Gender, &msg.CreatedAt, &msg.UpdatedAt,
		)
	if err != nil {
		return nil, err
	}
	return msg, nil
}

// SaveMessageHistory saves an edit history record.
func (s *Store) SaveMessageHistory(ctx context.Context, messageID, messageType, editedByUserID, editedByUserName, editedByType, oldValue, newValue string) error {
	query := `
		INSERT INTO message_history 
			(message_id, message_type, edited_by_user_id, edited_by_user_name, edited_by_user_type, old_value, new_value)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := s.db.Exec(ctx, query, messageID, messageType, editedByUserID, editedByUserName, editedByType, oldValue, newValue)
	return err
}
