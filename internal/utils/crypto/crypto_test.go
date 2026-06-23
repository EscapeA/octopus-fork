package crypto

import (
	"errors"
	"sync"
	"testing"
)

func resetKey() {
	globalKey = nil
	globalKeyOnce = sync.Once{}
}

func TestRoundTrip(t *testing.T) {
	resetKey()
	Init("test-secret-key")

	plain := "sk-abc123-very-secret"
	enc, err := Encrypt(plain)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if !IsEncrypted(enc) {
		t.Fatal("encrypted value should have enc: prefix")
	}
	if enc == plain {
		t.Fatal("encrypted value should differ from plaintext")
	}

	dec, err := Decrypt(enc)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if dec != plain {
		t.Fatalf("Decrypt = %q, want %q", dec, plain)
	}
}

func TestEmptyPassthrough(t *testing.T) {
	resetKey()
	Init("key")

	enc, err := Encrypt("")
	if err != nil {
		t.Fatal(err)
	}
	if enc != "" {
		t.Fatalf("empty should pass through, got %q", enc)
	}

	dec, err := Decrypt("")
	if err != nil {
		t.Fatal(err)
	}
	if dec != "" {
		t.Fatalf("empty should pass through, got %q", dec)
	}
}

func TestLegacyPassthrough(t *testing.T) {
	resetKey()
	Init("key")

	plain := "sk-legacy-not-encrypted"
	dec, err := Decrypt(plain)
	if err != nil {
		t.Fatal(err)
	}
	if dec != plain {
		t.Fatalf("legacy value should pass through, got %q", dec)
	}
}

func TestNoKeyReturnsError(t *testing.T) {
	resetKey()

	plain := "secret"
	enc, err := Encrypt(plain)
	if !errors.Is(err, ErrNoKey) {
		t.Fatalf("without key, Encrypt should return ErrNoKey, got enc=%q err=%v", enc, err)
	}
	if enc != "" {
		t.Fatalf("without key, Encrypt should return empty string, got %q", enc)
	}

	// Empty plaintext still passes through (no key needed).
	if enc, err := Encrypt(""); err != nil || enc != "" {
		t.Fatalf("empty plaintext should pass through, got enc=%q err=%v", enc, err)
	}
}
