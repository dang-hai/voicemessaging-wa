# WhatsApp API Wrapper

A FastAPI Python web server that wraps the Go whatsmeow library for WhatsApp Web integration.

## Features

- **Authentication**: QR code generation, phone number pairing, auth status, logout
- **Message Management**: List all messages, get chat-specific messages  
- **Read Status**: Mark messages as read/unread
- **Chat Overview**: Get list of chats with unread counts

## Authentication Methods

The API supports two authentication methods:

### 1. QR Code Authentication
- Generate a QR code using `/auth/qr`
- Scan the QR code with WhatsApp on your phone
- Instant authentication without entering phone numbers

### 2. Phone Number Pairing
- Send your phone number to `/auth/pair-phone`  
- Receive a 8-digit pairing code
- Enter the pairing code in WhatsApp Settings → Linked Devices → Link a Device
- More convenient for mobile users who don't want to scan QR codes

## Architecture

The system consists of two services:

1. **Go Service** (port 8080): Direct whatsmeow library wrapper
2. **Python FastAPI** (port 8081): User-friendly REST API

## Setup

1. Make setup script executable and run:
```bash
chmod +x setup.sh run-go.sh run-python.sh
./setup.sh
```

This will:
- Install Go dependencies
- Create a Python virtual environment (`venv/`)
- Install Python dependencies in the virtual environment

## Running

1. Start the Go service (in one terminal):
```bash
./run-go.sh
```

2. Start the Python API (in another terminal):
```bash
./run-python.sh
```

## API Endpoints

### Authentication
- `GET /auth/qr` - Get QR code for authentication
- `POST /auth/pair-phone` - Generate pairing code for phone number authentication
- `GET /auth/status` - Check authentication status  
- `POST /auth/logout` - Logout from WhatsApp

### Messages
- `GET /messages` - Get all messages
- `GET /messages/{chat_id}` - Get messages from specific chat
- `POST /messages/read-status` - Mark message as read/unread

### Chats
- `GET /chats` - Get list of all chats with unread counts

### System
- `GET /health` - Health check
- `GET /docs` - API documentation (Swagger UI)

## Usage Example

1. **Authenticate with QR code**:
   ```bash
   curl http://localhost:8081/auth/qr
   # Scan the QR code with WhatsApp
   ```

2. **Alternative: Authenticate with phone number**:
   ```bash
   curl -X POST http://localhost:8081/auth/pair-phone \
     -H "Content-Type: application/json" \
     -d '{"phone_number": "+1234567890", "show_notification": true}'
   # Enter the returned pairing code in WhatsApp on your phone
   ```

3. **Check auth status**:
   ```bash
   curl http://localhost:8081/auth/status
   ```

3. **Get messages**:
   ```bash
   curl http://localhost:8081/messages
   ```

4. **Mark message as read**:
   ```bash
   curl -X POST http://localhost:8081/messages/read-status \
     -H "Content-Type: application/json" \
     -d '{"message_id": "MESSAGE_ID", "read": true}'
   ```

## Database

The Go service uses SQLite to store WhatsApp session data in `whatsapp.db`.

## Dependencies

### Go
- whatsmeow library
- gorilla/mux for HTTP routing
- SQLite for data storage

### Python
- FastAPI for web framework
- httpx for HTTP client
- uvicorn for ASGI server