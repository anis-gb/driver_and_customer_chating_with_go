package store

import (
	"context"
	"time"
)

// BotStatusHistory represents an audit record for AI bot toggle events.
type BotStatusHistory struct {
	ID                string    `json:"id"`
	TargetUserID      string    `json:"target_user_id"`
	UserRole          string    `json:"user_role"`
	Enabled           bool      `json:"enabled"`
	ChangedByUserID   string    `json:"changed_by_user_id"`
	ChangedByUserName string    `json:"changed_by_user_name"`
	ChangedByUserType string    `json:"changed_by_user_type"`
	Reason            string    `json:"reason"`
	CreatedAt         time.Time `json:"created_at"`
}

// SaveBotStatusHistory saves a bot toggle audit event.
func (s *Store) SaveBotStatusHistory(ctx context.Context, targetUserID, userRole string, enabled bool, changedByUserID, changedByUserName, changedByUserType, reason string) error {
	query := `
		INSERT INTO bot_status_history 
			(target_user_id, user_role, enabled, changed_by_user_id, changed_by_user_name, changed_by_user_type, reason)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := s.db.Exec(ctx, query, targetUserID, userRole, enabled, changedByUserID, changedByUserName, changedByUserType, reason)
	return err
}

// GetBotStatusHistory retrieves all toggle audit records for a target user and role.
func (s *Store) GetBotStatusHistory(ctx context.Context, targetUserID, userRole string) ([]BotStatusHistory, error) {
	query := `
		SELECT id, target_user_id, user_role, enabled, changed_by_user_id, changed_by_user_name, 
		       changed_by_user_type, COALESCE(reason, ''), created_at
		FROM bot_status_history
		WHERE target_user_id = $1 AND user_role = $2
		ORDER BY created_at DESC
	`
	rows, err := s.db.Query(ctx, query, targetUserID, userRole)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []BotStatusHistory
	for rows.Next() {
		var h BotStatusHistory
		err := rows.Scan(
			&h.ID, &h.TargetUserID, &h.UserRole, &h.Enabled, &h.ChangedByUserID, &h.ChangedByUserName,
			&h.ChangedByUserType, &h.Reason, &h.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		history = append(history, h)
	}
	return history, nil
}
