package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
)

type WhatsAppAPI struct {
	client     *whatsmeow.Client
	log        waLog.Logger
	messages   []MessageInfo
	currentQR  string
}

type MessageInfo struct {
	ID        string          `json:"id"`
	Timestamp time.Time       `json:"timestamp"`
	Source    MessageSource   `json:"source"`
	Content   MessageContent  `json:"content"`
	IsRead    bool           `json:"is_read"`
}

type MessageSource struct {
	Chat     string `json:"chat"`
	Sender   string `json:"sender"`
	IsFromMe bool   `json:"is_from_me"`
	IsGroup  bool   `json:"is_group"`
}

type MessageContent struct {
	Text string `json:"text,omitempty"`
	Type string `json:"type"`
}

type QRResponse struct {
	QR string `json:"qr"`
}

type AuthStatusResponse struct {
	IsAuthenticated bool   `json:"is_authenticated"`
	Phone          string `json:"phone,omitempty"`
}

type MessagesResponse struct {
	Messages []MessageInfo `json:"messages"`
}

type ReadStatusRequest struct {
	MessageID string `json:"message_id"`
	Read      bool   `json:"read"`
}

func main() {
	dbLog := waLog.Stdout("Database", "INFO", true)
	container, err := sqlstore.New(context.Background(), "sqlite3", "file:whatsapp.db?_foreign_keys=on", dbLog)
	if err != nil {
		panic(err)
	}

	deviceStore, err := container.GetFirstDevice(context.Background())
	if err != nil {
		panic(err)
	}

	clientLog := waLog.Stdout("Client", "INFO", true)
	client := whatsmeow.NewClient(deviceStore, clientLog)

	api := &WhatsAppAPI{
		client:    client,
		log:       clientLog,
		messages:  make([]MessageInfo, 0),
		currentQR: "",
	}

	client.AddEventHandler(api.eventHandler)

	router := mux.NewRouter()
	
	// Authentication endpoints
	router.HandleFunc("/qr", api.getQR).Methods("GET")
	router.HandleFunc("/auth/status", api.getAuthStatus).Methods("GET")
	router.HandleFunc("/auth/logout", api.logout).Methods("POST")
	
	// Message endpoints
	router.HandleFunc("/messages", api.getMessages).Methods("GET")
	router.HandleFunc("/messages/{chatId}", api.getChatMessages).Methods("GET")
	router.HandleFunc("/messages/read-status", api.updateReadStatus).Methods("POST")
	
	server := &http.Server{
		Addr:    ":8080",
		Handler: router,
	}

	go func() {
		log.Println("Starting server on :8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	if client.Store.ID == nil {
		qrChan, _ := client.GetQRChannel(context.Background())
		err = client.Connect()
		if err != nil {
			panic(err)
		}
		for evt := range qrChan {
			if evt.Event == "code" {
				api.log.Infof("QR code: %s", evt.Code)
			} else {
				api.log.Infof("QR channel result: %s", evt.Event)
			}
		}
	} else {
		err = client.Connect()
		if err != nil {
			panic(err)
		}
	}

	<-c
	log.Println("Shutting down server...")
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	client.Disconnect()
	server.Shutdown(ctx)
}

func (api *WhatsAppAPI) eventHandler(evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:
		api.handleMessage(v)
	case *events.Receipt:
		api.handleReceipt(v)
	case *events.QR:
		if len(v.Codes) > 0 {
			api.currentQR = v.Codes[0]
			api.log.Infof("QR code updated: %s", api.currentQR)
		}
	}
}

func (api *WhatsAppAPI) handleMessage(evt *events.Message) {
	msg := MessageInfo{
		ID:        evt.Info.ID,
		Timestamp: evt.Info.Timestamp,
		Source: MessageSource{
			Chat:     evt.Info.Chat.String(),
			Sender:   evt.Info.Sender.String(),
			IsFromMe: evt.Info.IsFromMe,
			IsGroup:  evt.Info.IsGroup,
		},
		IsRead: false,
	}

	if evt.Message.GetConversation() != "" {
		msg.Content = MessageContent{
			Text: evt.Message.GetConversation(),
			Type: "text",
		}
	} else if evt.Message.GetExtendedTextMessage() != nil {
		msg.Content = MessageContent{
			Text: evt.Message.GetExtendedTextMessage().GetText(),
			Type: "text",
		}
	} else {
		msg.Content = MessageContent{
			Type: "other",
		}
	}

	api.messages = append(api.messages, msg)
	api.log.Infof("Received message: %s from %s", msg.Content.Text, msg.Source.Sender)
}

func (api *WhatsAppAPI) handleReceipt(evt *events.Receipt) {
	if evt.Type == types.ReceiptTypeRead || evt.Type == types.ReceiptTypeReadSelf {
		for i, msg := range api.messages {
			if msg.ID == evt.MessageIDs[0] {
				api.messages[i].IsRead = true
				break
			}
		}
	}
}

func (api *WhatsAppAPI) getQR(w http.ResponseWriter, r *http.Request) {
	if api.client.Store.ID != nil {
		http.Error(w, "Already authenticated", http.StatusBadRequest)
		return
	}

	if api.currentQR == "" {
		http.Error(w, "QR code not available", http.StatusNotFound)
		return
	}

	response := QRResponse{QR: api.currentQR}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (api *WhatsAppAPI) getAuthStatus(w http.ResponseWriter, r *http.Request) {
	response := AuthStatusResponse{
		IsAuthenticated: api.client.Store.ID != nil,
	}
	
	if response.IsAuthenticated && api.client.Store.ID != nil {
		response.Phone = api.client.Store.ID.User
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (api *WhatsAppAPI) logout(w http.ResponseWriter, r *http.Request) {
	if api.client.Store.ID == nil {
		http.Error(w, "Not authenticated", http.StatusBadRequest)
		return
	}

	err := api.client.Logout(context.Background())
	if err != nil {
		http.Error(w, "Failed to logout", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (api *WhatsAppAPI) getMessages(w http.ResponseWriter, r *http.Request) {
	if api.client.Store.ID == nil {
		http.Error(w, "Not authenticated", http.StatusUnauthorized)
		return
	}

	response := MessagesResponse{Messages: api.messages}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (api *WhatsAppAPI) getChatMessages(w http.ResponseWriter, r *http.Request) {
	if api.client.Store.ID == nil {
		http.Error(w, "Not authenticated", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	chatId := vars["chatId"]

	var chatMessages []MessageInfo
	for _, msg := range api.messages {
		if msg.Source.Chat == chatId {
			chatMessages = append(chatMessages, msg)
		}
	}

	response := MessagesResponse{Messages: chatMessages}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (api *WhatsAppAPI) updateReadStatus(w http.ResponseWriter, r *http.Request) {
	if api.client.Store.ID == nil {
		http.Error(w, "Not authenticated", http.StatusUnauthorized)
		return
	}

	var req ReadStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	for i, msg := range api.messages {
		if msg.ID == req.MessageID {
			api.messages[i].IsRead = req.Read
			
			if req.Read {
				chatJID, err := types.ParseJID(msg.Source.Chat)
				if err != nil {
					http.Error(w, "Invalid chat JID", http.StatusBadRequest)
					return
				}

				senderJID, err := types.ParseJID(msg.Source.Sender)
				if err != nil {
					http.Error(w, "Invalid sender JID", http.StatusBadRequest)
					return
				}

				err = api.client.MarkRead([]string{req.MessageID}, time.Now(), chatJID, senderJID)
				if err != nil {
					http.Error(w, "Failed to mark as read", http.StatusInternalServerError)
					return
				}
			}
			
			w.WriteHeader(http.StatusOK)
			return
		}
	}

	http.Error(w, "Message not found", http.StatusNotFound)
}