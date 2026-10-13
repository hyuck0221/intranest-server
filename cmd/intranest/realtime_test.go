package main

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// Exercise persistence and broadcasts together: both legacy chat subscribers
// and the general event endpoint must observe committed resource changes.
func TestRealtimeResourceChanges(t *testing.T) {
	store, err := openStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.close()
	api := newAPI(store, 1024, 4096, newChatHub())
	server := httptest.NewServer(api.handler())
	defer server.Close()
	clients := []*websocket.Conn{}
	for _, path := range []string{"/api/v2/chat/ws", "/api/v2/events/ws"} {
		conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		clients = append(clients, conn)
	}
	request := func(method, path string, body io.Reader, contentType string, status int) []byte {
		t.Helper()
		req, err := http.NewRequest(method, server.URL+"/api/v2"+path, body)
		if err != nil {
			t.Fatal(err)
		}
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		data, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != status {
			t.Fatalf("%s %s: got %d %s, want %d", method, path, res.StatusCode, data, status)
		}
		return data
	}
	change := func(resource, action, id string) {
		t.Helper()
		for _, conn := range clients {
			conn.SetReadDeadline(time.Now().Add(3 * time.Second))
			var event struct{ Type, Resource, Action, ID string }
			if err := conn.ReadJSON(&event); err != nil {
				t.Fatal(err)
			}
			if event.Type != "change" || event.Resource != resource || event.Action != action || event.ID != id {
				t.Fatalf("unexpected event: %+v", event)
			}
		}
	}
	jsonRequest := func(method, path, body string, status int) []byte {
		return request(method, path, strings.NewReader(body), "application/json", status)
	}
	// Failed validation must not publish an event before the next valid change.
	jsonRequest("POST", "/links", `{"title":"","url":"javascript:alert(1)"}`, 400)
	var link Link
	if err := json.Unmarshal(jsonRequest("POST", "/links", `{"title":"Handbook","url":"https://handbook.example"}`, 201), &link); err != nil {
		t.Fatal(err)
	}
	change("links", "created", link.ID)
	if persisted, err := store.getLink(link.ID); err != nil || persisted.Title != "Handbook" {
		t.Fatalf("event preceded persistence: %+v %v", persisted, err)
	}
	jsonRequest("PUT", "/links/"+link.ID, `{"title":"New handbook","url":"https://handbook.example/new"}`, 200)
	change("links", "updated", link.ID)
	request("DELETE", "/links/"+link.ID, nil, "", 204)
	change("links", "deleted", link.ID)
	var message ChatMessage
	if err := json.Unmarshal(jsonRequest("POST", "/messages", `{"author":"Team","content":"Live hello"}`, 201), &message); err != nil {
		t.Fatal(err)
	}
	for _, conn := range clients {
		var event struct {
			Type    string
			Message ChatMessage
		}
		conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		if err := conn.ReadJSON(&event); err != nil {
			t.Fatal(err)
		}
		if event.Type != "message" || event.Message.ID != message.ID {
			t.Fatalf("unexpected chat event: %+v", event)
		}
	}
	upload := func(data string, status int) []byte {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		part, err := writer.CreateFormFile("file", "hello.txt")
		if err != nil {
			t.Fatal(err)
		}
		io.WriteString(part, data)
		writer.Close()
		return request("POST", "/files", &body, writer.FormDataContentType(), status)
	}
	upload(strings.Repeat("x", 1025), 413)
	var file SharedFile
	if err := json.Unmarshal(upload("Shared document", 201), &file); err != nil {
		t.Fatal(err)
	}
	change("files", "created", file.ID)
	if data := request("GET", "/files/"+file.ID+"/download", nil, "", 200); string(data) != "Shared document" {
		t.Fatal("download did not match upload")
	}
	request("DELETE", "/files/"+file.ID, nil, "", 204)
	change("files", "deleted", file.ID)
	request("DELETE", "/files/"+file.ID, nil, "", 404)
	expired := ChatMessage{ID: "expired", Author: "Team", Content: "Old", CreatedAt: time.Now().Add(-48 * time.Hour)}
	if err := store.saveMessage(expired); err != nil {
		t.Fatal(err)
	}
	jsonRequest("PUT", "/settings", `{"chatRetentionDays":1}`, 200)
	change("messages", "pruned", "")
	change("settings", "updated", "")
	messages, err := store.listMessages(100)
	if err != nil || len(messages) != 1 || messages[0].ID != message.ID {
		t.Fatalf("retention mismatch: %+v %v", messages, err)
	}
	// No pruning event when there was no expired content.
	jsonRequest("PUT", "/settings", `{"chatRetentionDays":2}`, 200)
	change("settings", "updated", "")
}
