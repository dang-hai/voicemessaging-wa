# Multi-Session WhatsApp API with Supabase

## Overview
Transform the single-session WhatsApp API into a multi-tenant system supporting unlimited phone numbers with complete data isolation using Supabase as the backend database.

## Architecture Changes

### Current (Single Session)
```
[Single WhatsApp Client] → [Local SQLite] → [Single Phone Messages]
```

### Target (Multi-Session)
```
[Session Manager] → [Multiple WhatsApp Clients] → [Supabase Database] → [Phone-Isolated Data]
```

## Implementation Plan

### Phase 1: Database Migration to Supabase

#### 1.1 Supabase Setup
- [ ] Create Supabase project
- [ ] Run `supabase_schema.sql` to create tables
- [ ] Configure environment variables
- [ ] Set up Row Level Security policies

#### 1.2 Go Service Database Layer
```go
// New database interface
type SupabaseStore interface {
    // Session management
    CreateSession(phoneNumber string) (*Session, error)
    GetSession(phoneNumber string) (*Session, error)
    UpdateSessionStatus(phoneNumber string, status string) error
    
    // Message operations
    SaveMessage(phoneNumber string, msg *MessageInfo) error
    GetMessages(phoneNumber string) ([]*MessageInfo, error)
    GetChatMessages(phoneNumber, chatID string) ([]*MessageInfo, error)
    UpdateMessageReadStatus(phoneNumber, messageID string, isRead bool) error
    
    // Contact management
    SaveContact(phoneNumber string, contact *Contact) error
    GetContacts(phoneNumber string) ([]*Contact, error)
}
```

### Phase 2: Session Manager Implementation

#### 2.1 Session Manager Structure
```go
type SessionManager struct {
    sessions map[string]*WhatsAppSession
    supabase *SupabaseStore
    mu       sync.RWMutex
}

type WhatsAppSession struct {
    PhoneNumber string
    Client      *whatsmeow.Client
    Store       *SupabaseDeviceStore
    Messages    chan *MessageInfo
    Status      SessionStatus
    LastSeen    time.Time
}

type SessionStatus string
const (
    StatusPending        SessionStatus = "pending"
    StatusAuthenticated  SessionStatus = "authenticated"  
    StatusDisconnected   SessionStatus = "disconnected"
)
```

#### 2.2 Key Methods
```go
func (sm *SessionManager) CreateSession(phoneNumber string) error
func (sm *SessionManager) GetSession(phoneNumber string) (*WhatsAppSession, error)
func (sm *SessionManager) AuthenticateSession(phoneNumber string) error
func (sm *SessionManager) CloseSession(phoneNumber string) error
func (sm *SessionManager) GetSessionStatus(phoneNumber string) SessionStatus
```

### Phase 3: API Endpoint Updates

#### 3.1 New Endpoint Structure
```
OLD: /auth/pair-phone
NEW: /sessions/{phone}/auth/pair-phone

OLD: /messages
NEW: /sessions/{phone}/messages

OLD: /auth/status
NEW: /sessions/{phone}/auth/status
```

#### 3.2 Authentication & Authorization
```go
// JWT-based session authentication
type SessionToken struct {
    PhoneNumber string `json:"phone_number"`
    SessionID   string `json:"session_id"`
    ExpiresAt   int64  `json:"exp"`
}

// Middleware for session validation
func ValidateSessionToken(next http.Handler) http.Handler
func GetPhoneFromToken(r *http.Request) string
```

### Phase 4: Python API Updates

#### 4.1 New FastAPI Structure
```python
# Multi-session endpoints
@app.post("/sessions/create")
async def create_session(phone: str) -> SessionResponse

@app.post("/sessions/{phone}/auth/pair-phone")  
async def pair_phone(phone: str, request: PairPhoneRequest)

@app.get("/sessions/{phone}/messages")
async def get_messages(phone: str) -> MessagesResponse

@app.get("/sessions/list")
async def list_sessions() -> List[SessionSummary]

# Session management
@app.post("/sessions/{phone}/connect")
@app.post("/sessions/{phone}/disconnect") 
@app.get("/sessions/{phone}/status")
```

#### 4.2 Session Validation
```python
async def validate_session_access(phone: str, request: Request):
    token = extract_session_token(request)
    if token.phone_number != phone:
        raise HTTPException(401, "Unauthorized access to session")
```

## Security & Isolation

### Row Level Security (RLS)
- Each table filtered by `phone_number`
- JWT tokens contain phone number claims
- Database-level access control

### API Security
```python
# Phone number validation
def validate_phone_number(phone: str) -> str:
    if not re.match(r'^\+[1-9]\d{1,14}$', phone):
        raise ValueError("Invalid international phone number")
    return phone

# Session token generation
def generate_session_token(phone: str) -> str:
    payload = {
        "phone_number": phone,
        "session_id": str(uuid.uuid4()),
        "exp": datetime.utcnow() + timedelta(days=30)
    }
    return jwt.encode(payload, SECRET_KEY)
```

## Message Flow

### Multi-Session Message Handling
```go
func (session *WhatsAppSession) handleMessage(evt *events.Message) {
    msg := &MessageInfo{
        ID:          evt.Info.ID,
        PhoneNumber: session.PhoneNumber, // Key addition for isolation
        ChatID:      evt.Info.Chat.String(),
        SenderID:    evt.Info.Sender.String(),
        Content:     extractContent(evt.Message),
        Timestamp:   evt.Info.Timestamp,
        IsFromMe:    evt.Info.IsFromMe,
        IsGroup:     evt.Info.IsGroup,
    }
    
    // Save to Supabase with phone isolation
    session.Manager.Supabase.SaveMessage(session.PhoneNumber, msg)
}
```

## Configuration

### Environment Variables
```bash
SUPABASE_URL=https://your-project.supabase.co
SUPABASE_ANON_KEY=your-anon-key
SUPABASE_SERVICE_KEY=your-service-key
JWT_SECRET=your-jwt-secret
DATABASE_URL=postgresql://postgres:[password]@db.supabase.co:5432/postgres
```

### Deployment Considerations
- **Horizontal Scaling**: Multiple Go service instances can share Supabase
- **Load Balancing**: Route sessions based on phone number hash
- **Session Persistence**: WhatsApp sessions survive service restarts via Supabase
- **Real-time Updates**: Supabase subscriptions for live message delivery

## Migration Strategy

1. **Deploy Schema**: Run SQL migrations on Supabase
2. **Update Dependencies**: Add Supabase Go/Python clients  
3. **Parallel Implementation**: Build new endpoints alongside existing ones
4. **Data Migration**: Export existing SQLite data to Supabase
5. **Gradual Cutover**: Switch endpoints one by one
6. **Cleanup**: Remove SQLite dependencies

## Benefits

✅ **Multi-Tenant**: Unlimited phone numbers
✅ **Data Isolation**: Complete privacy between phones  
✅ **Scalability**: Cloud-native PostgreSQL performance
✅ **Real-time**: Live message updates via Supabase
✅ **High Availability**: No single points of failure
✅ **Session Persistence**: Survive service restarts
✅ **Analytics**: Rich querying capabilities
✅ **Compliance**: Row-level security for data protection

## Example Usage

### Create Session
```bash
curl -X POST http://localhost:8081/sessions/create \
  -H "Content-Type: application/json" \
  -d '{"phone_number": "+4917634590226"}'
```

### Authenticate Session  
```bash
curl -X POST http://localhost:8081/sessions/+4917634590226/auth/pair-phone \
  -H "Authorization: Bearer YOUR_SESSION_TOKEN" \
  -d '{"show_notification": true}'
```

### Get Messages
```bash
curl http://localhost:8081/sessions/+4917634590226/messages \
  -H "Authorization: Bearer YOUR_SESSION_TOKEN"
```

This architecture enables a production-ready WhatsApp API service that can handle thousands of concurrent phone number sessions with complete data isolation and enterprise-grade security.