package store

import (
	"context"
	"fmt"
)

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
				admin_id,
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
				admin_id,
				COALESCE(
					(SELECT full_name FROM driver_messages WHERE user_id = dm.user_id AND full_name IS NOT NULL AND full_name <> '' ORDER BY created_at DESC LIMIT 1),
					'Driver'
				) AS full_name,
				(SELECT profile_picture FROM driver_messages WHERE user_id = dm.user_id AND profile_picture IS NOT NULL AND profile_picture <> '' ORDER BY created_at DESC LIMIT 1) AS profile_picture,
				(SELECT gender FROM driver_messages WHERE user_id = dm.user_id AND gender IS NOT NULL AND gender <> '' ORDER BY created_at DESC LIMIT 1) AS gender,
				(SELECT user_phone FROM driver_messages WHERE user_id = dm.user_id AND user_phone IS NOT NULL AND user_phone <> '' ORDER BY created_at DESC LIMIT 1) AS user_phone
			FROM driver_messages dm
			ORDER BY user_id, created_at DESC
		),
		all_conversations AS (
			SELECT user_id, role, last_message, last_message_sender, is_seen, full_name, profile_picture, gender, '' AS user_phone, updated_at, admin_id FROM latest_customer_msgs
			UNION ALL
			SELECT user_id, role, last_message, last_message_sender, is_seen, full_name, profile_picture, gender, COALESCE(user_phone, '') AS user_phone, updated_at, admin_id FROM latest_driver_msgs
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
			user_phone,
			updated_at,
			COALESCE(admin_id, '') AS admin_id
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
		if err := rows.Scan(&ac.UserID, &ac.CustomerName, &ac.Role, &ac.LastMessage, &ac.LastMessageSender, &ac.IsSeen, &ac.ProfilePicture, &ac.Gender, &ac.UserPhone, &ac.UpdatedAt, &ac.AdminID); err != nil {
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
				admin_id,
				COALESCE(
					(SELECT full_name FROM %s WHERE user_id = m.user_id AND full_name IS NOT NULL AND full_name <> '' ORDER BY created_at DESC LIMIT 1),
					'%s'
				) AS full_name,
				(SELECT profile_picture FROM %s WHERE user_id = m.user_id AND profile_picture IS NOT NULL AND profile_picture <> '' ORDER BY created_at DESC LIMIT 1) AS profile_picture,
				(SELECT gender FROM %s WHERE user_id = m.user_id AND gender IS NOT NULL AND gender <> '' ORDER BY created_at DESC LIMIT 1) AS gender,
				COALESCE((SELECT user_phone FROM %s WHERE user_id = m.user_id AND user_phone IS NOT NULL AND user_phone <> '' ORDER BY created_at DESC LIMIT 1), '') AS user_phone
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
			user_phone,
			updated_at,
			COALESCE(admin_id, '') AS admin_id
		FROM latest_msgs
		ORDER BY updated_at DESC`, role, table, defaultName, table, table, table, table)

	rows, err := s.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var conversations []AdminConversation
	for rows.Next() {
		var ac AdminConversation
		if err := rows.Scan(&ac.UserID, &ac.CustomerName, &ac.Role, &ac.LastMessage, &ac.LastMessageSender, &ac.IsSeen, &ac.ProfilePicture, &ac.Gender, &ac.UserPhone, &ac.UpdatedAt, &ac.AdminID); err != nil {
			return nil, err
		}
		conversations = append(conversations, ac)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return conversations, nil
}
