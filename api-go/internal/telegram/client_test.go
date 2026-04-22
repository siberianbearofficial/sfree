package telegram

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/example/sfree/api-go/internal/sourcecap"
)

func TestClientUploadDownloadDelete(t *testing.T) {
	t.Parallel()
	const (
		token   = "token123"
		chatID  = "-100123"
		fileID  = "file-xyz"
		fileRef = "documents/file.bin"
		payload = "hello-telegram"
	)
	mux := http.NewServeMux()
	mux.HandleFunc("/bot"+token+"/sendDocument", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1024 * 1024); err != nil {
			t.Fatalf("parse multipart: %v", err)
		}
		if got := r.FormValue("chat_id"); got != chatID {
			t.Fatalf("unexpected chat_id: %s", got)
		}
		f, _, err := r.FormFile("document")
		if err != nil {
			t.Fatalf("document not found: %v", err)
		}
		defer func() { _ = f.Close() }()
		body, _ := io.ReadAll(f)
		if string(body) != payload {
			t.Fatalf("unexpected payload: %s", string(body))
		}
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":42,"document":{"file_id":"` + fileID + `"}}}`))
	})
	mux.HandleFunc("/bot"+token+"/getFile", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if got := r.FormValue("file_id"); got != fileID {
			t.Fatalf("unexpected file_id: %s", got)
		}
		_, _ = w.Write([]byte(`{"ok":true,"result":{"file_path":"` + fileRef + `"}}`))
	})
	mux.HandleFunc("/file/bot"+token+"/"+fileRef, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(payload))
	})
	mux.HandleFunc("/bot"+token+"/deleteMessage", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if got := r.FormValue("chat_id"); got != chatID {
			t.Fatalf("unexpected delete chat_id: %s", got)
		}
		if got := r.FormValue("message_id"); got != "42" {
			t.Fatalf("unexpected delete message_id: %s", got)
		}
		_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	cli, err := NewClientWithBaseURL(Config{Token: token, ChatID: chatID}, ts.URL)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	name, err := cli.Upload(context.Background(), "chunk.bin", strings.NewReader(payload))
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	var ref map[string]any
	if err := json.Unmarshal([]byte(name), &ref); err != nil {
		t.Fatalf("invalid chunk ref json: %v", err)
	}

	rc, err := cli.Download(context.Background(), name)
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	data, _ := io.ReadAll(rc)
	_ = rc.Close()
	if string(data) != payload {
		t.Fatalf("unexpected download data: %s", string(data))
	}

	if err := cli.Delete(context.Background(), name); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func TestConfigCodec(t *testing.T) {
	t.Parallel()
	cfg := Config{Token: "t", ChatID: "c"}
	raw, err := EncodeConfig(cfg)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	parsed, err := ParseConfig(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if parsed != cfg {
		t.Fatalf("unexpected parsed config: %+v", parsed)
	}
}

func TestCheckChat(t *testing.T) {
	t.Parallel()
	const (
		token  = "token123"
		chatID = "-100123"
	)
	mux := http.NewServeMux()
	mux.HandleFunc("/bot"+token+"/getChat", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if got := r.FormValue("chat_id"); got != chatID {
			t.Fatalf("unexpected chat_id: %s", got)
		}
		_, _ = w.Write([]byte(`{"ok":true,"result":{"id":-100123}}`))
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	cli, err := NewClientWithBaseURL(Config{Token: token, ChatID: chatID}, ts.URL)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	if err := cli.CheckChat(context.Background()); err != nil {
		t.Fatalf("check chat: %v", err)
	}
}

func TestProbeSourceHealth(t *testing.T) {
	t.Parallel()
	const (
		token  = "token123"
		chatID = "-100123"
	)
	mux := http.NewServeMux()
	mux.HandleFunc("/bot"+token+"/getChat", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if got := r.FormValue("chat_id"); got != chatID {
			t.Fatalf("unexpected chat_id: %s", got)
		}
		_, _ = w.Write([]byte(`{"ok":true,"result":{"id":-100123}}`))
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	cli, err := NewClientWithBaseURL(Config{Token: token, ChatID: chatID}, ts.URL)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	health, err := cli.ProbeSourceHealth(context.Background())
	if err != nil {
		t.Fatalf("source health: %v", err)
	}
	if health.Status != sourcecap.HealthHealthy || health.ReasonCode != "ok" {
		t.Fatalf("unexpected health: %+v", health)
	}
}
