# Customer Chat API Documentation

Base URL: `http://localhost:8080`

---

## Authentication

All customer API endpoints (except `/health` and `/api/health`) require **HMAC-SHA256** authentication.

### Required Headers

| Header | Description |
|--------|-------------|
| `current_timestamp` | Unix timestamp in seconds |
| `current_nonce` | Random unique string per request |
| `current_signature` | HMAC-SHA256 hex signature |

### Signature Payload Format

```
{HTTP_METHOD}|{REQUEST_PATH}|{timestamp}|{nonce}
```

**Example for POST /api/customer/messages:**
```
POST|/api/customer/messages|1690000000|abc123def456
```

### Signature Generation

```javascript
var secret = pm.collectionVariables.get("hmac_secret");
var method = pm.request.method.toUpperCase();
var uri = pm.request.url.getPath();
var timestamp = Math.floor(Date.now() / 1000).toString();
var nonce = Math.random().toString(36).substring(2) + Date.now().toString(36);
var payload = method + "|" + uri + "|" + timestamp + "|" + nonce;
var signature = CryptoJS.HmacSHA256(payload, secret).toString(CryptoJS.enc.Hex);

pm.request.headers.upsert({ key: 'current_timestamp', value: timestamp });
pm.request.headers.upsert({ key: 'current_nonce', value: nonce });
pm.request.headers.upsert({ key: 'current_signature', value: signature });
```

---

## WebSocket Connection

Real-time messaging uses WebSocket. Connect after obtaining HMAC credentials.

**Endpoint:** `ws://localhost:8080/ws`

**Query Parameters:**
- `user_id` (required): Customer ID
- `user_type` (required): `CUSTOMER` or `ADMIN`
- `timestamp` (required): Unix timestamp
- `nonce` (required): Random unique string
- `signature` (required): HMAC-SHA256 signature

**Signature Payload:**
```
GET|/ws|{timestamp}|{nonce}
```

**WebSocket Message Types Received:**
- `NEW_MESSAGE` - New message received
- `READ_STATUS` - Message read status updated
- `DELETE_MESSAGE` - Message deleted

---

## HTTP Endpoints

### 1. Send Customer Message

Send a new message with optional media attachments.

**Endpoint:** `POST /api/customer/messages`

**Content-Type:** `application/x-www-form-urlencoded` or `multipart/form-data`

**Form Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `user_id` | string | Yes | Sender user ID |
| `user_type` | string | No | Default: `CUSTOMER` |
| `user_phone` | string | Yes* | Customer phone number (*required for CUSTOMER type) |
| `content` | string | No | Message text |
| `full_name` | string | No | Sender full name |
| `profile_picture` | string | No | Profile picture URL |
| `gender` | string | No | Gender |
| `voice_messages` | file | No | Audio file (mp3, wav, aac, ogg, m4a, webm) |
| `photo` | file | No | Image file (jpg, jpeg, png, gif, webp, bmp, svg) |
| `file` | file | No | Document file (pdf, doc, docx, txt, zip, rar, 7z, xls, xlsx, ppt, pptx, csv, json, xml) |

**Note:** At least one of `content`, `voice_messages`, `photo`, or `file` is required.

**Admin Only Fields:**
- `target_user_id` (required when `user_type=ADMIN`)

**Response 201 Created:**
```json
{
    "success": true,
    "message": "message sent",
    "data": {
        "Type": "NEW_MESSAGE",
        "ID": "msg_123",
        "UserID": "30",
        "AdminID": null,
        "SendedBy": "CUSTOMER",
        "SenderName": "Customer",
        "Content": "Hello!",
        "Seen": false,
        "VoiceMessages": null,
        "Photo": null,
        "File": null,
        "UserPhone": "+8801712345678",
        "FullName": null,
        "ProfilePicture": null,
        "Gender": null,
        "CreatedAt": "2024-01-01T00:00:00Z"
    }
}
```

---

### 2. Get Customer Message History

Retrieve paginated message history for a customer.

**Endpoint:** `GET /api/customer/messages`

**Query Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `user_id` | string | Yes | Customer user ID |
| `user_type` | string | No | Default: `CUSTOMER`. Use `ADMIN` for admin view |
| `target_user_id` | string | Yes* | Required when `user_type=ADMIN` |
| `cursor` | string | No | RFC3339 timestamp for pagination (e.g., `2024-01-01T00:00:00Z`) |
| `limit` | integer | No | Messages per page (1-100, default: 50) |

**Response 200 OK:**
```json
{
    "messages": [
        {
            "ID": "msg_123",
            "UserID": "30",
            "AdminID": null,
            "SendedBy": "CUSTOMER",
            "SenderName": "Customer",
            "Content": "Hello!",
            "Seen": false,
            "VoiceMessages": null,
            "Photo": null,
            "File": null,
            "UserPhone": "+8801712345678",
            "FullName": null,
            "ProfilePicture": null,
            "Gender": null,
            "CreatedAt": "2024-01-01T00:00:00Z"
        }
    ],
    "next_cursor": "2024-01-01T00:00:00Z",
    "has_more": true
}
```

**Notes:**
- When `user_type=CUSTOMER`, the system automatically sets `target_user_id = user_id`
- When `user_type=ADMIN`, `target_user_id` is required to specify which customer's history to view
- Admin responses hide `AdminID` field and set `SenderName` to "Support Admin"

---

### 3. Mark Messages as Seen

Mark all messages as seen for a specific user.

**Endpoint:** `POST /api/customer/messages/seen`

**Content-Type:** `application/x-www-form-urlencoded`

**Form Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `user_id` | string | Yes | User ID whose messages to mark as seen |
| `user_type` | string | No | Default: `CUSTOMER` |

**Admin Only Fields:**
- `target_user_id` (required when `user_type=ADMIN`)

**Response 200 OK:**
```json
{
    "success": true,
    "message": "messages marked as seen",
    "data": null
}
```

---

### 4. Edit Customer Message

Edit an existing message. **Admin only.**

**Endpoint:** `PATCH /api/customer/messages/{id}`

**URL Parameter:**
- `id` (required): Message ID

**Content-Type:** `application/x-www-form-urlencoded`

**Form Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `user_type` | string | Yes | Must be `ADMIN` |
| `content` | string | Yes | Updated message content |

**Response 200 OK:**
```json
{
    "success": true,
    "message": "message edited successfully",
    "data": {
        "ID": "msg_123",
        "UserID": "30",
        "AdminID": null,
        "SendedBy": "CUSTOMER",
        "SenderName": "Customer",
        "Content": "Updated message content",
        "Seen": false,
        "VoiceMessages": null,
        "Photo": null,
        "File": null,
        "UserPhone": "+8801712345678",
        "FullName": null,
        "ProfilePicture": null,
        "Gender": null,
        "CreatedAt": "2024-01-01T00:00:00Z"
    }
}
```

---

## Error Responses

All errors follow this format:

```json
{
    "success": false,
    "message": "Error description",
    "data": null
}
```

### Common HTTP Status Codes

| Status | Meaning |
|--------|---------|
| 200 | Success |
| 201 | Created |
| 400 | Bad Request - Missing or invalid parameters |
| 401 | Unauthorized - Missing or invalid HMAC headers |
| 403 | Forbidden - Insufficient permissions |
| 404 | Not Found |
| 500 | Internal Server Error |

### Common Error Messages

- `missing HMAC authentication headers (current_timestamp, current_nonce, current_signature)`
- `invalid HMAC signature`
- `nonce has already been used (replay attack detected)`
- `request has expired (timestamp too old or too far in the future)`
- `user_id form field is required`
- `user_phone form field is required`
- `at least one of content, voice_messages, photo, or file is required`
- `only admins can edit messages`
- `target_user_id required for admin`
- `invalid cursor format (must be RFC3339)`
- `limit must be a positive integer`

---

## File Upload Limits

- **Max file size:** 10 MB per file
- **Allowed audio:** mp3, wav, aac, ogg, m4a, webm
- **Allowed images:** jpg, jpeg, png, gif, webp, bmp, svg
- **Allowed documents:** pdf, doc, docx, txt, zip, rar, 7z, xls, xlsx, ppt, pptx, csv, json, xml

---

## WebSocket Real-time Events

The server broadcasts the following events to connected WebSocket clients:

| Event Type | Description |
|------------|-------------|
| `NEW_MESSAGE` | A new message was sent |
| `READ_STATUS` | Messages were marked as seen |
| `DELETE_MESSAGE` | A message was deleted |

**Example WebSocket Message:**
```json
{
    "Type": "NEW_MESSAGE",
    "ID": "msg_123",
    "UserID": "30",
    "AdminID": null,
    "SendedBy": "CUSTOMER",
    "SenderName": "Customer",
    "Content": "Hello!",
    "Seen": false,
    "VoiceMessages": null,
    "Photo": null,
    "File": null,
    "UserPhone": "+8801712345678",
    "FullName": null,
    "ProfilePicture": null,
    "Gender": null,
    "CreatedAt": "2024-01-01T00:00:00Z"
}
```

---

## Notes

1. **Health Check:** `/health` and `/api/health` are public endpoints and do not require HMAC authentication.
2. **SSE Listener:** When a customer sends a message, the server automatically starts an SSE listener for that customer to receive vendor API responses.
3. **Vendor Integration:** Messages are forwarded to an external vendor API asynchronously. This does not affect the HTTP response time.
4. **Admin Edit/Delete:** Only users with `user_type=ADMIN` can edit or delete messages.
5. **Replay Protection:** Each nonce can only be used once within a 5-minute window.
