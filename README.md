# WhatsApp API Wrapper

A FastAPI Python web server that wraps the Go whatsmeow library for WhatsApp Web integration.

## Features

- **Authentication**: QR code generation, auth status, logout
- **Message Management**: List all messages, get chat-specific messages
- **Read Status**: Mark messages as read/unread
- **Chat Overview**: Get list of chats with unread counts

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

1. **Authenticate**:
   ```bash
   curl http://localhost:8081/auth/qr
   # Scan the QR code with WhatsApp
   ```

2. **Check auth status**:
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