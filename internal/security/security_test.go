package security

import "testing"

func TestEncryptDecryptRoundTrip(t *testing.T) {
	got, err := Encrypt("secret-value", "super-secret-key")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	plain, err := Decrypt(got, "super-secret-key")
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if plain != "secret-value" {
		t.Fatalf("unexpected plaintext %q", plain)
	}
}

func TestPasswordHashCompare(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if err := ComparePassword("correct horse battery staple", hash); err != nil {
		t.Fatalf("compare: %v", err)
	}
	if err := ComparePassword("wrong", hash); err == nil {
		t.Fatal("expected mismatch error")
	}
}
