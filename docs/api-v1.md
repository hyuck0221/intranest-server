# IntraNest Server API v1

Base path: `/api/v1`  
Response format: JSON unless a file download is requested.  
All endpoints except `GET /health` require `Authorization: Bearer <access-key>`.

## Connection check

`GET /health` is intentionally public and returns `200` only when the server is ready to serve requests.

```json
{
  "service": "intranest-server",
  "status": "ok",
  "version": "0.1.0",
  "apiVersion": "1"
}
```

Clients should verify both `status == "ok"` and `apiVersion == "1"` before showing shared content. The health endpoint does not grant access to protected data.

## Access key

The server creates a random 256-bit key in `data/access.token` on first start. Send it in the `Authorization` header for all protected REST requests. Compare errors by HTTP status; do not infer authorization from the public health response.

## Links

- `GET /links` — list links in creation order.
- `POST /links` — create `{ "title": "Team handbook", "url": "https://intranet.example/handbook" }`.
- `PUT /links/{id}` — replace the title and URL.
- `DELETE /links/{id}` — remove a link.

Only HTTP and HTTPS URLs are accepted. URL credentials are rejected.

## Chat

- `GET /messages?limit=100` — return the most recent messages in chronological order. The limit is 1–200.
- `GET /messages?limit=100&before=<message-id>` — return the next older page, excluding the cursor message.
- `POST /messages` — create `{ "author": "Mina", "content": "Hello" }`. Content is plain text.
- `POST /chat/ticket` — request a single-use WebSocket ticket. Tickets expire after 30 seconds.
- `GET /chat/ws?ticket=<ticket>` — open a WebSocket after obtaining a ticket. New messages arrive as `{ "type": "message", "message": { ... } }`.

The default history retention is 30 days. The server periodically deletes expired messages and applies a changed retention value immediately.
The author field is a sender-entered display name; the shared access key does not verify individual identities.

## Files

- `GET /files` — list file metadata.
- `POST /files` — upload one multipart field named `file`.
- `GET /files/{id}/download` — download the file as an attachment.
- `DELETE /files/{id}` — permanently delete the file.

Uploads default to a maximum of 100 MiB per file and 10 GiB total. Stored files use generated IDs, and downloads always use `Content-Disposition: attachment`.

## Settings

- `GET /settings` — read `{ "chatRetentionDays": 30 }`.
- `PUT /settings` — set `{ "chatRetentionDays": 45 }`; accepted values are 1–3,650 days.

## Browser origins

Set `INTRANEST_CORS_ORIGINS` to a comma-separated allowlist of exact web origins, such as `https://intranest.example`. Chrome extension origins are accepted by scheme and Chrome's 32-character extension ID format. Credentials are never enabled for CORS; clients send the explicit bearer key.
