package argon2

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	// Default parameters for Argon2id
	time    = 1
	memory  = 64 * 1024
	threads = 4
	keyLen  = 32
	saltLen = 16
)

// HashPassword hashes a password using Argon2id and returns a string containing
// the salt and parameters in a PHC-like format.
func HashPassword(password string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("argon2: failed to generate salt: %w", err)
	}

	hash := argon2.IDKey([]byte(password), salt, time, memory, threads, keyLen)

	// Format: $argon2id$v=19$m=65536,t=1,p=4$salt$hash
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	encoded := fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, memory, time, threads, b64Salt, b64Hash)
	return encoded, nil
}

// VerifyPassword compares a password against an encoded hash.
func VerifyPassword(password, encodedHash string) (bool, error) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 {
		return false, fmt.Errorf("argon2: invalid hash format")
	}

	if parts[1] != "argon2id" {
		return false, fmt.Errorf("argon2: incompatible variant: %s", parts[1])
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false, fmt.Errorf("argon2: invalid version: %w", err)
	}
	if version != argon2.Version {
		return false, fmt.Errorf("argon2: incompatible version: %d", version)
	}

	var m, t, p uint32
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
		return false, fmt.Errorf("argon2: invalid parameters: %w", err)
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("argon2: failed to decode salt: %w", err)
	}

	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, fmt.Errorf("argon2: failed to decode hash: %w", err)
	}

	comparisonHash := argon2.IDKey([]byte(password), salt, t, m, uint8(p), uint32(len(hash)))

	return subtle.ConstantTimeCompare(hash, comparisonHash) == 1, nil
}
