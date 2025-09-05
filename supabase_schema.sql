-- Supabase Schema for Multi-Session WhatsApp API
-- Enable Row Level Security for all tables

-- Sessions table: Track active WhatsApp sessions
CREATE TABLE IF NOT EXISTS sessions (
    id UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    phone_number TEXT NOT NULL UNIQUE,
    session_id TEXT NOT NULL,
    auth_status TEXT NOT NULL DEFAULT 'pending', -- 'pending', 'authenticated', 'disconnected'
    device_id TEXT,
    business_name TEXT,
    platform TEXT,
    last_seen TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Messages table: Store all WhatsApp messages per session
CREATE TABLE IF NOT EXISTS messages (
    id TEXT PRIMARY KEY, -- WhatsApp message ID
    phone_number TEXT NOT NULL,
    chat_id TEXT NOT NULL,
    sender_id TEXT NOT NULL,
    content JSONB NOT NULL, -- {text: string, type: string, media_url?: string}
    timestamp TIMESTAMP WITH TIME ZONE NOT NULL,
    is_from_me BOOLEAN NOT NULL DEFAULT FALSE,
    is_group BOOLEAN NOT NULL DEFAULT FALSE,
    is_read BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    FOREIGN KEY (phone_number) REFERENCES sessions(phone_number) ON DELETE CASCADE
);

-- Contacts table: Store contact information per session
CREATE TABLE IF NOT EXISTS contacts (
    id UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    phone_number TEXT NOT NULL,
    contact_id TEXT NOT NULL, -- WhatsApp JID
    display_name TEXT,
    push_name TEXT,
    is_business BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    FOREIGN KEY (phone_number) REFERENCES sessions(phone_number) ON DELETE CASCADE,
    UNIQUE(phone_number, contact_id)
);

-- Chat metadata table: Store chat-level information
CREATE TABLE IF NOT EXISTS chat_metadata (
    id UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    phone_number TEXT NOT NULL,
    chat_id TEXT NOT NULL,
    chat_name TEXT,
    is_group BOOLEAN NOT NULL DEFAULT FALSE,
    unread_count INTEGER DEFAULT 0,
    last_message_id TEXT,
    last_message_timestamp TIMESTAMP WITH TIME ZONE,
    muted_until TIMESTAMP WITH TIME ZONE,
    pinned BOOLEAN DEFAULT FALSE,
    archived BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    FOREIGN KEY (phone_number) REFERENCES sessions(phone_number) ON DELETE CASCADE,
    UNIQUE(phone_number, chat_id)
);

-- Device storage table: Store WhatsApp device session data
CREATE TABLE IF NOT EXISTS device_storage (
    id UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    phone_number TEXT NOT NULL,
    key TEXT NOT NULL,
    value BYTEA, -- Binary data for WhatsApp session storage
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    FOREIGN KEY (phone_number) REFERENCES sessions(phone_number) ON DELETE CASCADE,
    UNIQUE(phone_number, key)
);

-- Create indexes for better performance
CREATE INDEX IF NOT EXISTS idx_messages_phone_timestamp ON messages(phone_number, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_messages_chat_timestamp ON messages(phone_number, chat_id, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_messages_unread ON messages(phone_number, is_read) WHERE is_read = FALSE;
CREATE INDEX IF NOT EXISTS idx_sessions_phone ON sessions(phone_number);
CREATE INDEX IF NOT EXISTS idx_contacts_phone_contact ON contacts(phone_number, contact_id);
CREATE INDEX IF NOT EXISTS idx_chat_metadata_phone ON chat_metadata(phone_number);

-- Enable Row Level Security
ALTER TABLE sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE messages ENABLE ROW LEVEL SECURITY;
ALTER TABLE contacts ENABLE ROW LEVEL SECURITY;
ALTER TABLE chat_metadata ENABLE ROW LEVEL SECURITY;
ALTER TABLE device_storage ENABLE ROW LEVEL SECURITY;

-- RLS Policies: Users can only access their own data
-- Note: In production, you'd use auth.uid() or custom session tokens

-- Sessions policies
CREATE POLICY "Users can view own sessions" ON sessions
    FOR SELECT USING (auth.jwt() ->> 'phone_number' = phone_number);

CREATE POLICY "Users can insert own sessions" ON sessions
    FOR INSERT WITH CHECK (auth.jwt() ->> 'phone_number' = phone_number);

CREATE POLICY "Users can update own sessions" ON sessions
    FOR UPDATE USING (auth.jwt() ->> 'phone_number' = phone_number);

-- Messages policies  
CREATE POLICY "Users can view own messages" ON messages
    FOR SELECT USING (auth.jwt() ->> 'phone_number' = phone_number);

CREATE POLICY "Users can insert own messages" ON messages
    FOR INSERT WITH CHECK (auth.jwt() ->> 'phone_number' = phone_number);

CREATE POLICY "Users can update own messages" ON messages
    FOR UPDATE USING (auth.jwt() ->> 'phone_number' = phone_number);

-- Contacts policies
CREATE POLICY "Users can view own contacts" ON contacts
    FOR SELECT USING (auth.jwt() ->> 'phone_number' = phone_number);

CREATE POLICY "Users can manage own contacts" ON contacts
    FOR ALL USING (auth.jwt() ->> 'phone_number' = phone_number);

-- Chat metadata policies
CREATE POLICY "Users can view own chat metadata" ON chat_metadata
    FOR SELECT USING (auth.jwt() ->> 'phone_number' = phone_number);

CREATE POLICY "Users can manage own chat metadata" ON chat_metadata
    FOR ALL USING (auth.jwt() ->> 'phone_number' = phone_number);

-- Device storage policies
CREATE POLICY "Users can view own device data" ON device_storage
    FOR SELECT USING (auth.jwt() ->> 'phone_number' = phone_number);

CREATE POLICY "Users can manage own device data" ON device_storage
    FOR ALL USING (auth.jwt() ->> 'phone_number' = phone_number);

-- Functions for common operations
CREATE OR REPLACE FUNCTION update_session_last_seen()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE sessions 
    SET last_seen = NOW(), updated_at = NOW()
    WHERE phone_number = NEW.phone_number;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to update last_seen when messages are received
CREATE TRIGGER update_session_last_seen_trigger
    AFTER INSERT ON messages
    FOR EACH ROW
    EXECUTE FUNCTION update_session_last_seen();

-- Function to update unread counts
CREATE OR REPLACE FUNCTION update_unread_count()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.is_read != OLD.is_read THEN
        UPDATE chat_metadata 
        SET unread_count = (
            SELECT COUNT(*) 
            FROM messages 
            WHERE phone_number = NEW.phone_number 
              AND chat_id = NEW.chat_id 
              AND is_read = FALSE 
              AND is_from_me = FALSE
        )
        WHERE phone_number = NEW.phone_number AND chat_id = NEW.chat_id;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to update unread counts when message read status changes
CREATE TRIGGER update_unread_count_trigger
    AFTER UPDATE ON messages
    FOR EACH ROW
    EXECUTE FUNCTION update_unread_count();

-- Views for common queries
CREATE OR REPLACE VIEW session_summary AS
SELECT 
    s.phone_number,
    s.auth_status,
    s.last_seen,
    COUNT(m.id) as total_messages,
    COUNT(CASE WHEN NOT m.is_read AND NOT m.is_from_me THEN 1 END) as unread_messages,
    COUNT(DISTINCT m.chat_id) as active_chats
FROM sessions s
LEFT JOIN messages m ON s.phone_number = m.phone_number
GROUP BY s.phone_number, s.auth_status, s.last_seen;