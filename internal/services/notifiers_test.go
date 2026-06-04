package services

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/smtp"
	"strings"
	"testing"
)

func TestTelegramNotifierSendTest(t *testing.T) {
	var body []byte
	notifier := NewTelegramNotifier("token", "chat")
	notifier.Client = roundTripHTTPFunc(func(req *http.Request) (*http.Response, error) {
		data, _ := io.ReadAll(req.Body)
		body = append([]byte(nil), data...)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"ok":true}`)), Header: http.Header{}}, nil
	})
	if err := notifier.SendTest(context.Background(), "hello"); err != nil {
		t.Fatalf("send test failed: %v", err)
	}
	if !bytes.Contains(body, []byte(`"chat_id":"chat"`)) {
		t.Fatalf("expected chat id in payload: %s", string(body))
	}
}

func TestTelegramNotifierRetries(t *testing.T) {
	var attempts int
	notifier := NewTelegramNotifier("token", "chat")
	notifier.Client = roundTripHTTPFunc(func(req *http.Request) (*http.Response, error) {
		attempts++
		if attempts < 3 {
			return &http.Response{StatusCode: 500, Body: io.NopCloser(strings.NewReader("boom")), Header: http.Header{}}, nil
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"ok":true}`)), Header: http.Header{}}, nil
	})
	if err := notifier.SendNotification(context.Background(), "warning", "title", "message"); err != nil {
		t.Fatalf("send notification failed: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

func TestEmailNotifierSendTest(t *testing.T) {
	oldSendMail := sendMail
	defer func() { sendMail = oldSendMail }()
	var gotAddr string
	var gotFrom string
	var gotTo []string
	var gotMsg []byte
	sendMail = func(addr string, auth smtp.Auth, from string, to []string, msg []byte) error {
		gotAddr = addr
		gotFrom = from
		gotTo = append([]string(nil), to...)
		gotMsg = append([]byte(nil), msg...)
		return nil
	}
	notifier := NewEmailNotifier("smtp.example.com", 2525, "user", "pass", "from@example.com")
	if err := notifier.SendTest(context.Background(), "to@example.com", "subject", "body"); err != nil {
		t.Fatalf("send test failed: %v", err)
	}
	if gotAddr != "smtp.example.com:2525" || gotFrom != "from@example.com" || len(gotTo) != 1 || gotTo[0] != "to@example.com" {
		t.Fatalf("unexpected email send args: %s %s %v", gotAddr, gotFrom, gotTo)
	}
	if !bytes.Contains(gotMsg, []byte("subject")) || !bytes.Contains(gotMsg, []byte("body")) {
		t.Fatalf("unexpected email body: %s", string(gotMsg))
	}
}

func TestEmailNotifierTemplates(t *testing.T) {
	if got := renderEmailSubject("warning", "Panel offline"); got != "[WARNING] Panel offline" {
		t.Fatalf("unexpected subject %q", got)
	}
	body := renderEmailBody("danger", "Sync failed", "something went wrong")
	if !strings.Contains(body, "Severity: DANGER") || !strings.Contains(body, "something went wrong") {
		t.Fatalf("unexpected body %q", body)
	}
}

type roundTripHTTPFunc func(*http.Request) (*http.Response, error)

func (f roundTripHTTPFunc) Do(req *http.Request) (*http.Response, error) { return f(req) }
