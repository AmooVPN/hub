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

type roundTripHTTPFunc func(*http.Request) (*http.Response, error)

func (f roundTripHTTPFunc) Do(req *http.Request) (*http.Response, error) { return f(req) }
