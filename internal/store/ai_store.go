package store

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5"
)

// EnsureAISettingsTable creates the driver_ai_settings table if it doesn't exist.
func (s *Store) EnsureAISettingsTable(ctx context.Context) error {
	query := `
		CREATE TABLE IF NOT EXISTS driver_ai_settings (
			user_id VARCHAR(255) PRIMARY KEY,
			ai_enabled BOOLEAN DEFAULT TRUE NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		);
	`
	_, err := s.db.Exec(ctx, query)
	if err != nil {
		log.Printf("Failed to ensure driver_ai_settings table: %v", err)
		return err
	}
	return nil
}

// GetDriverAISetting retrieves the AI toggle setting for a driver.
// If no record exists, it returns true by default.
func (s *Store) GetDriverAISetting(ctx context.Context, userID string) (bool, error) {
	query := `SELECT ai_enabled FROM driver_ai_settings WHERE user_id = $1`
	var enabled bool
	err := s.db.QueryRow(ctx, query, userID).Scan(&enabled)
	if err != nil {
		if err == pgx.ErrNoRows {
			return true, nil // default is enabled
		}
		return false, err
	}
	return enabled, nil
}

// SetDriverAISetting updates or inserts the AI toggle state for a driver.
func (s *Store) SetDriverAISetting(ctx context.Context, userID string, enabled bool) error {
	query := `
		INSERT INTO driver_ai_settings (user_id, ai_enabled, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (user_id)
		DO UPDATE SET ai_enabled = $2, updated_at = NOW()
	`
	_, err := s.db.Exec(ctx, query, userID, enabled)
	return err
}
