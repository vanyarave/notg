# Messenger WebSocket Protocol

This document describes the realtime WebSocket protocol used by the messenger server.

All messages are JSON objects.

---

# Connection

Clients must authenticate before opening a WebSocket connection.

Step 1: login

POST /login

Request:

{
  "username": "alex"
}

Response:

{
  "token": "SESSION_TOKEN",
  "user_id": 1
}

Step 2: open WebSocket connection

ws://localhost:8080/ws?token=SESSION_TOKEN

The server validates the session token and attaches the user to the connection.

---

# Message Structure

All websocket messages follow this structure:

{
  "type": "MESSAGE_TYPE",
  "chat_id": 1,
  "text": "message text"
}

Fields:

type  
Defines the message type.

chat_id  
Chat identifier.

text  
Message content (optional depending on type).

---

# Message Types

## join

Client joins a chat.

Request:

{
  "type": "join",
  "chat_id": 1
}

Server registers the client in the chat and sends history.

---

## chat

Normal chat message.

Client → Server

{
  "type": "chat",
  "chat_id": 1,
  "text": "hello"
}

Server behavior:

1. Save message in database.
2. Broadcast message to all chat members.

Broadcast format:

{
  "type": "chat",
  "chat_id": 1,
  "user_id": 5,
  "text": "hello",
  "created_at": "2026-03-14T15:30:00Z"
}

---

## history

Server sends chat history after join.

Server → Client

{
  "type": "history",
  "chat_id": 1,
  "messages": [
    {
      "user_id": 2,
      "text": "hello",
      "created_at": "2026-03-14T15:00:00Z"
    }
  ]
}

---

## user_join

Presence event when a user enters the chat.

Server → Clients

{
  "type": "user_join",
  "chat_id": 1,
  "user_id": 5
}

---

## user_leave

Presence event when a user leaves the chat.

Server → Clients

{
  "type": "user_leave",
  "chat_id": 1,
  "user_id": 5
}

---

# Optional Events

These events can be implemented later.

---

## typing_start

User started typing.

{
  "type": "typing_start",
  "chat_id": 1
}

---

## typing_stop

User stopped typing.

{
  "type": "typing_stop",
  "chat_id": 1
}

---

# Error Messages

Server may respond with error events.

Example:

{
  "type": "error",
  "message": "invalid session"
}

---

# Design Principles

The protocol is designed to support:

- realtime messaging
- chat rooms
- direct messages
- typing indicators
- scalable event routing