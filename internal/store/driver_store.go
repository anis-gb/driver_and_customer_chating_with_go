package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

// InsertDriverMessage stores a driver chat message in PostgreSQL.
func (s *Store) InsertDriverMessage(ctx context.Context, userID string, adminID *string, sendedBy string, content string) (*Message, error) {
	query := `
		INSERT INTO driver_messages (user_id, admin_id, sended_by, content)
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

// GetDriverHistory returns messages for a specific driver user.
func (s *Store) GetDriverHistory(ctx context.Context, userID string, cursorTime time.Time, limit int) ([]OutgoingMessage, error) {
	var rows pgx.Rows
	var err error

	queryLimit := limit + 1

	if cursorTime.IsZero() {
		query := `
			SELECT id, user_id, admin_id, sended_by, 
			       CASE WHEN sended_by = 'ADMIN' THEN 'Support Admin' ELSE COALESCE(full_name, 'Driver') END as sender_name,
			       content, seen, created_at
			FROM driver_messages
			WHERE user_id = $1
			ORDER BY created_at DESC
			LIMIT $2`
		rows, err = s.db.Query(ctx, query, userID, queryLimit)
	} else {
		query := `
			SELECT id, user_id, admin_id, sended_by, 
			       CASE WHEN sended_by = 'ADMIN' THEN 'Support Admin' ELSE COALESCE(full_name, 'Driver') END as sender_name,
			       content, seen, created_at
			FROM driver_messages
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

// MarkDriverMessagesAsSeen updates the 'seen' status to true for driver messages.
func (s *Store) MarkDriverMessagesAsSeen(ctx context.Context, targetUserID string, viewerType string) error {
	var query string
	if viewerType == "ADMIN" {
		query = `
			UPDATE driver_messages 
			SET seen = true, updated_at = NOW() 
			WHERE user_id = $1 AND sended_by != 'ADMIN' AND seen = false`
	} else {
		query = `
			UPDATE driver_messages 
			SET seen = true, updated_at = NOW() 
			WHERE user_id = $1 AND sended_by = 'ADMIN' AND seen = false`
	}

	_, err := s.db.Exec(ctx, query, targetUserID)
	return err
}

// EditDriverMessage updates the content of a specific message sent by an admin.
func (s *Store) EditDriverMessage(ctx context.Context, messageID string, content string) (*OutgoingMessage, error) {
	query := `
		UPDATE driver_messages
		SET content = $1, updated_at = NOW()
		WHERE id = $2 AND sended_by = 'ADMIN'
		RETURNING id, user_id, admin_id, sended_by, 
			CASE WHEN sended_by = 'ADMIN' THEN 'Support Admin' ELSE COALESCE(full_name, 'Driver') END as sender_name,
			content, seen, created_at
	`

	var m OutgoingMessage
	err := s.db.QueryRow(ctx, query, content, messageID).
		Scan(&m.ID, &m.UserID, &m.AdminID, &m.SendedBy, &m.SenderName, &m.Content, &m.Seen, &m.CreatedAt)
	if err != nil {
		return nil, err
	}
	m.Type = "EDIT_MESSAGE"
	return &m, nil
}
