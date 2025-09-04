package cryptoutil

import "testing"

func TestEncryptDecrypt(t *testing.T) {
	key := "testkey"
	s, err := GenerateSecret()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	enc, err := Encrypt(s, key)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	dec, err := Decrypt(enc, key)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if dec != s {
		t.Fatalf("expected %s, got %s", s, dec)
	}
}
