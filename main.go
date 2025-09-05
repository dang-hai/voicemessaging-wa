package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"whatsapp-wrapper/database"
	"whatsapp-wrapper/session"

	"github.com/gorilla/mux"
	waLog "go.mau.fi/whatsmeow/util/log"
)

type MultiSessionAPI struct {
	sessionManager *session.SessionManager
	supabase       database.SupabaseStore
	log            waLog.Logger
}

type CreateSessionRequest struct {
	PhoneNumber string `json:"phone_number"`
}

type CreateSessionResponse struct {
	PhoneNumber string `json:"phone_number"`
	SessionID   string `json:"session_id"`
	Status      string `json:"status"`
}

type SessionStatusResponse struct {
	PhoneNumber string `json:"phone_number"`
	Status      string `json:"status"`
	LastSeen    string `json:"last_seen,omitempty"`
	Error       string `json:"error,omitempty"`
}

type QRResponse struct {
	QR string `json:"qr"`
}

type AuthStatusResponse struct {
	IsAuthenticated bool   `json:"is_authenticated"`
	Phone          string `json:"phone,omitempty"`
}

type MessagesResponse struct {
	Messages []*database.Message `json:"messages"`
}

type SessionListResponse struct {
	Sessions []*database.Session `json:"sessions"`
}

type ReadStatusRequest struct {
	MessageID string `json:"message_id"`
	Read      bool   `json:"read"`
}

type PairPhoneRequest struct {
	PhoneNumber    string `json:"phone_number"`
	ShowNotification bool `json:"show_notification"`
}

type PairCodeResponse struct {
	PairCode string `json:"pair_code"`
}

func main() {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://postgres:password@localhost:5432/whatsapp?sslmode=disable"
	}

	supabaseDB, err := database.NewSupabaseDB(databaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to Supabase: %v", err)
	}

	clientLog := waLog.Stdout("SessionManager", "INFO", true)
	sessionManager, err := session.NewSessionManager(supabaseDB, databaseURL, clientLog)
	if err != nil {
		log.Fatalf("Failed to create session manager: %v", err)
	}

	api := &MultiSessionAPI{
		sessionManager: sessionManager,
		supabase:       supabaseDB,
		log:            clientLog,
	}

	router := mux.NewRouter()
	
	// Session management endpoints
	router.HandleFunc("/sessions/create", api.createSession).Methods("POST")
	router.HandleFunc("/sessions/list", api.listSessions).Methods("GET")
	router.HandleFunc("/sessions/{phone}/status", api.getSessionStatus).Methods("GET")
	router.HandleFunc("/sessions/{phone}/connect", api.connectSession).Methods("POST")
	router.HandleFunc("/sessions/{phone}/disconnect", api.disconnectSession).Methods("POST")
	router.HandleFunc("/sessions/{phone}/delete", api.deleteSession).Methods("DELETE")
	
	// Authentication endpoints (phone-scoped)
	router.HandleFunc("/sessions/{phone}/qr", api.getQR).Methods("GET")
	router.HandleFunc("/sessions/{phone}/auth/status", api.getAuthStatus).Methods("GET")
	router.HandleFunc("/sessions/{phone}/auth/pair-phone", api.pairPhone).Methods("POST")
	
	// Message endpoints (phone-scoped)
	router.HandleFunc("/sessions/{phone}/messages", api.getMessages).Methods("GET")
	router.HandleFunc("/sessions/{phone}/messages/{chatId}", api.getChatMessages).Methods("GET")
	router.HandleFunc("/sessions/{phone}/messages/read-status", api.updateReadStatus).Methods("POST")
	router.HandleFunc("/sessions/{phone}/messages/unread-count", api.getUnreadCount).Methods("GET")
	
	server := &http.Server{
		Addr:    ":8080",
		Handler: router,
	}

	go func() {
		log.Println("Starting multi-session WhatsApp API server on :8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	<-c
	log.Println("Shutting down server...")
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	sessionManager.Close()
	server.Shutdown(ctx)
}

// Session management handlers
func (api *MultiSessionAPI) createSession(w http.ResponseWriter, r *http.Request) {
	var req CreateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.PhoneNumber == "" {
		http.Error(w, "Phone number is required", http.StatusBadRequest)
		return
	}

	session, err := api.sessionManager.CreateSession(req.PhoneNumber)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create session: %v", err), http.StatusInternalServerError)
		return
	}

	sessionID := "pending"
	if session.Client.Store.ID != nil {
		sessionID = session.Client.Store.ID.String()
	}

	response := CreateSessionResponse{
		PhoneNumber: session.PhoneNumber,
		SessionID:   sessionID,
		Status:      string(session.Status),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (api *MultiSessionAPI) listSessions(w http.ResponseWriter, r *http.Request) {
	sessions, err := api.sessionManager.ListSessions()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to list sessions: %v", err), http.StatusInternalServerError)
		return
	}

	response := SessionListResponse{Sessions: sessions}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (api *MultiSessionAPI) getSessionStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	phoneNumber := vars["phone"]

	status, err := api.sessionManager.GetSessionStatus(phoneNumber)
	if err != nil {
		http.Error(w, fmt.Sprintf("Session not found: %v", err), http.StatusNotFound)
		return
	}

	session, _ := api.sessionManager.GetSession(phoneNumber)
	response := SessionStatusResponse{
		PhoneNumber: phoneNumber,
		Status:      string(status),
		LastSeen:    session.LastSeen.Format(time.RFC3339),
	}

	if session.ErrorMessage != "" {
		response.Error = session.ErrorMessage
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (api *MultiSessionAPI) connectSession(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	phoneNumber := vars["phone"]

	err := api.sessionManager.ConnectSession(phoneNumber)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to connect session: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (api *MultiSessionAPI) disconnectSession(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	phoneNumber := vars["phone"]

	err := api.sessionManager.DisconnectSession(phoneNumber)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to disconnect session: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (api *MultiSessionAPI) deleteSession(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	phoneNumber := vars["phone"]

	err := api.sessionManager.DeleteSession(phoneNumber)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete session: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// Authentication handlers (phone-scoped)
func (api *MultiSessionAPI) getQR(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	phoneNumber := vars["phone"]

	qrCode, err := api.sessionManager.GetQRCode(phoneNumber)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get QR code: %v", err), http.StatusBadRequest)
		return
	}

	response := QRResponse{QR: qrCode}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (api *MultiSessionAPI) getAuthStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	phoneNumber := vars["phone"]

	session, err := api.sessionManager.GetSession(phoneNumber)
	if err != nil {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	response := AuthStatusResponse{
		IsAuthenticated: session.Status == "authenticated",
		Phone:          phoneNumber,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (api *MultiSessionAPI) pairPhone(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	phoneNumber := vars["phone"]

	var req PairPhoneRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.PhoneNumber == "" {
		req.PhoneNumber = phoneNumber
	}

	pairCode, err := api.sessionManager.PairPhone(phoneNumber, req.PhoneNumber, req.ShowNotification)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to generate pair code: %v", err), http.StatusInternalServerError)
		return
	}

	response := PairCodeResponse{PairCode: pairCode}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Message handlers (phone-scoped)
func (api *MultiSessionAPI) getMessages(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	phoneNumber := vars["phone"]

	limitStr := r.URL.Query().Get("limit")
	limit := 50 // default
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	messages, err := api.supabase.GetMessages(phoneNumber, limit)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get messages: %v", err), http.StatusInternalServerError)
		return
	}

	response := MessagesResponse{Messages: messages}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (api *MultiSessionAPI) getChatMessages(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	phoneNumber := vars["phone"]
	chatID := vars["chatId"]

	limitStr := r.URL.Query().Get("limit")
	limit := 50 // default
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	messages, err := api.supabase.GetChatMessages(phoneNumber, chatID, limit)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get chat messages: %v", err), http.StatusInternalServerError)
		return
	}

	response := MessagesResponse{Messages: messages}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (api *MultiSessionAPI) updateReadStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	phoneNumber := vars["phone"]

	var req ReadStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	err := api.supabase.UpdateMessageReadStatus(phoneNumber, req.MessageID, req.Read)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to update read status: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (api *MultiSessionAPI) getUnreadCount(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	phoneNumber := vars["phone"]

	count, err := api.supabase.GetUnreadMessageCount(phoneNumber)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get unread count: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]int{"unread_count": count}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

