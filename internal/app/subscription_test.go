package app

import "testing"

func TestBuildSubscriptionRaw(t *testing.T) {
	raw := buildSubscriptionRaw([]clientConfigRow{{CopyValue: "vless://one"}, {CopyValue: ""}, {CopyValue: "trojan://two"}})
	if raw != "vless://one\ntrojan://two" {
		t.Fatalf("unexpected raw subscription: %q", raw)
	}
}

func TestBuildSubscriptionRawEmpty(t *testing.T) {
	if got := buildSubscriptionRaw(nil); got != "" {
		t.Fatalf("expected empty raw subscription, got %q", got)
	}
}
