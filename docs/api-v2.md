# IntraNest Server API v2

Base path: `/api/v2`  
Response format: JSON unless a file download is requested.  
No endpoint requires an access key or sign-in.

## Connection check

`GET /health` returns `200` when the server is ready.

```json
{
  "service": "intranest-server",
  "status": "ok",
  "version": "0.2.0",
  "apiVersion": "2"
}
```

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
- `PUT /messages/{id}` — update `{ "content": "Edited message" }`; the original author and creation time are preserved. Anyone with server access can edit any message. Empty content or more than 4,000 characters returns 400; a missing message returns 404.
- `DELETE /messages/{id}` — delete any message (204); a missing message returns 404.
- `GET /chat/ws` — open a WebSocket. New messages arrive as `{ "type": "message", "message": { ... } }`.

The default history retention is 30 days. The server periodically deletes expired messages and applies a changed retention value immediately. The author field is entered by the sender; it is not a verified account.

## Files

- `GET /files` — list file metadata.
- `POST /files` — upload one multipart field named `file`.
- `GET /files/{id}/download` — download the file as an attachment.
- `DELETE /files/{id}` — permanently delete the file.

Uploads default to a maximum of 100 MiB per file and 10 GiB total. Stored files use generated IDs, and downloads always use `Content-Disposition: attachment`.

## Settings

- `GET /settings` — read `{ "chatRetentionDays": 30 }`.
- `PUT /settings` — set `{ "chatRetentionDays": 45 }`; accepted values are 1–3,650 days.

## Realtime changes

`GET /events/ws` and the backward-compatible `GET /chat/ws` share one broadcast stream. Keep it open for every view, including links and files.

- New chat messages: `{ "type": "message", "message": { ... } }`.
- Link/file changes: `{ "type": "change", "resource": "links", "action": "created", "id": "..." }`. Resources are `links` and `files`; actions are `created`, `updated` (links only), and `deleted`.
- Retention changes: `{ "type": "change", "resource": "settings", "action": "updated", "id": "" }`.
- Expired chat history removed: `{ "type": "change", "resource": "messages", "action": "pruned", "id": "" }`.

Events are published after persistence succeeds. Failed validation and failed uploads do not publish resource changes. Change events invalidate the corresponding HTTP snapshot rather than carrying a full list. There is no event replay: fetch links/files/messages/settings on each connection and reconnect, queue another snapshot if an invalidation arrives while fetching, and merge messages received during a chat snapshot by ID. Clients reconnect with backoff and retry failed snapshots automatically. The interface does not require a refresh button.

## Browser access

The server listens on all interfaces by default and allows cross-origin browser requests with `Access-Control-Allow-Origin: *`. No cookies or credentials are used. Anyone who can reach the server can read, add, and delete its content; use a trusted network and do not expose the port to the public internet.

Message edits broadcast `{ "type": "change", "resource": "messages", "action": "updated", "id": "...", "message": { ... } }`. Deletes broadcast the same change envelope with `action: "deleted"` and no message. Both chat and general-event WebSocket subscribers receive changes after persistence.
