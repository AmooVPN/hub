package security

import (
	"testing"
	"time"
)

func TestAccessTokenRoundTrip(t *testing.T) {
	token, err := SignAccessToken("42", time.Minute, "secret")
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	claims, err := ValidateAccessToken(token, "secret")
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if claims.Subject != "42" {
		t.Fatalf("subject = %q", claims.Subject)
	}
}

func TestRefreshTokenHash(t *testing.T) {
	if HashRefreshToken("abc") == HashRefreshToken("xyz") {
		t.Fatal("hashes should differ")
	}
}
