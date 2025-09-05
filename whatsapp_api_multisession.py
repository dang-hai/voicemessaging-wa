from fastapi import FastAPI, HTTPException, Depends
from fastapi.responses import JSONResponse
from pydantic import BaseModel
from typing import List, Optional, Dict, Any
import httpx
import asyncio
from datetime import datetime
import re

app = FastAPI(title="Multi-Session WhatsApp API", description="FastAPI wrapper for multi-session WhatsApp Go service")

GO_SERVICE_URL = "http://localhost:8080"

class CreateSessionRequest(BaseModel):
    phone_number: str

class CreateSessionResponse(BaseModel):
    phone_number: str
    session_id: str
    status: str

class SessionStatusResponse(BaseModel):
    phone_number: str
    status: str
    last_seen: Optional[str] = None
    error: Optional[str] = None

class SessionListResponse(BaseModel):
    sessions: List[Dict[str, Any]]

class Message(BaseModel):
    id: str
    phone_number: str
    chat_id: str
    sender_id: str
    content: Dict[str, Any]
    timestamp: datetime
    is_from_me: bool
    is_group: bool
    is_read: bool
    created_at: datetime

class QRResponse(BaseModel):
    qr: str

class AuthStatus(BaseModel):
    is_authenticated: bool
    phone: Optional[str] = None

class ReadStatusUpdate(BaseModel):
    message_id: str
    read: bool

class MessagesResponse(BaseModel):
    messages: List[Message]

class UnreadCountResponse(BaseModel):
    unread_count: int

class PairPhoneRequest(BaseModel):
    phone_number: str
    show_notification: bool = True

class PairCodeResponse(BaseModel):
    pair_code: str

def validate_phone_number(phone: str) -> str:
    """Validate and normalize phone number format"""
    # Remove any spaces, hyphens, or parentheses
    phone = re.sub(r'[\s\-\(\)\+]', '', phone)
    
    # Add + if not present and doesn't start with 00
    if not phone.startswith(('+', '00')):
        phone = '+' + phone
    elif phone.startswith('00'):
        phone = '+' + phone[2:]
    
    # Validate international format
    if not re.match(r'^\+[1-9]\d{1,14}$', phone):
        raise HTTPException(status_code=400, detail="Invalid international phone number format")
    
    return phone

async def get_http_client():
    return httpx.AsyncClient(timeout=30.0)

@app.get("/")
async def root():
    return {"message": "Multi-Session WhatsApp API", "docs": "/docs"}

@app.get("/health")
async def health_check():
    try:
        async with httpx.AsyncClient() as client:
            response = await client.get(f"{GO_SERVICE_URL}/sessions/list")
            if response.status_code == 200:
                return {"status": "healthy", "go_service": "running"}
            else:
                return {"status": "degraded", "go_service": "issues"}
    except Exception:
        return {"status": "unhealthy", "go_service": "down"}

# Session Management Endpoints
@app.post("/sessions/create", response_model=CreateSessionResponse)
async def create_session(request: CreateSessionRequest):
    """Create a new WhatsApp session for a phone number"""
    phone = validate_phone_number(request.phone_number)
    
    try:
        async with httpx.AsyncClient() as client:
            response = await client.post(
                f"{GO_SERVICE_URL}/sessions/create",
                json={"phone_number": phone}
            )
            if response.status_code == 200:
                return response.json()
            else:
                raise HTTPException(status_code=500, detail="Failed to create session")
    except httpx.RequestError:
        raise HTTPException(status_code=503, detail="Go service unavailable")

@app.get("/sessions/list", response_model=SessionListResponse)
async def list_sessions():
    """List all active sessions"""
    try:
        async with httpx.AsyncClient() as client:
            response = await client.get(f"{GO_SERVICE_URL}/sessions/list")
            if response.status_code == 200:
                return response.json()
            else:
                raise HTTPException(status_code=500, detail="Failed to list sessions")
    except httpx.RequestError:
        raise HTTPException(status_code=503, detail="Go service unavailable")

@app.get("/sessions/{phone}/status", response_model=SessionStatusResponse)
async def get_session_status(phone: str):
    """Get status of a specific session"""
    phone = validate_phone_number(phone)
    
    try:
        async with httpx.AsyncClient() as client:
            response = await client.get(f"{GO_SERVICE_URL}/sessions/{phone}/status")
            if response.status_code == 200:
                return response.json()
            elif response.status_code == 404:
                raise HTTPException(status_code=404, detail="Session not found")
            else:
                raise HTTPException(status_code=500, detail="Failed to get session status")
    except httpx.RequestError:
        raise HTTPException(status_code=503, detail="Go service unavailable")

@app.post("/sessions/{phone}/connect")
async def connect_session(phone: str):
    """Connect a session to WhatsApp"""
    phone = validate_phone_number(phone)
    
    try:
        async with httpx.AsyncClient() as client:
            response = await client.post(f"{GO_SERVICE_URL}/sessions/{phone}/connect")
            if response.status_code == 200:
                return {"message": "Session connected successfully"}
            elif response.status_code == 404:
                raise HTTPException(status_code=404, detail="Session not found")
            else:
                raise HTTPException(status_code=500, detail="Failed to connect session")
    except httpx.RequestError:
        raise HTTPException(status_code=503, detail="Go service unavailable")

@app.post("/sessions/{phone}/disconnect")
async def disconnect_session(phone: str):
    """Disconnect a session from WhatsApp"""
    phone = validate_phone_number(phone)
    
    try:
        async with httpx.AsyncClient() as client:
            response = await client.post(f"{GO_SERVICE_URL}/sessions/{phone}/disconnect")
            if response.status_code == 200:
                return {"message": "Session disconnected successfully"}
            elif response.status_code == 404:
                raise HTTPException(status_code=404, detail="Session not found")
            else:
                raise HTTPException(status_code=500, detail="Failed to disconnect session")
    except httpx.RequestError:
        raise HTTPException(status_code=503, detail="Go service unavailable")

@app.delete("/sessions/{phone}")
async def delete_session(phone: str):
    """Delete a session and all its data"""
    phone = validate_phone_number(phone)
    
    try:
        async with httpx.AsyncClient() as client:
            response = await client.delete(f"{GO_SERVICE_URL}/sessions/{phone}/delete")
            if response.status_code == 200:
                return {"message": "Session deleted successfully"}
            elif response.status_code == 404:
                raise HTTPException(status_code=404, detail="Session not found")
            else:
                raise HTTPException(status_code=500, detail="Failed to delete session")
    except httpx.RequestError:
        raise HTTPException(status_code=503, detail="Go service unavailable")

# Authentication Endpoints (Phone-scoped)
@app.get("/sessions/{phone}/auth/qr", response_model=QRResponse)
async def get_qr_code(phone: str):
    """Get QR code for WhatsApp authentication for a specific session"""
    phone = validate_phone_number(phone)
    
    try:
        async with httpx.AsyncClient() as client:
            response = await client.get(f"{GO_SERVICE_URL}/sessions/{phone}/qr")
            if response.status_code == 200:
                return response.json()
            elif response.status_code == 400:
                raise HTTPException(status_code=400, detail="Already authenticated or QR not available")
            elif response.status_code == 404:
                raise HTTPException(status_code=404, detail="Session not found")
            else:
                raise HTTPException(status_code=500, detail="Failed to get QR code")
    except httpx.RequestError:
        raise HTTPException(status_code=503, detail="Go service unavailable")

@app.get("/sessions/{phone}/auth/status", response_model=AuthStatus)
async def get_auth_status(phone: str):
    """Get authentication status for a specific session"""
    phone = validate_phone_number(phone)
    
    try:
        async with httpx.AsyncClient() as client:
            response = await client.get(f"{GO_SERVICE_URL}/sessions/{phone}/auth/status")
            if response.status_code == 200:
                return response.json()
            elif response.status_code == 404:
                raise HTTPException(status_code=404, detail="Session not found")
            else:
                raise HTTPException(status_code=500, detail="Failed to get auth status")
    except httpx.RequestError:
        raise HTTPException(status_code=503, detail="Go service unavailable")

@app.post("/sessions/{phone}/auth/pair-phone", response_model=PairCodeResponse)
async def pair_phone(phone: str, pair_request: Optional[PairPhoneRequest] = None):
    """Generate pairing code for phone number authentication for a specific session"""
    phone = validate_phone_number(phone)
    
    # Use the session phone number if no specific phone number provided in request
    request_data = {
        "phone_number": phone,
        "show_notification": pair_request.show_notification if pair_request else True
    }
    
    try:
        async with httpx.AsyncClient() as client:
            response = await client.post(
                f"{GO_SERVICE_URL}/sessions/{phone}/auth/pair-phone",
                json=request_data
            )
            if response.status_code == 200:
                return response.json()
            elif response.status_code == 400:
                error_text = response.text
                if "Already authenticated" in error_text:
                    raise HTTPException(status_code=400, detail="Already authenticated")
                else:
                    raise HTTPException(status_code=400, detail="Failed to generate pairing code")
            elif response.status_code == 404:
                raise HTTPException(status_code=404, detail="Session not found")
            else:
                raise HTTPException(status_code=500, detail="Failed to generate pairing code")
    except httpx.RequestError:
        raise HTTPException(status_code=503, detail="Go service unavailable")

# Message Endpoints (Phone-scoped)
@app.get("/sessions/{phone}/messages", response_model=MessagesResponse)
async def get_messages(phone: str, limit: int = 50):
    """Get all messages for a specific session"""
    phone = validate_phone_number(phone)
    
    try:
        async with httpx.AsyncClient() as client:
            response = await client.get(
                f"{GO_SERVICE_URL}/sessions/{phone}/messages",
                params={"limit": limit}
            )
            if response.status_code == 200:
                return response.json()
            elif response.status_code == 404:
                raise HTTPException(status_code=404, detail="Session not found")
            else:
                raise HTTPException(status_code=500, detail="Failed to get messages")
    except httpx.RequestError:
        raise HTTPException(status_code=503, detail="Go service unavailable")

@app.get("/sessions/{phone}/messages/{chat_id}", response_model=MessagesResponse)
async def get_chat_messages(phone: str, chat_id: str, limit: int = 50):
    """Get messages from a specific chat for a specific session"""
    phone = validate_phone_number(phone)
    
    try:
        async with httpx.AsyncClient() as client:
            response = await client.get(
                f"{GO_SERVICE_URL}/sessions/{phone}/messages/{chat_id}",
                params={"limit": limit}
            )
            if response.status_code == 200:
                return response.json()
            elif response.status_code == 404:
                raise HTTPException(status_code=404, detail="Session or chat not found")
            else:
                raise HTTPException(status_code=500, detail="Failed to get chat messages")
    except httpx.RequestError:
        raise HTTPException(status_code=503, detail="Go service unavailable")

@app.post("/sessions/{phone}/messages/read-status")
async def update_read_status(phone: str, read_status: ReadStatusUpdate):
    """Mark a message as read or unread for a specific session"""
    phone = validate_phone_number(phone)
    
    try:
        async with httpx.AsyncClient() as client:
            response = await client.post(
                f"{GO_SERVICE_URL}/sessions/{phone}/messages/read-status",
                json=read_status.dict()
            )
            if response.status_code == 200:
                return {"message": "Read status updated successfully"}
            elif response.status_code == 404:
                raise HTTPException(status_code=404, detail="Session or message not found")
            elif response.status_code == 400:
                raise HTTPException(status_code=400, detail="Invalid request")
            else:
                raise HTTPException(status_code=500, detail="Failed to update read status")
    except httpx.RequestError:
        raise HTTPException(status_code=503, detail="Go service unavailable")

@app.get("/sessions/{phone}/messages/unread-count", response_model=UnreadCountResponse)
async def get_unread_count(phone: str):
    """Get unread message count for a specific session"""
    phone = validate_phone_number(phone)
    
    try:
        async with httpx.AsyncClient() as client:
            response = await client.get(f"{GO_SERVICE_URL}/sessions/{phone}/messages/unread-count")
            if response.status_code == 200:
                return response.json()
            elif response.status_code == 404:
                raise HTTPException(status_code=404, detail="Session not found")
            else:
                raise HTTPException(status_code=500, detail="Failed to get unread count")
    except httpx.RequestError:
        raise HTTPException(status_code=503, detail="Go service unavailable")

@app.get("/sessions/{phone}/chats")
async def get_chats(phone: str):
    """Get list of all chats with latest message info for a specific session"""
    phone = validate_phone_number(phone)
    
    try:
        async with httpx.AsyncClient() as client:
            response = await client.get(f"{GO_SERVICE_URL}/sessions/{phone}/messages")
            if response.status_code == 200:
                data = response.json()
                messages = data.get("messages", [])
                
                # Group messages by chat and get latest message for each
                chats = {}
                for message in messages:
                    chat_id = message["chat_id"]
                    if chat_id not in chats or message["timestamp"] > chats[chat_id]["latest_timestamp"]:
                        chats[chat_id] = {
                            "chat_id": chat_id,
                            "is_group": message["is_group"],
                            "latest_message": message["content"].get("text", f"[{message['content']['type']}]"),
                            "latest_timestamp": message["timestamp"],
                            "unread_count": 0
                        }
                
                # Count unread messages
                for message in messages:
                    chat_id = message["chat_id"]
                    if not message["is_read"] and not message["is_from_me"]:
                        chats[chat_id]["unread_count"] += 1
                
                return {"chats": list(chats.values())}
            elif response.status_code == 404:
                raise HTTPException(status_code=404, detail="Session not found")
            else:
                raise HTTPException(status_code=500, detail="Failed to get chats")
    except httpx.RequestError:
        raise HTTPException(status_code=503, detail="Go service unavailable")

# Legacy endpoints for backward compatibility (using first available session)
@app.get("/auth/status", response_model=AuthStatus, deprecated=True)
async def legacy_get_auth_status():
    """Legacy endpoint - get auth status for first available session"""
    sessions_response = await list_sessions()
    if not sessions_response.sessions:
        raise HTTPException(status_code=404, detail="No sessions available")
    
    first_phone = sessions_response.sessions[0]["phone_number"]
    return await get_auth_status(first_phone)

@app.get("/messages", response_model=MessagesResponse, deprecated=True)
async def legacy_get_messages():
    """Legacy endpoint - get messages for first available session"""
    sessions_response = await list_sessions()
    if not sessions_response.sessions:
        raise HTTPException(status_code=404, detail="No sessions available")
    
    first_phone = sessions_response.sessions[0]["phone_number"]
    return await get_messages(first_phone)

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8081)