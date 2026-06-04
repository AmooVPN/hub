package app

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/AmooVPN/hub/internal/models"
)

func TestRemoteAttachmentIdentity(t *testing.T) {
	client := &models.Client{ID: 7, Username: "Ali Example"}
	inbound := &models.Inbound{ID: 11}
	if got := remoteAttachmentIdentity(client, inbound); got != "hub_ali_example_7_11" {
		t.Fatalf("unexpected identity: %s", got)
	}
}

func TestGenerateAttachmentConfig(t *testing.T) {
	panel := &models.Panel{ID: 1, BaseURL: "https://panel.example.com", Username: "admin"}
	attachment := &models.ClientAttachment{RemoteClientID: "client-id", RemoteEmail: "client@example.com"}

	tests := []struct {
		name     string
		inbound  *models.Inbound
		contains []string
	}{
		{name: "vless", inbound: &models.Inbound{ID: 2, Remark: "VLESS", Protocol: "vless", Port: 443, Network: "tcp", Security: "reality"}, contains: []string{"vless://client-id@panel.example.com:443", "type=tcp", "security=reality"}},
		{name: "trojan", inbound: &models.Inbound{ID: 3, Remark: "Trojan", Protocol: "trojan", Port: 8443, Network: "ws", Security: "tls"}, contains: []string{"trojan://client-id@panel.example.com:8443", "type=ws", "security=tls"}},
		{name: "vmess", inbound: &models.Inbound{ID: 4, Remark: "VMess", Protocol: "vmess", Port: 443, Network: "ws", Security: "tls"}, contains: []string{"vmess://"}},
		{name: "shadowsocks", inbound: &models.Inbound{ID: 5, Remark: "SS", Protocol: "shadowsocks", Port: 8388, Network: "tcp"}, contains: []string{"ss://", "#client%40example.com"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := generateAttachmentConfig(panel, tc.inbound, attachment)
			for _, expected := range tc.contains {
				if !strings.Contains(got, expected) {
					t.Fatalf("expected %q in %q", expected, got)
				}
			}
			if tc.name == "vmess" {
				payload, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(got, "vmess://"))
				if err != nil {
					t.Fatalf("decode vmess payload: %v", err)
				}
				if !strings.Contains(string(payload), "panel.example.com") || !strings.Contains(string(payload), "client-id") {
					t.Fatalf("unexpected vmess payload: %s", payload)
				}
			}
		})
	}
}
