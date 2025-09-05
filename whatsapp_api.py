from fastapi import FastAPI, HTTPException, Depends
from fastapi.responses import JSONResponse
from pydantic import BaseModel
from typing import List, Optional
import httpx
import asyncio
from datetime import datetime

app = FastAPI(title="WhatsApp API Wrapper", description="FastAPI wrapper for WhatsApp Go service")

GO_SERVICE_URL = "http://localhost:8080"

class MessageSource(BaseModel):
    chat: str
    sender: str
    is_from_me: bool
    is_group: bool

class MessageContent(BaseModel):
    text: Optional[str] = None
    type: str

class Message(BaseModel):
    id: str
    timestamp: datetime
    source: MessageSource
    content: MessageContent
    is_read: bool

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

class PairPhoneRequest(BaseModel):
    phone_number: str
    show_notification: bool = True

class PairCodeResponse(BaseModel):
    pair_code: str

async def get_http_client():
    return httpx.AsyncClient(timeout=30.0)

@app.get("/")
async def root():
    return {"message": "WhatsApp API Wrapper", "docs": "/docs"}

@app.get("/health")
async def health_check():
    try:
        async with httpx.AsyncClient() as client:
            response = await client.get(f"{GO_SERVICE_URL}/auth/status")
            if response.status_code == 200:
                return {"status": "healthy", "go_service": "running"}
            else:
                return {"status": "degraded", "go_service": "issues"}
    except Exception:
        return {"status": "unhealthy", "go_service": "down"}

@app.get("/auth/qr", response_model=QRResponse)
async def get_qr_code():
    """Get QR code for WhatsApp authentication"""
    try:
        async with httpx.AsyncClient() as client:
            response = await client.get(f"{GO_SERVICE_URL}/qr")
            if response.status_code == 200:
                return response.json()
            elif response.status_code == 400:
                raise HTTPException(status_code=400, detail="Already authenticated")
            elif response.status_code == 408:
                raise HTTPException(status_code=408, detail="QR generation timeout")
            else:
                raise HTTPException(status_code=500, detail="Failed to get QR code")
    except httpx.RequestError:
        raise HTTPException(status_code=503, detail="Go service unavailable")

@app.get("/auth/status", response_model=AuthStatus)
async def get_auth_status():
    """Get current authentication status"""
    try:
        async with httpx.AsyncClient() as client:
            response = await client.get(f"{GO_SERVICE_URL}/auth/status")
            if response.status_code == 200:
                return response.json()
            else:
                raise HTTPException(status_code=500, detail="Failed to get auth status")
    except httpx.RequestError:
        raise HTTPException(status_code=503, detail="Go service unavailable")

@app.post("/auth/logout")
async def logout():
    """Logout from WhatsApp"""
    try:
        async with httpx.AsyncClient() as client:
            response = await client.post(f"{GO_SERVICE_URL}/auth/logout")
            if response.status_code == 200:
                return {"message": "Logged out successfully"}
            elif response.status_code == 400:
                raise HTTPException(status_code=400, detail="Not authenticated")
            else:
                raise HTTPException(status_code=500, detail="Failed to logout")
    except httpx.RequestError:
        raise HTTPException(status_code=503, detail="Go service unavailable")

@app.post("/auth/pair-phone", response_model=PairCodeResponse)
async def pair_phone(pair_request: PairPhoneRequest):
    """Generate pairing code for phone number authentication"""
    try:
        async with httpx.AsyncClient() as client:
            response = await client.post(
                f"{GO_SERVICE_URL}/auth/pair-phone",
                json=pair_request.dict()
            )
            if response.status_code == 200:
                return response.json()
            elif response.status_code == 400:
                error_text = response.text
                if "Already authenticated" in error_text:
                    raise HTTPException(status_code=400, detail="Already authenticated")
                elif "Phone number is required" in error_text:
                    raise HTTPException(status_code=400, detail="Phone number is required")
                else:
                    raise HTTPException(status_code=400, detail="Invalid phone number format")
            else:
                raise HTTPException(status_code=500, detail="Failed to generate pairing code")
    except httpx.RequestError:
        raise HTTPException(status_code=503, detail="Go service unavailable")

@app.get("/messages", response_model=MessagesResponse)
async def get_messages():
    """Get all messages"""
    try:
        async with httpx.AsyncClient() as client:
            response = await client.get(f"{GO_SERVICE_URL}/messages")
            if response.status_code == 200:
                return response.json()
            elif response.status_code == 401:
                raise HTTPException(status_code=401, detail="Not authenticated")
            else:
                raise HTTPException(status_code=500, detail="Failed to get messages")
    except httpx.RequestError:
        raise HTTPException(status_code=503, detail="Go service unavailable")

@app.get("/messages/{chat_id}", response_model=MessagesResponse)
async def get_chat_messages(chat_id: str):
    """Get messages from a specific chat"""
    try:
        async with httpx.AsyncClient() as client:
            response = await client.get(f"{GO_SERVICE_URL}/messages/{chat_id}")
            if response.status_code == 200:
                return response.json()
            elif response.status_code == 401:
                raise HTTPException(status_code=401, detail="Not authenticated")
            else:
                raise HTTPException(status_code=500, detail="Failed to get chat messages")
    except httpx.RequestError:
        raise HTTPException(status_code=503, detail="Go service unavailable")

@app.post("/messages/read-status")
async def update_read_status(read_status: ReadStatusUpdate):
    """Mark a message as read or unread"""
    try:
        async with httpx.AsyncClient() as client:
            response = await client.post(
                f"{GO_SERVICE_URL}/messages/read-status",
                json=read_status.dict()
            )
            if response.status_code == 200:
                return {"message": "Read status updated successfully"}
            elif response.status_code == 401:
                raise HTTPException(status_code=401, detail="Not authenticated")
            elif response.status_code == 404:
                raise HTTPException(status_code=404, detail="Message not found")
            elif response.status_code == 400:
                raise HTTPException(status_code=400, detail="Invalid request")
            else:
                raise HTTPException(status_code=500, detail="Failed to update read status")
    except httpx.RequestError:
        raise HTTPException(status_code=503, detail="Go service unavailable")

@app.get("/chats")
async def get_chats():
    """Get list of all chats with latest message info"""
    try:
        async with httpx.AsyncClient() as client:
            response = await client.get(f"{GO_SERVICE_URL}/messages")
            if response.status_code == 200:
                data = response.json()
                messages = data.get("messages", [])
                
                # Group messages by chat and get latest message for each
                chats = {}
                for message in messages:
                    chat_id = message["source"]["chat"]
                    if chat_id not in chats or message["timestamp"] > chats[chat_id]["latest_timestamp"]:
                        chats[chat_id] = {
                            "chat_id": chat_id,
                            "is_group": message["source"]["is_group"],
                            "latest_message": message["content"]["text"] or f"[{message['content']['type']}]",
                            "latest_timestamp": message["timestamp"],
                            "unread_count": 0
                        }
                
                # Count unread messages
                for message in messages:
                    chat_id = message["source"]["chat"]
                    if not message["is_read"] and not message["source"]["is_from_me"]:
                        chats[chat_id]["unread_count"] += 1
                
                return {"chats": list(chats.values())}
            elif response.status_code == 401:
                raise HTTPException(status_code=401, detail="Not authenticated")
            else:
                raise HTTPException(status_code=500, detail="Failed to get chats")
    except httpx.RequestError:
        raise HTTPException(status_code=503, detail="Go service unavailable")

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8081)