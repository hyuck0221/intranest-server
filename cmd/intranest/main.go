package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	bolt "go.etcd.io/bbolt"
)

var version = "0.1.0"

const (
	apiVersion       = "1"
	defaultRetention = 30
	defaultMaxUpload = int64(100 << 20)
	defaultMaxTotal  = int64(10 << 30)
)

type Link struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	URL       string    `json:"url"`
	CreatedAt time.Time `json:"createdAt"`
}

type ChatMessage struct {
	ID        string    `json:"id"`
	Author    string    `json:"author"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"createdAt"`
}

type SharedFile struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"createdAt"`
}

type Settings struct {
	ChatRetentionDays int `json:"chatRetentionDays"`
}

type Store struct {
	db       *bolt.DB
	filesDir string
}

var (
	bucketLinks    = []byte("links")
	bucketMessages = []byte("messages")
	bucketFiles    = []byte("files")
	bucketSettings = []byte("settings")
)

func openStore(dataDir string) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, err
	}
	if err := os.Chmod(dataDir, 0o700); err != nil {
		return nil, err
	}
	filesDir := filepath.Join(dataDir, "files")
	if err := os.MkdirAll(filesDir, 0o700); err != nil {
		return nil, err
	}
	if err := os.Chmod(filesDir, 0o700); err != nil {
		return nil, err
	}
	databasePath := filepath.Join(dataDir, "intranest.db")
	db, err := bolt.Open(databasePath, 0o600, &bolt.Options{Timeout: 2 * time.Second})
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(databasePath, 0o600); err != nil {
		db.Close()
		return nil, err
	}
	s := &Store{db: db, filesDir: filesDir}
	err = db.Update(func(tx *bolt.Tx) error {
		for _, name := range [][]byte{bucketLinks, bucketMessages, bucketFiles, bucketSettings} {
			if _, err := tx.CreateBucketIfNotExists(name); err != nil {
				return err
			}
		}
		b := tx.Bucket(bucketSettings)
		if b.Get([]byte("chatRetentionDays")) == nil {
			return b.Put([]byte("chatRetentionDays"), []byte(strconv.Itoa(defaultRetention)))
		}
		return nil
	})
	if err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) close() error { return s.db.Close() }

func randomID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func encode(v any) ([]byte, error) { return json.Marshal(v) }

func decode[T any](value []byte) (T, error) {
	var result T
	err := json.Unmarshal(value, &result)
	return result, err
}

func (s *Store) listLinks() ([]Link, error) {
	items := make([]Link, 0)
	err := s.db.View(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketLinks).ForEach(func(_, value []byte) error {
			item, err := decode[Link](value)
			if err == nil {
				items = append(items, item)
			}
			return err
		})
	})
	sort.Slice(items, func(i, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].Title < items[j].Title
		}
		return items[i].CreatedAt.Before(items[j].CreatedAt)
	})
	return items, err
}

func (s *Store) saveLink(item Link) error {
	value, err := encode(item)
	if err != nil {
		return err
	}
	return s.db.Update(func(tx *bolt.Tx) error { return tx.Bucket(bucketLinks).Put([]byte(item.ID), value) })
}

func (s *Store) getLink(id string) (Link, error) {
	var item Link
	err := s.db.View(func(tx *bolt.Tx) error {
		value := tx.Bucket(bucketLinks).Get([]byte(id))
		if value == nil {
			return bolt.ErrBucketNotFound
		}
		var err error
		item, err = decode[Link](value)
		return err
	})
	return item, err
}

func (s *Store) deleteLink(id string) error {
	return s.db.Update(func(tx *bolt.Tx) error { return tx.Bucket(bucketLinks).Delete([]byte(id)) })
}

func (s *Store) retentionDays() (int, error) {
	var days int
	err := s.db.View(func(tx *bolt.Tx) error {
		value := tx.Bucket(bucketSettings).Get([]byte("chatRetentionDays"))
		parsed, err := strconv.Atoi(string(value))
		if err != nil {
			return err
		}
		days = parsed
		return nil
	})
	return days, err
}

func (s *Store) setRetentionDays(days int) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketSettings).Put([]byte("chatRetentionDays"), []byte(strconv.Itoa(days)))
	})
}

func (s *Store) listMessages(limit int) ([]ChatMessage, error) {
	return s.listMessagesBefore("", limit)
}

func (s *Store) listMessagesBefore(beforeID string, limit int) ([]ChatMessage, error) {
	items := make([]ChatMessage, 0, limit)
	foundCursor := beforeID == ""
	err := s.db.View(func(tx *bolt.Tx) error {
		cursor := tx.Bucket(bucketMessages).Cursor()
		for key, value := cursor.Last(); key != nil && len(items) < limit; key, value = cursor.Prev() {
			item, err := decode[ChatMessage](value)
			if err != nil {
				return err
			}
			if !foundCursor {
				if item.ID == beforeID {
					foundCursor = true
				}
				continue
			}
			items = append(items, item)
		}
		return nil
	})
	for left, right := 0, len(items)-1; left < right; left, right = left+1, right-1 {
		items[left], items[right] = items[right], items[left]
	}
	return items, err
}

func (s *Store) saveMessage(item ChatMessage) error {
	value, err := encode(item)
	if err != nil {
		return err
	}
	key := fmt.Sprintf("%020d-%s", item.CreatedAt.UnixMicro(), item.ID)
	return s.db.Update(func(tx *bolt.Tx) error { return tx.Bucket(bucketMessages).Put([]byte(key), value) })
}

func (s *Store) pruneMessages(before time.Time) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		cursor := tx.Bucket(bucketMessages).Cursor()
		for key, value := cursor.First(); key != nil; key, value = cursor.Next() {
			message, err := decode[ChatMessage](value)
			if err != nil {
				return err
			}
			if message.CreatedAt.Before(before) {
				if err := cursor.Delete(); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (s *Store) listFiles() ([]SharedFile, error) {
	items := make([]SharedFile, 0)
	err := s.db.View(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketFiles).ForEach(func(_, value []byte) error {
			item, err := decode[SharedFile](value)
			if err == nil {
				items = append(items, item)
			}
			return err
		})
	})
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })
	return items, err
}

func (s *Store) saveFile(item SharedFile) error {
	value, err := encode(item)
	if err != nil {
		return err
	}
	return s.db.Update(func(tx *bolt.Tx) error { return tx.Bucket(bucketFiles).Put([]byte(item.ID), value) })
}

func (s *Store) getFile(id string) (SharedFile, error) {
	var item SharedFile
	err := s.db.View(func(tx *bolt.Tx) error {
		value := tx.Bucket(bucketFiles).Get([]byte(id))
		if value == nil {
			return bolt.ErrBucketNotFound
		}
		var err error
		item, err = decode[SharedFile](value)
		return err
	})
	return item, err
}

func (s *Store) deleteFileRecord(id string) error {
	return s.db.Update(func(tx *bolt.Tx) error { return tx.Bucket(bucketFiles).Delete([]byte(id)) })
}

type wsClient struct {
	conn *websocket.Conn
	send chan []byte
	hub  *chatHub
}

type chatHub struct {
	mu      sync.Mutex
	clients map[*wsClient]struct{}
}

func newChatHub() *chatHub { return &chatHub{clients: make(map[*wsClient]struct{})} }

func (h *chatHub) add(client *wsClient) {
	h.mu.Lock()
	h.clients[client] = struct{}{}
	h.mu.Unlock()
}

func (h *chatHub) remove(client *wsClient) {
	h.mu.Lock()
	if _, ok := h.clients[client]; ok {
		delete(h.clients, client)
		close(client.send)
	}
	h.mu.Unlock()
}

func (h *chatHub) publish(message ChatMessage) {
	payload, err := json.Marshal(map[string]any{"type": "message", "message": message})
	if err != nil {
		return
	}
	h.mu.Lock()
	for client := range h.clients {
		select {
		case client.send <- payload:
		default:
			delete(h.clients, client)
			close(client.send)
			go client.conn.Close()
		}
	}
	h.mu.Unlock()
}

func (client *wsClient) writePump() {
	ticker := time.NewTicker(25 * time.Second)
	defer func() {
		ticker.Stop()
		client.hub.remove(client)
		client.conn.Close()
	}()
	for {
		select {
		case message, ok := <-client.send:
			client.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				client.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := client.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			client.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := client.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (client *wsClient) readPump() {
	defer client.hub.remove(client)
	client.conn.SetReadLimit(1024)
	client.conn.SetReadDeadline(time.Now().Add(70 * time.Second))
	client.conn.SetPongHandler(func(string) error {
		client.conn.SetReadDeadline(time.Now().Add(70 * time.Second))
		return nil
	})
	for {
		if _, _, err := client.conn.ReadMessage(); err != nil {
			return
		}
	}
}

type API struct {
	store          *Store
	token          string
	maxUploadBytes int64
	maxTotalBytes  int64
	uploadMu       sync.Mutex
	allowedOrigins map[string]struct{}
	ticketsMu      sync.Mutex
	tickets        map[string]time.Time
	hub            *chatHub
}

func newAPI(store *Store, token string, maxUploadBytes, maxTotalBytes int64, origins []string, hub *chatHub) *API {
	allow := make(map[string]struct{}, len(origins))
	for _, origin := range origins {
		if trimmed := strings.TrimSpace(origin); trimmed != "" {
			allow[strings.TrimSuffix(trimmed, "/")] = struct{}{}
		}
	}
	return &API{
		store: store, token: token, maxUploadBytes: maxUploadBytes, maxTotalBytes: maxTotalBytes, allowedOrigins: allow,
		tickets: make(map[string]time.Time), hub: hub,
	}
}

func (a *API) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health", a.health)
	mux.Handle("GET /api/v1/session", a.protect(http.HandlerFunc(a.session)))
	mux.Handle("GET /api/v1/links", a.protect(http.HandlerFunc(a.listLinks)))
	mux.Handle("POST /api/v1/links", a.protect(http.HandlerFunc(a.createLink)))
	mux.Handle("PUT /api/v1/links/{id}", a.protect(http.HandlerFunc(a.updateLink)))
	mux.Handle("DELETE /api/v1/links/{id}", a.protect(http.HandlerFunc(a.removeLink)))
	mux.Handle("GET /api/v1/messages", a.protect(http.HandlerFunc(a.listMessages)))
	mux.Handle("POST /api/v1/messages", a.protect(http.HandlerFunc(a.createMessage)))
	mux.Handle("POST /api/v1/chat/ticket", a.protect(http.HandlerFunc(a.createChatTicket)))
	mux.HandleFunc("GET /api/v1/chat/ws", a.chatSocket)
	mux.Handle("GET /api/v1/files", a.protect(http.HandlerFunc(a.listFiles)))
	mux.Handle("POST /api/v1/files", a.protect(http.HandlerFunc(a.uploadFile)))
	mux.Handle("GET /api/v1/files/{id}/download", a.protect(http.HandlerFunc(a.downloadFile)))
	mux.Handle("DELETE /api/v1/files/{id}", a.protect(http.HandlerFunc(a.removeFile)))
	mux.Handle("GET /api/v1/settings", a.protect(http.HandlerFunc(a.getSettings)))
	mux.Handle("PUT /api/v1/settings", a.protect(http.HandlerFunc(a.updateSettings)))
	return a.securityHeaders(a.cors(mux))
}

func (a *API) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

var extensionOriginPattern = regexp.MustCompile(`^chrome-extension://[a-p]{32}$`)

func (a *API) originAllowed(origin string) bool {
	if origin == "" {
		return true
	}
	if extensionOriginPattern.MatchString(origin) {
		return true
	}
	_, ok := a.allowedOrigins[strings.TrimSuffix(origin, "/")]
	return ok
}

func (a *API) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			if !a.originAllowed(origin) {
				writeError(w, http.StatusForbidden, "origin_not_allowed", "This site is not allowed to access this server.")
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Add("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Max-Age", "600")
			if r.Method == http.MethodOptions && strings.EqualFold(r.Header.Get("Access-Control-Request-Private-Network"), "true") {
				w.Header().Set("Access-Control-Allow-Private-Network", "true")
			}
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *API) protect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			writeError(w, http.StatusUnauthorized, "access_key_required", "Enter this server's access key to continue.")
			return
		}
		provided := strings.TrimPrefix(header, "Bearer ")
		if len(provided) != len(a.token) || subtle.ConstantTimeCompare([]byte(provided), []byte(a.token)) != 1 {
			writeError(w, http.StatusUnauthorized, "invalid_access_key", "The access key is not valid.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *API) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"service": "intranest-server", "status": "ok", "version": version, "apiVersion": apiVersion,
	})
}

func (a *API) session(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "authorized"})
}

func (a *API) listLinks(w http.ResponseWriter, _ *http.Request) {
	items, err := a.store.listLinks()
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

type linkInput struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

func normalizeLinkInput(input linkInput) (linkInput, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.URL = strings.TrimSpace(input.URL)
	if input.Title == "" || len([]rune(input.Title)) > 100 {
		return input, errors.New("title must contain between 1 and 100 characters")
	}
	parsed, err := url.ParseRequestURI(input.URL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil {
		return input, errors.New("link must be a valid http or https address without credentials")
	}
	return input, nil
}

func (a *API) createLink(w http.ResponseWriter, r *http.Request) {
	var input linkInput
	if !readJSON(w, r, &input, 16<<10) {
		return
	}
	input, err := normalizeLinkInput(input)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_link", err.Error())
		return
	}
	id, err := randomID()
	if err != nil {
		writeInternal(w, err)
		return
	}
	item := Link{ID: id, Title: input.Title, URL: input.URL, CreatedAt: time.Now().UTC()}
	if err := a.store.saveLink(item); err != nil {
		writeInternal(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (a *API) updateLink(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	item, err := a.store.getLink(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "link_not_found", "That link no longer exists.")
		return
	}
	var input linkInput
	if !readJSON(w, r, &input, 16<<10) {
		return
	}
	input, err = normalizeLinkInput(input)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_link", err.Error())
		return
	}
	item.Title, item.URL = input.Title, input.URL
	if err := a.store.saveLink(item); err != nil {
		writeInternal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (a *API) removeLink(w http.ResponseWriter, r *http.Request) {
	if err := a.store.deleteLink(r.PathValue("id")); err != nil {
		writeInternal(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) listMessages(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			writeError(w, http.StatusBadRequest, "invalid_limit", "Limit must be between 1 and 200.")
			return
		}
		limit = min(parsed, 200)
	}
	before := r.URL.Query().Get("before")
	if before != "" && !validID(before) {
		writeError(w, http.StatusBadRequest, "invalid_cursor", "The message cursor is invalid.")
		return
	}
	items, err := a.store.listMessagesBefore(before, limit)
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

type messageInput struct {
	Author  string `json:"author"`
	Content string `json:"content"`
}

func (a *API) createMessage(w http.ResponseWriter, r *http.Request) {
	var input messageInput
	if !readJSON(w, r, &input, 16<<10) {
		return
	}
	input.Author = strings.TrimSpace(input.Author)
	input.Content = strings.TrimSpace(input.Content)
	if input.Author == "" || len([]rune(input.Author)) > 40 || input.Content == "" || len([]rune(input.Content)) > 4000 {
		writeError(w, http.StatusBadRequest, "invalid_message", "A message needs a name and 1–4,000 characters of text.")
		return
	}
	id, err := randomID()
	if err != nil {
		writeInternal(w, err)
		return
	}
	message := ChatMessage{ID: id, Author: input.Author, Content: input.Content, CreatedAt: time.Now().UTC()}
	if err := a.store.saveMessage(message); err != nil {
		writeInternal(w, err)
		return
	}
	a.hub.publish(message)
	writeJSON(w, http.StatusCreated, message)
}

func (a *API) createChatTicket(w http.ResponseWriter, _ *http.Request) {
	ticket, err := randomID()
	if err != nil {
		writeInternal(w, err)
		return
	}
	a.ticketsMu.Lock()
	now := time.Now()
	for key, expiry := range a.tickets {
		if now.After(expiry) {
			delete(a.tickets, key)
		}
	}
	a.tickets[ticket] = now.Add(30 * time.Second)
	a.ticketsMu.Unlock()
	writeJSON(w, http.StatusCreated, map[string]any{"ticket": ticket, "expiresInSeconds": 30})
}

func (a *API) consumeTicket(ticket string) bool {
	a.ticketsMu.Lock()
	defer a.ticketsMu.Unlock()
	expires, ok := a.tickets[ticket]
	delete(a.tickets, ticket)
	return ok && time.Now().Before(expires)
}

func (a *API) chatSocket(w http.ResponseWriter, r *http.Request) {
	if !a.originAllowed(r.Header.Get("Origin")) {
		http.Error(w, "origin not allowed", http.StatusForbidden)
		return
	}
	if !a.consumeTicket(r.URL.Query().Get("ticket")) {
		http.Error(w, "valid one-time chat ticket required", http.StatusUnauthorized)
		return
	}
	upgrader := websocket.Upgrader{CheckOrigin: func(req *http.Request) bool { return a.originAllowed(req.Header.Get("Origin")) }}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	client := &wsClient{conn: conn, send: make(chan []byte, 64), hub: a.hub}
	a.hub.add(client)
	go client.writePump()
	client.readPump()
}

func (a *API) listFiles(w http.ResponseWriter, _ *http.Request) {
	items, err := a.store.listFiles()
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func safeFilename(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	name = filepath.Base(name)
	name = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, name)
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == "/" {
		return "shared-file"
	}
	if len([]rune(name)) > 180 {
		name = string([]rune(name)[:180])
	}
	return name
}

func (a *API) uploadFile(w http.ResponseWriter, r *http.Request) {
	a.uploadMu.Lock()
	defer a.uploadMu.Unlock()

	r.Body = http.MaxBytesReader(w, r.Body, a.maxUploadBytes+(1<<20))
	reader, err := r.MultipartReader()
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_upload", "Choose one file to upload.")
		return
	}
	var fileName string
	var tempName string
	var written int64
	removeTemp := func() {
		if tempName != "" {
			_ = os.Remove(tempName)
		}
	}
	for {
		part, nextErr := reader.NextPart()
		if nextErr == io.EOF {
			break
		}
		if nextErr != nil {
			removeTemp()
			var maxBytesError *http.MaxBytesError
			if errors.As(nextErr, &maxBytesError) {
				writeError(w, http.StatusRequestEntityTooLarge, "file_too_large", "This server's upload limit was exceeded.")
			} else {
				writeError(w, http.StatusBadRequest, "invalid_upload", "The upload could not be read.")
			}
			return
		}
		if part.FormName() == "file" && part.FileName() != "" {
			if fileName != "" {
				part.Close()
				removeTemp()
				writeError(w, http.StatusBadRequest, "invalid_upload", "Upload one file at a time.")
				return
			}
			fileName = part.FileName()
			temp, createErr := os.CreateTemp(a.store.filesDir, ".upload-*")
			if createErr != nil {
				part.Close()
				writeInternal(w, createErr)
				return
			}
			tempName = temp.Name()
			written, err = io.Copy(temp, io.LimitReader(part, a.maxUploadBytes+1))
			closeErr := temp.Close()
			part.Close()
			if err != nil || closeErr != nil {
				removeTemp()
				writeError(w, http.StatusBadRequest, "upload_failed", "The upload could not be saved.")
				return
			}
			if written > a.maxUploadBytes {
				removeTemp()
				writeError(w, http.StatusRequestEntityTooLarge, "file_too_large", "This server's per-file upload limit was exceeded.")
				return
			}
		} else {
			part.Close()
		}
	}
	if fileName == "" {
		writeError(w, http.StatusBadRequest, "invalid_upload", "Choose one file to upload.")
		return
	}
	defer removeTemp()
	files, err := a.store.listFiles()
	if err != nil {
		writeInternal(w, err)
		return
	}
	var totalBytes int64
	for _, item := range files {
		totalBytes += item.Size
	}
	if written > a.maxTotalBytes-totalBytes {
		writeError(w, http.StatusInsufficientStorage, "storage_limit_reached", "This server's shared file storage limit has been reached.")
		return
	}
	id, err := randomID()
	if err != nil {
		writeInternal(w, err)
		return
	}
	item := SharedFile{ID: id, Name: safeFilename(fileName), Size: written, CreatedAt: time.Now().UTC()}
	target := filepath.Join(a.store.filesDir, id)
	if err := os.Rename(tempName, target); err != nil {
		writeInternal(w, err)
		return
	}
	tempName = ""
	if err := a.store.saveFile(item); err != nil {
		os.Remove(target)
		writeInternal(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func validID(id string) bool {
	decoded, err := hex.DecodeString(id)
	return err == nil && len(decoded) == 16
}

func (a *API) downloadFile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !validID(id) {
		writeError(w, http.StatusNotFound, "file_not_found", "That file does not exist.")
		return
	}
	item, err := a.store.getFile(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "file_not_found", "That file does not exist.")
		return
	}
	file, err := os.Open(filepath.Join(a.store.filesDir, id))
	if err != nil {
		writeError(w, http.StatusNotFound, "file_not_found", "That file is no longer available.")
		return
	}
	defer file.Close()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": item.Name}))
	w.Header().Set("Content-Length", strconv.FormatInt(item.Size, 10))
	io.Copy(w, file)
}

func (a *API) removeFile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !validID(id) {
		writeError(w, http.StatusNotFound, "file_not_found", "That file does not exist.")
		return
	}
	if _, err := a.store.getFile(id); err != nil {
		writeError(w, http.StatusNotFound, "file_not_found", "That file does not exist.")
		return
	}
	if err := os.Remove(filepath.Join(a.store.filesDir, id)); err != nil && !errors.Is(err, os.ErrNotExist) {
		writeInternal(w, err)
		return
	}
	if err := a.store.deleteFileRecord(id); err != nil {
		writeInternal(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) getSettings(w http.ResponseWriter, _ *http.Request) {
	days, err := a.store.retentionDays()
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, Settings{ChatRetentionDays: days})
}

func (a *API) updateSettings(w http.ResponseWriter, r *http.Request) {
	var settings Settings
	if !readJSON(w, r, &settings, 4<<10) {
		return
	}
	if settings.ChatRetentionDays < 1 || settings.ChatRetentionDays > 3650 {
		writeError(w, http.StatusBadRequest, "invalid_retention", "Chat retention must be between 1 and 3,650 days.")
		return
	}
	if err := a.store.setRetentionDays(settings.ChatRetentionDays); err != nil {
		writeInternal(w, err)
		return
	}
	a.pruneChat()
	writeJSON(w, http.StatusOK, settings)
}

func (a *API) pruneChat() {
	days, err := a.store.retentionDays()
	if err != nil {
		slog.Error("could not read chat retention setting", "error", err)
		return
	}
	if err := a.store.pruneMessages(time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour)); err != nil {
		slog.Error("could not prune expired chat messages", "error", err)
	}
}

func readJSON(w http.ResponseWriter, r *http.Request, target any, maxBytes int64) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "The request body is invalid.")
		return false
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeError(w, http.StatusBadRequest, "invalid_json", "Send one JSON object per request.")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		slog.Error("could not write response", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"error": code, "message": message})
}

func writeInternal(w http.ResponseWriter, err error) {
	slog.Error("request failed", "error", err)
	writeError(w, http.StatusInternalServerError, "internal_error", "The server could not complete this request.")
}

func makeAccessToken(dataDir string) (string, error) {
	if token := strings.TrimSpace(os.Getenv("INTRANEST_ACCESS_TOKEN")); token != "" {
		if len(token) < 32 {
			return "", errors.New("INTRANEST_ACCESS_TOKEN must contain at least 32 characters")
		}
		return token, nil
	}
	path := filepath.Join(dataDir, "access.token")
	data, err := os.ReadFile(path)
	if err == nil {
		token := strings.TrimSpace(string(data))
		if len(token) >= 32 {
			if err := os.Chmod(path, 0o600); err != nil {
				return "", err
			}
			return token, nil
		}
		return "", errors.New("access.token must contain at least 32 characters")
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	token := hex.EncodeToString(bytes)
	if err := os.WriteFile(path, []byte(token+"\n"), 0o600); err != nil {
		return "", err
	}
	return token, nil
}

func parseOrigins(raw string) []string {
	items := strings.Split(raw, ",")
	origins := make([]string, 0, len(items))
	for _, item := range items {
		if origin := strings.TrimSpace(item); origin != "" {
			origins = append(origins, origin)
		}
	}
	return origins
}

func maxUploadFromEnv() int64 {
	raw := strings.TrimSpace(os.Getenv("INTRANEST_MAX_FILE_BYTES"))
	if raw == "" {
		return defaultMaxUpload
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value < 1<<20 || value > 2<<30 {
		slog.Warn("invalid INTRANEST_MAX_FILE_BYTES; using 100 MiB default")
		return defaultMaxUpload
	}
	return value
}

func maxTotalFromEnv() int64 {
	raw := strings.TrimSpace(os.Getenv("INTRANEST_MAX_TOTAL_BYTES"))
	if raw == "" {
		return defaultMaxTotal
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value < 1<<20 {
		slog.Warn("invalid INTRANEST_MAX_TOTAL_BYTES; using 10 GiB default")
		return defaultMaxTotal
	}
	return value
}

func run() error {
	dataDir := strings.TrimSpace(os.Getenv("INTRANEST_DATA_DIR"))
	if dataDir == "" {
		dataDir = "./data"
	}
	store, err := openStore(dataDir)
	if err != nil {
		return fmt.Errorf("open data store: %w", err)
	}
	defer store.close()
	token, err := makeAccessToken(dataDir)
	if err != nil {
		return fmt.Errorf("load access key: %w", err)
	}
	hub := newChatHub()
	api := newAPI(store, token, maxUploadFromEnv(), maxTotalFromEnv(), parseOrigins(os.Getenv("INTRANEST_CORS_ORIGINS")), hub)
	api.pruneChat()
	addr := strings.TrimSpace(os.Getenv("INTRANEST_LISTEN_ADDR"))
	if addr == "" {
		addr = "0.0.0.0:8080"
	}
	server := &http.Server{
		Addr: addr, Handler: api.handler(), ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 10 * time.Minute, WriteTimeout: 0, IdleTimeout: 90 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			api.pruneChat()
		}
	}()
	go func() {
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
		<-signals
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			slog.Error("server shutdown failed", "error", err)
		}
	}()
	if _, _, err := net.SplitHostPort(addr); err != nil {
		return fmt.Errorf("invalid listen address: %w", err)
	}
	if cert, key := os.Getenv("INTRANEST_TLS_CERT"), os.Getenv("INTRANEST_TLS_KEY"); (cert == "") != (key == "") {
		return errors.New("set both INTRANEST_TLS_CERT and INTRANEST_TLS_KEY to enable HTTPS")
	}
	slog.Info("IntraNest server starting", "version", version, "listen", addr, "data", dataDir,
		"accessKeyFile", filepath.Join(dataDir, "access.token"), "https", os.Getenv("INTRANEST_TLS_CERT") != "")
	if os.Getenv("INTRANEST_TLS_CERT") != "" {
		err = server.ListenAndServeTLS(os.Getenv("INTRANEST_TLS_CERT"), os.Getenv("INTRANEST_TLS_KEY"))
	} else {
		slog.Warn("HTTPS is disabled; use trusted LAN networking or configure a TLS certificate before sending sensitive data")
		err = server.ListenAndServe()
	}
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func main() {
	if err := run(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
