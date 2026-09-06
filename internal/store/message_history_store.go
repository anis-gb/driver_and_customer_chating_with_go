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

// GetDriverMessageByID retrieves a single driver message by ID.
func (s *Store) GetDriverMessageByID(ctx context.Context, messageID string) (*Message, error) {
	query := `
		SELECT id, user_id, admin_id, sended_by, content, seen, 
		       COALESCE(voice_messages, ''), COALESCE(photo, ''), COALESCE(file, ''), 
		       COALESCE(user_phone, ''), COALESCE(full_name, ''), COALESCE(profile_picture, ''), 
		       COALESCE(gender, ''), created_at, updated_at
		FROM driver_messages
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

// GetMessageEditHistory retrieves the edit history for a specific message.
func (s *Store) GetMessageEditHistory(ctx context.Context, messageID string) ([]MessageHistory, error) {
	query := `
		SELECT id, message_id, message_type, edited_by_user_id, edited_by_user_name, 
		       edited_by_user_type, edit_time, COALESCE(old_value, ''), COALESCE(new_value, ''), created_at
		FROM message_history
		WHERE message_id = $1
		ORDER BY edit_time ASC
	`
	rows, err := s.db.Query(ctx, query, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []MessageHistory
	for rows.Next() {
		var h MessageHistory
		err := rows.Scan(
			&h.ID, &h.MessageID, &h.MessageType, &h.EditedByUserID, &h.EditedByUserName,
			&h.EditedByType, &h.EditTime, &h.OldValue, &h.NewValue, &h.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		history = append(history, h)
	}
	return history, nil
}

