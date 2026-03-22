package repository

import (
	"testing"
)

func TestVerifyPassword_SHA256(t *testing.T) {
	// Magento version 1: SHA256(salt + password)
	// Pre-computed: sha256("testsalt" + "mypassword") = known hash
	// We'll test the round-trip instead: hash then verify
	salt := "testsalt"
	password := "mypassword"
	hash := mageSHA256Hash(salt, password)
	fullHash := hash + ":" + salt + ":1"

	if !VerifyPassword(fullHash, password) {
		t.Error("SHA256 password verification failed")
	}
	if VerifyPassword(fullHash, "wrongpassword") {
		t.Error("SHA256 should reject wrong password")
	}
}

func TestVerifyPassword_Argon2id(t *testing.T) {
	// Known Magento 2.4.8 hash for "roni_cost3@example.com"
	// Truncated to 16-byte salt as sodium requires
	knownHash := "ac64d043464f913b1ca7e1a65dfaf2b9a4ccef11040fbeb96ff559f74677ff79:ZmhZ5CUERQyYinxGCwiWpD6buW1D8Zhe:3_32_2_67108864"

	if !VerifyPassword(knownHash, "roni_cost3@example.com") {
		t.Error("Argon2id verification failed for known Magento hash")
	}
	if VerifyPassword(knownHash, "wrongpassword") {
		t.Error("Argon2id should reject wrong password")
	}
}

func TestVerifyPassword_EmptyHash(t *testing.T) {
	if VerifyPassword("", "password") {
		t.Error("empty hash should return false")
	}
	if VerifyPassword("nocolon", "password") {
		t.Error("hash without colon should return false")
	}
}

func TestVerifyPassword_UnsupportedVersion(t *testing.T) {
	if VerifyPassword("hash:salt:99", "password") {
		t.Error("unsupported version should return false")
	}
}

func TestHashPassword_RoundTrip(t *testing.T) {
	password := "TestPassword123!"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	// Hash should be in format: hex_hash:salt:3_32_2_67108864
	parts := splitN(hash, ":", 3)
	if len(parts) != 3 {
		t.Fatalf("hash format wrong, expected 3 parts, got %d: %s", len(parts), hash)
	}

	// Hex hash should be 64 chars (32 bytes)
	if len(parts[0]) != 64 {
		t.Errorf("hash hex length: got %d, want 64", len(parts[0]))
	}

	// Version should indicate Argon2id
	if parts[2] != "3_32_2_67108864" {
		t.Errorf("version: got %q, want %q", parts[2], "3_32_2_67108864")
	}

	// Round-trip: verify the password against the hash we just generated
	if !VerifyPassword(hash, password) {
		t.Error("round-trip verification failed")
	}
	if VerifyPassword(hash, "WrongPassword") {
		t.Error("should reject wrong password")
	}
}

func TestMageSHA256Hash(t *testing.T) {
	// sha256("abc") = ba7816bf...
	// But mageSHA256Hash does sha256(salt + password), so sha256("salted" + "pw")
	hash := mageSHA256Hash("", "")
	// sha256("") = e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
	if hash != "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Errorf("sha256 of empty string: got %s", hash)
	}
}

// helper to avoid importing strings in test
func splitN(s, sep string, n int) []string {
	result := make([]string, 0, n)
	for i := 0; i < n-1; i++ {
		idx := -1
		for j := 0; j < len(s); j++ {
			if s[j] == sep[0] {
				idx = j
				break
			}
		}
		if idx < 0 {
			break
		}
		result = append(result, s[:idx])
		s = s[idx+1:]
	}
	result = append(result, s)
	return result
}
