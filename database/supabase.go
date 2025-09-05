package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

type SupabaseDB struct {
	db *sql.DB
}

// Database models
type Session struct {
	ID           string    `json:"id" db:"id"`
	PhoneNumber  string    `json:"phone_number" db:"phone_number"`
	SessionID    string    `json:"session_id" db:"session_id"`
	AuthStatus   string    `json:"auth_status" db:"auth_status"`
	DeviceID     *string   `json:"device_id" db:"device_id"`
	BusinessName *string   `json:"business_name" db:"business_name"`
	Platform     *string   `json:"platform" db:"platform"`
	LastSeen     time.Time `json:"last_seen" db:"last_seen"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

type Message struct {
	ID           string            `json:"id" db:"id"`
	PhoneNumber  string            `json:"phone_number" db:"phone_number"`
	ChatID       string            `json:"chat_id" db:"chat_id"`
	SenderID     string            `json:"sender_id" db:"sender_id"`
	Content      map[string]interface{} `json:"content" db:"content"`
	Timestamp    time.Time         `json:"timestamp" db:"timestamp"`
	IsFromMe     bool              `json:"is_from_me" db:"is_from_me"`
	IsGroup      bool              `json:"is_group" db:"is_group"`
	IsRead       bool              `json:"is_read" db:"is_read"`
	CreatedAt    time.Time         `json:"created_at" db:"created_at"`
}

type Contact struct {
	ID           string    `json:"id" db:"id"`
	PhoneNumber  string    `json:"phone_number" db:"phone_number"`
	ContactID    string    `json:"contact_id" db:"contact_id"`
	DisplayName  *string   `json:"display_name" db:"display_name"`
	PushName     *string   `json:"push_name" db:"push_name"`
	IsBusiness   bool      `json:"is_business" db:"is_business"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

type ChatMetadata struct {
	ID                    string     `json:"id" db:"id"`
	PhoneNumber          string     `json:"phone_number" db:"phone_number"`
	ChatID               string     `json:"chat_id" db:"chat_id"`
	ChatName             *string    `json:"chat_name" db:"chat_name"`
	IsGroup              bool       `json:"is_group" db:"is_group"`
	UnreadCount          int        `json:"unread_count" db:"unread_count"`
	LastMessageID        *string    `json:"last_message_id" db:"last_message_id"`
	LastMessageTimestamp *time.Time `json:"last_message_timestamp" db:"last_message_timestamp"`
	MutedUntil           *time.Time `json:"muted_until" db:"muted_until"`
	Pinned               bool       `json:"pinned" db:"pinned"`
	Archived             bool       `json:"archived" db:"archived"`
	CreatedAt            time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at" db:"updated_at"`
}

type DeviceStorage struct {
	ID          string    `json:"id" db:"id"`
	PhoneNumber string    `json:"phone_number" db:"phone_number"`
	Key         string    `json:"key" db:"key"`
	Value       []byte    `json:"value" db:"value"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// Database interface
type SupabaseStore interface {
	// Session management
	CreateSession(session *Session) error
	GetSession(phoneNumber string) (*Session, error)
	UpdateSession(session *Session) error
	DeleteSession(phoneNumber string) error
	ListSessions() ([]*Session, error)

	// Message operations
	SaveMessage(message *Message) error
	GetMessages(phoneNumber string, limit int) ([]*Message, error)
	GetChatMessages(phoneNumber, chatID string, limit int) ([]*Message, error)
	UpdateMessageReadStatus(phoneNumber, messageID string, isRead bool) error
	GetUnreadMessageCount(phoneNumber string) (int, error)

	// Contact management
	SaveContact(contact *Contact) error
	GetContacts(phoneNumber string) ([]*Contact, error)
	GetContact(phoneNumber, contactID string) (*Contact, error)

	// Chat metadata
	SaveChatMetadata(metadata *ChatMetadata) error
	GetChatMetadata(phoneNumber, chatID string) (*ChatMetadata, error)
	GetChatsForPhone(phoneNumber string) ([]*ChatMetadata, error)
	UpdateChatUnreadCount(phoneNumber, chatID string, count int) error

	// Device storage (for WhatsApp session data)
	SaveDeviceData(phoneNumber, key string, value []byte) error
	GetDeviceData(phoneNumber, key string) ([]byte, error)
	DeleteDeviceData(phoneNumber, key string) error
	GetAllDeviceKeys(phoneNumber string) ([]string, error)

	// Health check
	Ping() error
	Close() error
}

// NewSupabaseDB creates a new Supabase database connection
func NewSupabaseDB(databaseURL string) (*SupabaseDB, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &SupabaseDB{db: db}, nil
}

// Session management methods
func (s *SupabaseDB) CreateSession(session *Session) error {
	query := `
		INSERT INTO sessions (phone_number, session_id, auth_status, device_id, business_name, platform)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at, last_seen
	`
	
	err := s.db.QueryRow(query, session.PhoneNumber, session.SessionID, session.AuthStatus,
		session.DeviceID, session.BusinessName, session.Platform).Scan(
		&session.ID, &session.CreatedAt, &session.UpdatedAt, &session.LastSeen)
	
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	
	return nil
}

func (s *SupabaseDB) GetSession(phoneNumber string) (*Session, error) {
	query := `
		SELECT id, phone_number, session_id, auth_status, device_id, business_name, 
		       platform, last_seen, created_at, updated_at
		FROM sessions 
		WHERE phone_number = $1
	`
	
	session := &Session{}
	err := s.db.QueryRow(query, phoneNumber).Scan(
		&session.ID, &session.PhoneNumber, &session.SessionID, &session.AuthStatus,
		&session.DeviceID, &session.BusinessName, &session.Platform,
		&session.LastSeen, &session.CreatedAt, &session.UpdatedAt)
	
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("session not found for phone number: %s", phoneNumber)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	
	return session, nil
}

func (s *SupabaseDB) UpdateSession(session *Session) error {
	query := `
		UPDATE sessions 
		SET session_id = $2, auth_status = $3, device_id = $4, business_name = $5, 
		    platform = $6, last_seen = NOW(), updated_at = NOW()
		WHERE phone_number = $1
	`
	
	_, err := s.db.Exec(query, session.PhoneNumber, session.SessionID, session.AuthStatus,
		session.DeviceID, session.BusinessName, session.Platform)
	
	if err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}
	
	return nil
}

func (s *SupabaseDB) DeleteSession(phoneNumber string) error {
	query := `DELETE FROM sessions WHERE phone_number = $1`
	
	_, err := s.db.Exec(query, phoneNumber)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}
	
	return nil
}

func (s *SupabaseDB) ListSessions() ([]*Session, error) {
	query := `
		SELECT id, phone_number, session_id, auth_status, device_id, business_name,
		       platform, last_seen, created_at, updated_at
		FROM sessions 
		ORDER BY created_at DESC
	`
	
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}
	defer rows.Close()
	
	var sessions []*Session
	for rows.Next() {
		session := &Session{}
		err := rows.Scan(&session.ID, &session.PhoneNumber, &session.SessionID,
			&session.AuthStatus, &session.DeviceID, &session.BusinessName,
			&session.Platform, &session.LastSeen, &session.CreatedAt, &session.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}
		sessions = append(sessions, session)
	}
	
	return sessions, nil
}

// Message operations
func (s *SupabaseDB) SaveMessage(message *Message) error {
	contentJSON, err := json.Marshal(message.Content)
	if err != nil {
		return fmt.Errorf("failed to marshal message content: %w", err)
	}
	
	query := `
		INSERT INTO messages (id, phone_number, chat_id, sender_id, content, timestamp, 
		                     is_from_me, is_group, is_read)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (id) DO UPDATE SET
		    content = EXCLUDED.content,
		    is_read = EXCLUDED.is_read
	`
	
	_, err = s.db.Exec(query, message.ID, message.PhoneNumber, message.ChatID,
		message.SenderID, contentJSON, message.Timestamp, message.IsFromMe,
		message.IsGroup, message.IsRead)
	
	if err != nil {
		return fmt.Errorf("failed to save message: %w", err)
	}
	
	return nil
}

func (s *SupabaseDB) GetMessages(phoneNumber string, limit int) ([]*Message, error) {
	query := `
		SELECT id, phone_number, chat_id, sender_id, content, timestamp,
		       is_from_me, is_group, is_read, created_at
		FROM messages 
		WHERE phone_number = $1
		ORDER BY timestamp DESC
		LIMIT $2
	`
	
	rows, err := s.db.Query(query, phoneNumber, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}
	defer rows.Close()
	
	return s.scanMessages(rows)
}

func (s *SupabaseDB) GetChatMessages(phoneNumber, chatID string, limit int) ([]*Message, error) {
	query := `
		SELECT id, phone_number, chat_id, sender_id, content, timestamp,
		       is_from_me, is_group, is_read, created_at
		FROM messages 
		WHERE phone_number = $1 AND chat_id = $2
		ORDER BY timestamp DESC
		LIMIT $3
	`
	
	rows, err := s.db.Query(query, phoneNumber, chatID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get chat messages: %w", err)
	}
	defer rows.Close()
	
	return s.scanMessages(rows)
}

func (s *SupabaseDB) scanMessages(rows *sql.Rows) ([]*Message, error) {
	var messages []*Message
	
	for rows.Next() {
		message := &Message{}
		var contentJSON []byte
		
		err := rows.Scan(&message.ID, &message.PhoneNumber, &message.ChatID,
			&message.SenderID, &contentJSON, &message.Timestamp,
			&message.IsFromMe, &message.IsGroup, &message.IsRead, &message.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}
		
		if err := json.Unmarshal(contentJSON, &message.Content); err != nil {
			return nil, fmt.Errorf("failed to unmarshal message content: %w", err)
		}
		
		messages = append(messages, message)
	}
	
	return messages, nil
}

func (s *SupabaseDB) UpdateMessageReadStatus(phoneNumber, messageID string, isRead bool) error {
	query := `
		UPDATE messages 
		SET is_read = $3
		WHERE phone_number = $1 AND id = $2
	`
	
	_, err := s.db.Exec(query, phoneNumber, messageID, isRead)
	if err != nil {
		return fmt.Errorf("failed to update message read status: %w", err)
	}
	
	return nil
}

func (s *SupabaseDB) GetUnreadMessageCount(phoneNumber string) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM messages
		WHERE phone_number = $1 AND is_read = false AND is_from_me = false
	`
	
	var count int
	err := s.db.QueryRow(query, phoneNumber).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get unread message count: %w", err)
	}
	
	return count, nil
}

// Device storage methods for WhatsApp session persistence
func (s *SupabaseDB) SaveDeviceData(phoneNumber, key string, value []byte) error {
	query := `
		INSERT INTO device_storage (phone_number, key, value)
		VALUES ($1, $2, $3)
		ON CONFLICT (phone_number, key) DO UPDATE SET
		    value = EXCLUDED.value,
		    updated_at = NOW()
	`
	
	_, err := s.db.Exec(query, phoneNumber, key, value)
	if err != nil {
		return fmt.Errorf("failed to save device data: %w", err)
	}
	
	return nil
}

func (s *SupabaseDB) GetDeviceData(phoneNumber, key string) ([]byte, error) {
	query := `SELECT value FROM device_storage WHERE phone_number = $1 AND key = $2`
	
	var value []byte
	err := s.db.QueryRow(query, phoneNumber, key).Scan(&value)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("device data not found for key: %s", key)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get device data: %w", err)
	}
	
	return value, nil
}

func (s *SupabaseDB) DeleteDeviceData(phoneNumber, key string) error {
	query := `DELETE FROM device_storage WHERE phone_number = $1 AND key = $2`
	
	_, err := s.db.Exec(query, phoneNumber, key)
	if err != nil {
		return fmt.Errorf("failed to delete device data: %w", err)
	}
	
	return nil
}

func (s *SupabaseDB) GetAllDeviceKeys(phoneNumber string) ([]string, error) {
	query := `SELECT key FROM device_storage WHERE phone_number = $1`
	
	rows, err := s.db.Query(query, phoneNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get device keys: %w", err)
	}
	defer rows.Close()
	
	var keys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, fmt.Errorf("failed to scan device key: %w", err)
		}
		keys = append(keys, key)
	}
	
	return keys, nil
}

// Utility methods
func (s *SupabaseDB) Ping() error {
	return s.db.Ping()
}

func (s *SupabaseDB) Close() error {
	return s.db.Close()
}

// Contact management methods
func (s *SupabaseDB) SaveContact(contact *Contact) error {
	query := `
		INSERT INTO contacts (phone_number, contact_id, display_name, push_name, is_business)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (phone_number, contact_id) DO UPDATE SET
		    display_name = EXCLUDED.display_name,
		    push_name = EXCLUDED.push_name,
		    is_business = EXCLUDED.is_business,
		    updated_at = NOW()
	`
	
	_, err := s.db.Exec(query, contact.PhoneNumber, contact.ContactID, contact.DisplayName,
		contact.PushName, contact.IsBusiness)
	
	if err != nil {
		return fmt.Errorf("failed to save contact: %w", err)
	}
	
	return nil
}

func (s *SupabaseDB) GetContacts(phoneNumber string) ([]*Contact, error) {
	query := `
		SELECT id, phone_number, contact_id, display_name, push_name, is_business, created_at, updated_at
		FROM contacts 
		WHERE phone_number = $1
		ORDER BY display_name
	`
	
	rows, err := s.db.Query(query, phoneNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get contacts: %w", err)
	}
	defer rows.Close()
	
	var contacts []*Contact
	for rows.Next() {
		contact := &Contact{}
		err := rows.Scan(&contact.ID, &contact.PhoneNumber, &contact.ContactID,
			&contact.DisplayName, &contact.PushName, &contact.IsBusiness,
			&contact.CreatedAt, &contact.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan contact: %w", err)
		}
		contacts = append(contacts, contact)
	}
	
	return contacts, nil
}

func (s *SupabaseDB) GetContact(phoneNumber, contactID string) (*Contact, error) {
	query := `
		SELECT id, phone_number, contact_id, display_name, push_name, is_business, created_at, updated_at
		FROM contacts 
		WHERE phone_number = $1 AND contact_id = $2
	`
	
	contact := &Contact{}
	err := s.db.QueryRow(query, phoneNumber, contactID).Scan(
		&contact.ID, &contact.PhoneNumber, &contact.ContactID,
		&contact.DisplayName, &contact.PushName, &contact.IsBusiness,
		&contact.CreatedAt, &contact.UpdatedAt)
	
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("contact not found for contact ID: %s", contactID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get contact: %w", err)
	}
	
	return contact, nil
}

// Chat metadata methods
func (s *SupabaseDB) SaveChatMetadata(metadata *ChatMetadata) error {
	query := `
		INSERT INTO chat_metadata (phone_number, chat_id, chat_name, is_group, unread_count,
		                          last_message_id, last_message_timestamp, muted_until, pinned, archived)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (phone_number, chat_id) DO UPDATE SET
		    chat_name = EXCLUDED.chat_name,
		    unread_count = EXCLUDED.unread_count,
		    last_message_id = EXCLUDED.last_message_id,
		    last_message_timestamp = EXCLUDED.last_message_timestamp,
		    muted_until = EXCLUDED.muted_until,
		    pinned = EXCLUDED.pinned,
		    archived = EXCLUDED.archived,
		    updated_at = NOW()
	`
	
	_, err := s.db.Exec(query, metadata.PhoneNumber, metadata.ChatID, metadata.ChatName,
		metadata.IsGroup, metadata.UnreadCount, metadata.LastMessageID,
		metadata.LastMessageTimestamp, metadata.MutedUntil, metadata.Pinned, metadata.Archived)
	
	if err != nil {
		return fmt.Errorf("failed to save chat metadata: %w", err)
	}
	
	return nil
}

func (s *SupabaseDB) GetChatMetadata(phoneNumber, chatID string) (*ChatMetadata, error) {
	query := `
		SELECT id, phone_number, chat_id, chat_name, is_group, unread_count,
		       last_message_id, last_message_timestamp, muted_until, pinned, archived,
		       created_at, updated_at
		FROM chat_metadata 
		WHERE phone_number = $1 AND chat_id = $2
	`
	
	metadata := &ChatMetadata{}
	err := s.db.QueryRow(query, phoneNumber, chatID).Scan(
		&metadata.ID, &metadata.PhoneNumber, &metadata.ChatID, &metadata.ChatName,
		&metadata.IsGroup, &metadata.UnreadCount, &metadata.LastMessageID,
		&metadata.LastMessageTimestamp, &metadata.MutedUntil, &metadata.Pinned,
		&metadata.Archived, &metadata.CreatedAt, &metadata.UpdatedAt)
	
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("chat metadata not found for chat ID: %s", chatID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get chat metadata: %w", err)
	}
	
	return metadata, nil
}

func (s *SupabaseDB) GetChatsForPhone(phoneNumber string) ([]*ChatMetadata, error) {
	query := `
		SELECT id, phone_number, chat_id, chat_name, is_group, unread_count,
		       last_message_id, last_message_timestamp, muted_until, pinned, archived,
		       created_at, updated_at
		FROM chat_metadata 
		WHERE phone_number = $1
		ORDER BY last_message_timestamp DESC NULLS LAST, created_at DESC
	`
	
	rows, err := s.db.Query(query, phoneNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get chats: %w", err)
	}
	defer rows.Close()
	
	var chats []*ChatMetadata
	for rows.Next() {
		metadata := &ChatMetadata{}
		err := rows.Scan(&metadata.ID, &metadata.PhoneNumber, &metadata.ChatID, &metadata.ChatName,
			&metadata.IsGroup, &metadata.UnreadCount, &metadata.LastMessageID,
			&metadata.LastMessageTimestamp, &metadata.MutedUntil, &metadata.Pinned,
			&metadata.Archived, &metadata.CreatedAt, &metadata.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan chat metadata: %w", err)
		}
		chats = append(chats, metadata)
	}
	
	return chats, nil
}

func (s *SupabaseDB) UpdateChatUnreadCount(phoneNumber, chatID string, count int) error {
	query := `
		UPDATE chat_metadata 
		SET unread_count = $3, updated_at = NOW()
		WHERE phone_number = $1 AND chat_id = $2
	`
	
	_, err := s.db.Exec(query, phoneNumber, chatID, count)
	if err != nil {
		return fmt.Errorf("failed to update chat unread count: %w", err)
	}
	
	return nil
}