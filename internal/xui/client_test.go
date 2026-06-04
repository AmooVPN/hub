package xui

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestXUIClientLoginAndRetryAfterSessionExpiry(t *testing.T) {
	var loginCount int32
	var listCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.URL.Path == "/login" && req.Method == http.MethodPost:
			atomic.AddInt32(&loginCount, 1)
			w.WriteHeader(http.StatusOK)
		case req.URL.Path == "/panel/api/inbounds/list" && req.Method == http.MethodGet:
			count := atomic.AddInt32(&listCount, 1)
			if count == 1 {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []XUIInbound{{ID: 1, Remark: "inbound-1", Protocol: "vless", Enabled: true}},
			})
		default:
			http.NotFound(w, req)
		}
	}))
	defer server.Close()

	client := NewClient(42, server.URL, "admin", "panel-password", "")
	if err := client.Login(context.Background()); err != nil {
		t.Fatalf("login: %v", err)
	}
	inbounds, err := client.ListInbounds(context.Background())
	if err != nil {
		t.Fatalf("list inbounds: %v", err)
	}
	if len(inbounds) != 1 || inbounds[0].Remark != "inbound-1" {
		t.Fatalf("unexpected inbounds: %+v", inbounds)
	}
	if got := atomic.LoadInt32(&loginCount); got != 2 {
		t.Fatalf("expected login retry, got %d logins", got)
	}
	if got := atomic.LoadInt32(&listCount); got != 2 {
		t.Fatalf("expected retry of list request, got %d requests", got)
	}
}

func TestXUIClientAddClientSendsJSON(t *testing.T) {
	var body []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.URL.Path == "/login" && req.Method == http.MethodPost:
			w.WriteHeader(http.StatusOK)
		case req.URL.Path == "/panel/api/inbounds/7/client/add" && req.Method == http.MethodPost:
			var err error
			body, err = io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}
			_ = json.NewEncoder(w).Encode(XUIClientResult{ID: "remote-1", Email: "client@example.com", Enable: true})
		default:
			http.NotFound(w, req)
		}
	}))
	defer server.Close()

	client := NewClient(7, server.URL, "admin", "panel-password", "")
	if err := client.Login(context.Background()); err != nil {
		t.Fatalf("login: %v", err)
	}
	result, err := client.AddClient(context.Background(), 7, XUIClientCreateRequest{Email: "client@example.com", ID: "remote-1", Enable: true, TotalGB: 10})
	if err != nil {
		t.Fatalf("add client: %v", err)
	}
	if result.ID != "remote-1" || !result.Enable {
		t.Fatalf("unexpected result: %+v", result)
	}
	if !strings.Contains(string(body), `"email":"client@example.com"`) || !strings.Contains(string(body), `"id":"remote-1"`) {
		t.Fatalf("expected JSON payload, got %s", string(body))
	}
}

func TestXUIClientNormalizesTimeoutsAndHidesSecrets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.URL.Path == "/login" && req.Method == http.MethodPost:
			w.WriteHeader(http.StatusOK)
		case req.URL.Path == "/panel/api/inbounds/list" && req.Method == http.MethodGet:
			time.Sleep(100 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, req)
		}
	}))
	defer server.Close()

	client := NewClient(1, server.URL, "admin", "super-secret-password", "")
	client.HTTPClient.Timeout = 10 * time.Millisecond

	if err := client.Login(context.Background()); err != nil {
		t.Fatalf("login: %v", err)
	}
	_, err := client.ListInbounds(context.Background())
	if !errors.Is(err, ErrXUITimeout) {
		t.Fatalf("expected timeout, got %v", err)
	}
	for _, secret := range []string{client.BaseURL, client.Password} {
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("error leaked secret %q: %v", secret, err)
		}
	}
}

func TestXUIClientUsesBearerTokenWithoutLogin(t *testing.T) {
	var loginCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.URL.Path == "/login" && req.Method == http.MethodPost:
			atomic.AddInt32(&loginCount, 1)
			w.WriteHeader(http.StatusUnauthorized)
		case req.URL.Path == "/panel/api/inbounds/list" && req.Method == http.MethodGet:
			if got := req.Header.Get("Authorization"); got != "Bearer panel-token" {
				t.Fatalf("unexpected authorization header: %q", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []XUIInbound{{ID: 1, Remark: "token-inbound", Protocol: "vless", Enabled: true}}})
		default:
			http.NotFound(w, req)
		}
	}))
	defer server.Close()

	client := NewClient(9, server.URL, "admin", "", "panel-token")
	if err := client.Login(context.Background()); err != nil {
		t.Fatalf("login: %v", err)
	}
	inbounds, err := client.ListInbounds(context.Background())
	if err != nil {
		t.Fatalf("list inbounds: %v", err)
	}
	if len(inbounds) != 1 || inbounds[0].Remark != "token-inbound" {
		t.Fatalf("unexpected inbounds: %+v", inbounds)
	}
	if got := atomic.LoadInt32(&loginCount); got != 0 {
		t.Fatalf("expected no login requests, got %d", got)
	}
}
