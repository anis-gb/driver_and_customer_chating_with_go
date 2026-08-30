package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// InsertCustomerMessage stores a customer chat message in PostgreSQL.
func (s *Store) InsertCustomerMessage(ctx context.Context, userID string, adminID *string, sendedBy string, content string, voice string, photo string, file string, userPhone string, fullName string, profilePicture string, gender string) (*Message, error) {
	query := `
		INSERT INTO customer_messages (
			user_id,
			admin_id, 
			sended_by, 
			content, 
			voice_messages, 
			photo, file, 
			user_phone, 
			full_name, 
			profile_picture, 
			gender
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)RETURNING 
			id, 
			user_id, 
			admin_id, 
			sended_by, 
			content, 
			seen, 
			COALESCE(voice_messages, ''), 
			COALESCE(photo, ''), 
			COALESCE(file, ''), 
			COALESCE(user_phone, ''), 
			COALESCE(full_name, ''), 
			COALESCE(profile_picture, ''), 
			COALESCE(gender, ''), 
			created_at, 
			updated_at`

	msg := &Message{}
	err := s.db.QueryRow(ctx, query, userID, adminID, sendedBy, content, voice, photo, file, userPhone, fullName, profilePicture, gender).
		Scan(&msg.ID, &msg.UserID, &msg.AdminID, &msg.SendedBy, &msg.Content, &msg.Seen, &msg.VoiceMessages, &msg.Photo, &msg.File, &msg.UserPhone, &msg.FullName, &msg.ProfilePicture, &msg.Gender, &msg.CreatedAt, &msg.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return msg, nil
}

// GetCustomerHistory returns messages for a specific customer user.
func (s *Store) GetCustomerHistory(ctx context.Context, userID string, cursorTime time.Time, limit int) ([]OutgoingMessage, error) {
	var rows pgx.Rows
	var err error

	queryLimit := limit + 1

	if cursorTime.IsZero() {
		query := `
			SELECT id, user_id, admin_id, sended_by, 
			       CASE 
			           WHEN sended_by = 'ADMIN' THEN COALESCE(NULLIF(full_name, ''), 'Support Admin')
			           ELSE COALESCE(NULLIF(full_name, ''), 'Customer')
			       END as sender_name,
			       content, seen, COALESCE(voice_messages, ''), COALESCE(photo, ''), COALESCE(file, ''), COALESCE(user_phone, ''), COALESCE(full_name, ''), COALESCE(profile_picture, ''), COALESCE(gender, ''), created_at
			FROM customer_messages
			WHERE user_id = $1
			ORDER BY created_at DESC
			LIMIT $2`
		rows, err = s.db.Query(ctx, query, userID, queryLimit)
	} else {
		query := `
			SELECT id, user_id, admin_id, sended_by, 
			       CASE 
			           WHEN sended_by = 'ADMIN' THEN COALESCE(NULLIF(full_name, ''), 'Support Admin')
			           ELSE COALESCE(NULLIF(full_name, ''), 'Customer')
			       END as sender_name,
			       content, seen, COALESCE(voice_messages, ''), COALESCE(photo, ''), COALESCE(file, ''), COALESCE(user_phone, ''), COALESCE(full_name, ''), COALESCE(profile_picture, ''), COALESCE(gender, ''), created_at
			FROM customer_messages
			WHERE user_id = $1 AND created_at < $2
			ORDER BY created_at DESC
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
		err := rows.Scan(&m.ID, &m.UserID, &m.AdminID, &m.SendedBy, &m.SenderName, &m.Content, &m.Seen, &m.VoiceMessages, &m.Photo, &m.File, &m.UserPhone, &m.FullName, &m.ProfilePicture, &m.Gender, &m.CreatedAt)
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

// MarkCustomerMessagesAsSeen updates the 'seen' status to true for customer messages.
func (s *Store) MarkCustomerMessagesAsSeen(ctx context.Context, targetUserID string, viewerType string) error {
	var query string
	if viewerType == "ADMIN" {
		query = `
			UPDATE customer_messages 
			SET seen = true, updated_at = NOW() 
			WHERE user_id = $1 AND sended_by != 'ADMIN' AND seen = false`
	} else {
		query = `
			UPDATE customer_messages 
			SET seen = true, updated_at = NOW() 
			WHERE user_id = $1 AND sended_by = 'ADMIN' AND seen = false`
	}

	_, err := s.db.Exec(ctx, query, targetUserID)
	return err
}

// EditCustomerMessage updates the content of a specific message sent by an admin.
func (s *Store) EditCustomerMessage(ctx context.Context, messageID string, content string) (*OutgoingMessage, error) {
	query := `
		UPDATE customer_messages
		SET content = $1, updated_at = NOW()
		WHERE id = $2 AND sended_by = 'ADMIN'
		RETURNING id, user_id, admin_id, sended_by, 
			CASE WHEN sended_by = 'ADMIN' THEN 'Support Admin' ELSE COALESCE(full_name, 'Customer') END as sender_name,
			content, seen, COALESCE(voice_messages, ''), COALESCE(photo, ''), COALESCE(file, ''), COALESCE(user_phone, ''), COALESCE(full_name, ''), COALESCE(profile_picture, ''), COALESCE(gender, ''), created_at
	`

	var m OutgoingMessage
	err := s.db.QueryRow(ctx, query, content, messageID).
		Scan(&m.ID, &m.UserID, &m.AdminID, &m.SendedBy, &m.SenderName, &m.Content, &m.Seen, &m.VoiceMessages, &m.Photo, &m.File, &m.UserPhone, &m.FullName, &m.ProfilePicture, &m.Gender, &m.CreatedAt)
	if err != nil {
		return nil, err
	}
	m.Type = "EDIT_MESSAGE"
	return &m, nil
}

// DeleteCustomerMessage deletes a message by ID (admin only)
func (s *Store) DeleteCustomerMessage(ctx context.Context, messageID string) error {
	query := `
		DELETE FROM customer_messages 
		WHERE id = $1 AND sended_by = 'ADMIN'
	`

	result, err := s.db.Exec(ctx, query, messageID)
	if err != nil {
		return err
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("message not found or not an admin message")
	}

	return nil
}
