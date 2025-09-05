package session

import (
	"context"
	"fmt"
	"sync"
	"time"

	"whatsapp-wrapper/database"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
)

type SessionStatus string

const (
	StatusPending        SessionStatus = "pending"
	StatusAuthenticating SessionStatus = "authenticating"
	StatusAuthenticated  SessionStatus = "authenticated"
	StatusDisconnected   SessionStatus = "disconnected"
	StatusError          SessionStatus = "error"
)

type WhatsAppSession struct {
	PhoneNumber   string
	Client        *whatsmeow.Client
	Store         database.SupabaseStore
	Status        SessionStatus
	LastSeen      time.Time
	CurrentQR     string
	ErrorMessage  string
	mu            sync.RWMutex
}

type SessionManager struct {
	sessions  map[string]*WhatsAppSession
	supabase  database.SupabaseStore
	container *sqlstore.Container
	logger    waLog.Logger
	mu        sync.RWMutex
}

func NewSessionManager(supabaseStore database.SupabaseStore, databaseURL string, logger waLog.Logger) (*SessionManager, error) {
	dbLog := waLog.Stdout("Database", "INFO", true)
	container, err := sqlstore.New(context.Background(), "postgres", databaseURL, dbLog)
	if err != nil {
		return nil, fmt.Errorf("failed to create sqlstore container: %w", err)
	}

	return &SessionManager{
		sessions:  make(map[string]*WhatsAppSession),
		supabase:  supabaseStore,
		container: container,
		logger:    logger,
	}, nil
}

func (sm *SessionManager) CreateSession(phoneNumber string) (*WhatsAppSession, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if session, exists := sm.sessions[phoneNumber]; exists {
		return session, nil
	}

	deviceStore := sm.container.NewDevice()

	clientLog := waLog.Stdout(fmt.Sprintf("Client-%s", phoneNumber), "INFO", true)
	client := whatsmeow.NewClient(deviceStore, clientLog)

	session := &WhatsAppSession{
		PhoneNumber: phoneNumber,
		Client:      client,
		Store:       sm.supabase,
		Status:      StatusPending,
		LastSeen:    time.Now(),
	}

	client.AddEventHandler(func(evt interface{}) {
		sm.handleSessionEvent(phoneNumber, evt)
	})

	sm.sessions[phoneNumber] = session

	dbSession := &database.Session{
		PhoneNumber: phoneNumber,
		SessionID:   "pending",
		AuthStatus:  string(StatusPending),
	}
	
	err := sm.supabase.CreateSession(dbSession)
	if err != nil {
		delete(sm.sessions, phoneNumber)
		return nil, fmt.Errorf("failed to create session in database: %w", err)
	}

	sm.logger.Infof("Created new session for phone number: %s", phoneNumber)
	return session, nil
}

func (sm *SessionManager) GetSession(phoneNumber string) (*WhatsAppSession, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, exists := sm.sessions[phoneNumber]
	if !exists {
		return nil, fmt.Errorf("session not found for phone number: %s", phoneNumber)
	}

	return session, nil
}

func (sm *SessionManager) ConnectSession(phoneNumber string) error {
	session, err := sm.GetSession(phoneNumber)
	if err != nil {
		return err
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	if session.Client.IsConnected() {
		return nil
	}

	err = session.Client.Connect()
	if err != nil {
		session.Status = StatusError
		session.ErrorMessage = err.Error()
		return fmt.Errorf("failed to connect session: %w", err)
	}

	return nil
}

func (sm *SessionManager) DisconnectSession(phoneNumber string) error {
	session, err := sm.GetSession(phoneNumber)
	if err != nil {
		return err
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	session.Client.Disconnect()
	session.Status = StatusDisconnected
	
	return sm.updateSessionStatus(phoneNumber, StatusDisconnected)
}

func (sm *SessionManager) GetQRCode(phoneNumber string) (string, error) {
	session, err := sm.GetSession(phoneNumber)
	if err != nil {
		return "", err
	}

	if session.Client.Store.ID != nil {
		return "", fmt.Errorf("session already authenticated")
	}

	if !session.Client.IsConnected() {
		err := sm.ConnectSession(phoneNumber)
		if err != nil {
			return "", err
		}
	}

	qrChan, err := session.Client.GetQRChannel(context.Background())
	if err != nil {
		return "", fmt.Errorf("failed to get QR channel: %w", err)
	}

	for evt := range qrChan {
		if evt.Event == "code" {
			session.mu.Lock()
			session.CurrentQR = evt.Code
			session.mu.Unlock()
			return evt.Code, nil
		}
	}

	return "", fmt.Errorf("failed to get QR code")
}

func (sm *SessionManager) PairPhone(phoneNumber, targetPhone string, showNotification bool) (string, error) {
	session, err := sm.GetSession(phoneNumber)
	if err != nil {
		return "", err
	}

	if session.Client.Store.ID != nil {
		return "", fmt.Errorf("session already authenticated")
	}

	if !session.Client.IsConnected() {
		err := sm.ConnectSession(phoneNumber)
		if err != nil {
			return "", err
		}
		time.Sleep(time.Second)
	}

	session.mu.Lock()
	session.Status = StatusAuthenticating
	session.mu.Unlock()

	pairCode, err := session.Client.PairPhone(context.Background(), targetPhone, showNotification, whatsmeow.PairClientChrome, "Chrome (Windows)")
	if err != nil {
		session.mu.Lock()
		session.Status = StatusError
		session.ErrorMessage = err.Error()
		session.mu.Unlock()
		return "", fmt.Errorf("failed to generate pair code: %w", err)
	}

	sm.updateSessionStatus(phoneNumber, StatusAuthenticating)
	return pairCode, nil
}

func (sm *SessionManager) GetSessionStatus(phoneNumber string) (SessionStatus, error) {
	session, err := sm.GetSession(phoneNumber)
	if err != nil {
		return StatusError, err
	}

	session.mu.RLock()
	defer session.mu.RUnlock()
	
	return session.Status, nil
}

func (sm *SessionManager) ListSessions() ([]*database.Session, error) {
	return sm.supabase.ListSessions()
}

func (sm *SessionManager) DeleteSession(phoneNumber string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[phoneNumber]
	if exists {
		session.Client.Disconnect()
		delete(sm.sessions, phoneNumber)
	}

	return sm.supabase.DeleteSession(phoneNumber)
}

func (sm *SessionManager) handleSessionEvent(phoneNumber string, evt interface{}) {
	session, err := sm.GetSession(phoneNumber)
	if err != nil {
		sm.logger.Errorf("Failed to get session for event handling: %v", err)
		return
	}

	switch v := evt.(type) {
	case *events.Message:
		sm.handleMessage(session, v)
	case *events.Receipt:
		sm.handleReceipt(session, v)
	case *events.QR:
		if len(v.Codes) > 0 {
			session.mu.Lock()
			session.CurrentQR = v.Codes[0]
			session.mu.Unlock()
			sm.logger.Infof("QR code updated for %s: %s", phoneNumber, session.CurrentQR)
		}
	case *events.PairSuccess:
		session.mu.Lock()
		session.Status = StatusAuthenticated
		session.mu.Unlock()
		sm.updateSessionStatus(phoneNumber, StatusAuthenticated)
		sm.logger.Infof("Pairing successful for %s! Device: %s, Business: %s, Platform: %s", 
			phoneNumber, v.ID.String(), v.BusinessName, v.Platform)
	case *events.PairError:
		session.mu.Lock()
		session.Status = StatusError
		session.ErrorMessage = v.Error.Error()
		session.mu.Unlock()
		sm.updateSessionStatus(phoneNumber, StatusError)
		sm.logger.Errorf("Pairing failed for %s! Device: %s, Error: %v", phoneNumber, v.ID.String(), v.Error)
	case *events.Connected:
		if session.Client.Store.ID != nil {
			session.mu.Lock()
			session.Status = StatusAuthenticated
			session.mu.Unlock()
			sm.updateSessionStatus(phoneNumber, StatusAuthenticated)
		}
		sm.logger.Infof("WhatsApp client connected successfully for %s!", phoneNumber)
	case *events.Disconnected:
		session.mu.Lock()
		session.Status = StatusDisconnected
		session.mu.Unlock()
		sm.updateSessionStatus(phoneNumber, StatusDisconnected)
		sm.logger.Infof("WhatsApp client disconnected for %s", phoneNumber)
	}
}

func (sm *SessionManager) handleMessage(session *WhatsAppSession, evt *events.Message) {
	msg := &database.Message{
		ID:          evt.Info.ID,
		PhoneNumber: session.PhoneNumber,
		ChatID:      evt.Info.Chat.String(),
		SenderID:    evt.Info.Sender.String(),
		Content:     sm.extractMessageContent(evt),
		Timestamp:   evt.Info.Timestamp,
		IsFromMe:    evt.Info.IsFromMe,
		IsGroup:     evt.Info.IsGroup,
		IsRead:      false,
	}

	err := session.Store.SaveMessage(msg)
	if err != nil {
		sm.logger.Errorf("Failed to save message for %s: %v", session.PhoneNumber, err)
		return
	}

	sm.logger.Infof("Received message for %s: %s from %s", session.PhoneNumber, 
		msg.Content["text"], msg.SenderID)
}

func (sm *SessionManager) handleReceipt(session *WhatsAppSession, evt *events.Receipt) {
	if (evt.Type == types.ReceiptTypeRead || evt.Type == types.ReceiptTypeReadSelf) && len(evt.MessageIDs) > 0 {
		for _, msgID := range evt.MessageIDs {
			err := session.Store.UpdateMessageReadStatus(session.PhoneNumber, msgID, true)
			if err != nil {
				sm.logger.Errorf("Failed to update read status for %s: %v", session.PhoneNumber, err)
			}
		}
	}
}

func (sm *SessionManager) extractMessageContent(evt *events.Message) map[string]interface{} {
	content := make(map[string]interface{})
	
	if evt.Message.GetConversation() != "" {
		content["text"] = evt.Message.GetConversation()
		content["type"] = "text"
	} else if evt.Message.GetExtendedTextMessage() != nil {
		content["text"] = evt.Message.GetExtendedTextMessage().GetText()
		content["type"] = "text"
	} else {
		content["type"] = "other"
	}
	
	return content
}

func (sm *SessionManager) updateSessionStatus(phoneNumber string, status SessionStatus) error {
	dbSession := &database.Session{
		PhoneNumber: phoneNumber,
		AuthStatus:  string(status),
	}
	
	return sm.supabase.UpdateSession(dbSession)
}

func (sm *SessionManager) Close() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for phoneNumber, session := range sm.sessions {
		session.Client.Disconnect()
		sm.logger.Infof("Disconnected session for %s", phoneNumber)
	}

	return sm.supabase.Close()
}