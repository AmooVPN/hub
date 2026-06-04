package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

var sendMail = smtp.SendMail

type TelegramNotifier struct {
	BotToken string
	ChatID   string
	Client   HTTPDoer
}

type EmailNotifier struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

func NewTelegramNotifier(token, chatID string) *TelegramNotifier {
	return &TelegramNotifier{BotToken: strings.TrimSpace(token), ChatID: strings.TrimSpace(chatID), Client: http.DefaultClient}
}

func NewEmailNotifier(host string, port int, username, password, from string) *EmailNotifier {
	return &EmailNotifier{Host: strings.TrimSpace(host), Port: port, Username: strings.TrimSpace(username), Password: password, From: strings.TrimSpace(from)}
}

func (n *TelegramNotifier) Configured() bool {
	return n != nil && n.BotToken != "" && n.ChatID != ""
}

func (n *TelegramNotifier) SendTest(ctx context.Context, message string) error {
	if n == nil {
		return errors.New("telegram notifier is not configured")
	}
	if !n.Configured() {
		return errors.New("telegram notifier is not configured")
	}
	payload := map[string]any{"chat_id": n.ChatID, "text": message, "disable_web_page_preview": true}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.telegram.org/bot"+n.BotToken+"/sendMessage", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := n.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("telegram api returned %d", resp.StatusCode)
	}
	return nil
}

func (n *EmailNotifier) Configured() bool {
	return n != nil && n.Host != "" && n.From != ""
}

func (n *EmailNotifier) SendTest(ctx context.Context, to, subject, body string) error {
	if n == nil {
		return errors.New("email notifier is not configured")
	}
	if !n.Configured() {
		return errors.New("email notifier is not configured")
	}
	if strings.TrimSpace(to) == "" {
		return errors.New("recipient email is required")
	}
	addr := n.Host + ":" + strconv.Itoa(n.Port)
	msg := []byte("To: " + to + "\r\n" + "From: " + n.From + "\r\n" + "Subject: " + subject + "\r\n" + "Content-Type: text/plain; charset=UTF-8\r\n\r\n" + body + "\r\n")
	var auth smtp.Auth
	if n.Username != "" {
		auth = smtp.PlainAuth("", n.Username, n.Password, n.Host)
	}
	return sendMail(addr, auth, n.From, []string{to}, msg)
}

func (n *EmailNotifier) MaskedAddress() string {
	if n == nil || n.Host == "" {
		return "disabled"
	}
	return n.Host + ":" + strconv.Itoa(n.Port)
}

func (n *EmailNotifier) Timeout() time.Duration { return 0 }
