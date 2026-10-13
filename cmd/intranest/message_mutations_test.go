package main

import (
	"encoding/json"
	"github.com/gorilla/websocket"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestMessageEditsAndDeletesBroadcastCommittedChanges(t *testing.T) {
	store, err := openStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.close()
	server := httptest.NewServer(newAPI(store, 1024, 4096, newChatHub()).handler())
	defer server.Close()
	var clients []*websocket.Conn
	for _, path := range []string{"/chat/ws", "/events/ws"} {
		conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/api/v2"+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		clients = append(clients, conn)
	}
	request := func(method, path, body string, status int) []byte {
		t.Helper()
		req, _ := http.NewRequest(method, server.URL+"/api/v2"+path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		data, _ := io.ReadAll(res.Body)
		if res.StatusCode != status {
			t.Fatalf("%s %s: %d %s", method, path, res.StatusCode, data)
		}
		return data
	}
	var original ChatMessage
	if err := json.Unmarshal(request("POST", "/messages", `{"author":"Someone else","content":"Original"}`, 201), &original); err != nil {
		t.Fatal(err)
	}
	for _, conn := range clients {
		conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		if _, _, err := conn.ReadMessage(); err != nil {
			t.Fatal(err)
		}
	}
	request("PUT", "/messages/"+original.ID, `{"content":"  "}`, 400)
	request("PUT", "/messages/"+original.ID, `{"content":"`+strings.Repeat("한", 4001)+`"}`, 400)
	request("PUT", "/messages/missing", `{"content":"Edited"}`, 404)
	var edited ChatMessage
	if err := json.Unmarshal(request("PUT", "/messages/"+original.ID, `{"content":"  Edited by anyone  "}`, 200), &edited); err != nil {
		t.Fatal(err)
	}
	if edited.Content != "Edited by anyone" || edited.Author != original.Author || !edited.CreatedAt.Equal(original.CreatedAt) {
		t.Fatalf("metadata changed: %+v", edited)
	}
	for _, conn := range clients {
		var event struct {
			Type, Resource, Action, ID string
			Message                    ChatMessage
		}
		conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		if err := conn.ReadJSON(&event); err != nil {
			t.Fatal(err)
		}
		if event.Type != "change" || event.Action != "updated" || event.Resource != "messages" || event.ID != original.ID || event.Message.Content != edited.Content {
			t.Fatalf("bad edit event: %+v", event)
		}
	}
	persisted, err := store.listMessages(100)
	if err != nil || len(persisted) != 1 || persisted[0].Content != edited.Content {
		t.Fatalf("edit not persisted: %+v %v", persisted, err)
	}
	request("DELETE", "/messages/"+original.ID, "", 204)
	for _, conn := range clients {
		var event struct{ Type, Resource, Action, ID string }
		conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		if err := conn.ReadJSON(&event); err != nil {
			t.Fatal(err)
		}
		if event.Action != "deleted" || event.ID != original.ID || event.Resource != "messages" {
			t.Fatalf("bad delete: %+v", event)
		}
	}
	request("DELETE", "/messages/"+original.ID, "", 404)
	request("PUT", "/messages/"+original.ID, `{"content":"Cannot resurrect"}`, 404)
	remaining, err := store.listMessages(100)
	if err != nil || len(remaining) != 0 {
		t.Fatalf("delete not persisted: %+v %v", remaining, err)
	}
}
