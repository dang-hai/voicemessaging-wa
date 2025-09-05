# Multi-Session WhatsApp API

A FastAPI Python web server that wraps a multi-session Go whatsmeow service for WhatsApp Web integration supporting unlimited phone numbers with complete data isolation.

## 🚀 Features

- **Multi-Session Support**: Manage unlimited WhatsApp sessions simultaneously
- **Complete Data Isolation**: Each phone number has isolated messages, contacts, and settings
- **Session Management**: Create, connect, disconnect, and delete sessions
- **Authentication Methods**: QR code generation and phone number pairing per session
- **Message Management**: Per-session message retrieval and management
- **Supabase Integration**: PostgreSQL database with Row Level Security
- **Real-time Events**: WhatsApp event handling per session
- **High Availability**: Cloud-native architecture with session persistence

## 🏗️ Architecture

### Multi-Session Architecture
```
[Session Manager] → [Multiple WhatsApp Clients] → [Supabase Database] → [Phone-Isolated Data]
```

The system consists of three main components:

1. **Go Multi-Session Service** (port 8080): Session manager with multiple whatsmeow clients
2. **Python FastAPI** (port 8081): User-friendly REST API wrapper
3. **Supabase Database**: PostgreSQL with Row Level Security for data isolation

### Session Isolation
- Each phone number gets its own WhatsApp client instance
- Complete database isolation using phone number as tenant key
- Independent authentication state per session
- Isolated message storage and contact management

## 📦 Setup

### Prerequisites
- Go 1.24+ 
- Python 3.8+
- PostgreSQL database (Supabase recommended)

### 1. Database Setup
Set up Supabase or PostgreSQL and run the schema:
```bash
psql -h your-db-host -U postgres -d your-database -f supabase_schema.sql
```

### 2. Environment Configuration
```bash
export DATABASE_URL="postgresql://user:password@host:port/database?sslmode=require"
```

### 3. Installation
```bash
chmod +x setup.sh run-go.sh run-python.sh
./setup.sh
```

This will:
- Install Go dependencies with Supabase support
- Create Python virtual environment
- Install Python dependencies

## 🚀 Running

### Start the Go Multi-Session Service
```bash
DATABASE_URL="your-db-url" ./run-go.sh
```

### Start the Python API Wrapper  
```bash
./run-python.sh
```

The Python API will be available at `http://localhost:8081/docs`

## 📚 API Endpoints

### Session Management
- `POST /sessions/create` - Create new session for phone number
- `GET /sessions/list` - List all active sessions
- `GET /sessions/{phone}/status` - Get session status
- `POST /sessions/{phone}/connect` - Connect session to WhatsApp
- `POST /sessions/{phone}/disconnect` - Disconnect session
- `DELETE /sessions/{phone}` - Delete session and all data

### Authentication (Per Session)
- `GET /sessions/{phone}/auth/qr` - Get QR code for session
- `POST /sessions/{phone}/auth/pair-phone` - Generate pairing code
- `GET /sessions/{phone}/auth/status` - Check session auth status

### Messages (Per Session)
- `GET /sessions/{phone}/messages` - Get messages for session
- `GET /sessions/{phone}/messages/{chat_id}` - Get chat messages
- `POST /sessions/{phone}/messages/read-status` - Update read status
- `GET /sessions/{phone}/messages/unread-count` - Get unread count

### Chats (Per Session)
- `GET /sessions/{phone}/chats` - Get chats with metadata

### System
- `GET /health` - Health check
- `GET /docs` - API documentation

## 💻 Usage Examples

### 1. Create and Authenticate Session
```bash
# Create session
curl -X POST http://localhost:8081/sessions/create \
  -H "Content-Type: application/json" \
  -d '{"phone_number": "+1234567890"}'

# Authenticate via QR code
curl http://localhost:8081/sessions/+1234567890/auth/qr

# Or authenticate via phone pairing
curl -X POST http://localhost:8081/sessions/+1234567890/auth/pair-phone \
  -H "Content-Type: application/json" \
  -d '{"show_notification": true}'
```

### 2. Manage Multiple Sessions
```bash
# Create multiple sessions
curl -X POST http://localhost:8081/sessions/create -d '{"phone_number": "+1234567890"}'
curl -X POST http://localhost:8081/sessions/create -d '{"phone_number": "+0987654321"}'

# List all sessions
curl http://localhost:8081/sessions/list

# Get session status
curl http://localhost:8081/sessions/+1234567890/status
```

### 3. Message Management
```bash
# Get messages for specific session
curl http://localhost:8081/sessions/+1234567890/messages

# Get messages from specific chat
curl http://localhost:8081/sessions/+1234567890/messages/CHAT_ID

# Mark message as read
curl -X POST http://localhost:8081/sessions/+1234567890/messages/read-status \
  -H "Content-Type: application/json" \
  -d '{"message_id": "MESSAGE_ID", "read": true}'
```

### 4. Session Lifecycle Management
```bash
# Connect session
curl -X POST http://localhost:8081/sessions/+1234567890/connect

# Disconnect session
curl -X POST http://localhost:8081/sessions/+1234567890/disconnect

# Delete session and all data
curl -X DELETE http://localhost:8081/sessions/+1234567890
```

## 🗄️ Database Schema

The system uses the following tables with phone-number based isolation:

- **sessions**: Track active WhatsApp sessions
- **messages**: Store all messages with phone isolation  
- **contacts**: Contact information per session
- **chat_metadata**: Chat-level information and unread counts
- **device_storage**: WhatsApp session persistence data

Each table includes Row Level Security policies ensuring data isolation.

## 🔒 Security Features

- **Row Level Security**: Database-level isolation by phone number
- **Phone Number Validation**: International format validation
- **Session Isolation**: Complete separation between phone numbers
- **Data Privacy**: No cross-session data access possible

## 📈 Scalability

- **Horizontal Scaling**: Multiple service instances share database
- **Session Persistence**: WhatsApp sessions survive service restarts
- **Cloud Native**: Designed for containerized deployment
- **Load Balancing**: Route sessions by phone number hash

## 🔧 Configuration

### Environment Variables
```bash
DATABASE_URL=postgresql://postgres:password@host:port/database
SUPABASE_URL=https://your-project.supabase.co
SUPABASE_ANON_KEY=your-anon-key
```

### Database Connection
The service automatically connects to PostgreSQL using the DATABASE_URL environment variable.

## 📁 Project Structure

```
├── main.go                          # Multi-session Go service
├── database/
│   └── supabase.go                 # Database layer with CRUD operations  
├── session/
│   └── manager.go                  # Session manager for multiple clients
├── whatsapp_api_multisession.py    # Multi-session Python API wrapper
├── supabase_schema.sql             # Database schema with RLS
├── MULTI_SESSION_PLAN.md           # Detailed implementation plan
└── README_MULTISESSION.md          # This file
```

## 🆚 Legacy Compatibility

The API includes deprecated legacy endpoints for backward compatibility:
- `GET /auth/status` - Uses first available session
- `GET /messages` - Uses first available session

## 🚦 Migration from Single Session

1. Deploy new multi-session architecture
2. Run database migrations (supabase_schema.sql)
3. Update client applications to use new session endpoints
4. Migrate existing session data to new format
5. Remove legacy single-session code

## 🛠️ Dependencies

### Go Dependencies
- `go.mau.fi/whatsmeow` - WhatsApp multidevice library
- `github.com/gorilla/mux` - HTTP routing
- `github.com/lib/pq` - PostgreSQL driver

### Python Dependencies  
- `fastapi` - Web framework
- `httpx` - HTTP client
- `uvicorn` - ASGI server
- `pydantic` - Data validation

## 📄 License

This project is part of the voice messaging application and follows the same licensing terms.

## 🤝 Contributing

1. Review the MULTI_SESSION_PLAN.md for architecture details
2. Test changes against multiple concurrent sessions
3. Ensure data isolation between phone numbers
4. Update documentation for API changes

## 🔗 Related Files

- `MULTI_SESSION_PLAN.md` - Detailed implementation plan and architecture
- `supabase_schema.sql` - Complete database schema with RLS policies
- Original single-session files preserved for reference