package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/crypto/argon2"
)

const (
	saltLen    = 16
	keyLen     = 32
	time       = 1
	memory     = 64 * 1024
	threads    = 4
	keyringFile = "keyring.bin"
)

type KeyringData struct {
	DevicePrivateKey ed25519.PrivateKey `json:"device_private_key"`
	PasetoToken      string             `json:"paseto_token"`
	UserID           string             `json:"user_id"`
	DeviceID         string             `json:"device_id"`
}

type Keyring struct {
	data       *KeyringData
	masterKey  []byte
	path       string
}

func NewKeyring(dir string) *Keyring {
	return &Keyring{
		path: filepath.Join(dir, keyringFile),
	}
}

// DeriveKey derives a master key from a password and salt.
func DeriveKey(password string, salt []byte) []byte {
	return argon2.IDKey([]byte(password), salt, time, memory, threads, keyLen)
}

// Unlock attempts to decrypt the keyring with the provided password.
func (k *Keyring) Unlock(password string) error {
	data, err := os.ReadFile(k.path)
	if err != nil {
		if os.IsNotExist(err) {
			return errors.New("keyring does not exist")
		}
		return err
	}

	if len(data) < saltLen+aes.BlockSize {
		return errors.New("keyring file too small")
	}

	salt := data[:saltLen]
	ciphertext := data[saltLen:]

	masterKey := DeriveKey(password, salt)
	
	plaintext, err := decrypt(ciphertext, masterKey)
	if err != nil {
		return errors.New("invalid password or corrupted keyring")
	}

	var keyringData KeyringData
	err = json.Unmarshal(plaintext, &keyringData)
	if err != nil {
		return fmt.Errorf("failed to unmarshal keyring data: %w", err)
	}

	k.data = &keyringData
	k.masterKey = masterKey
	return nil
}

// Init initializes a new keyring with a password.
func (k *Keyring) Init(password string, initialData *KeyringData) error {
	salt := make([]byte, saltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return err
	}

	masterKey := DeriveKey(password, salt)
	
	plaintext, err := json.Marshal(initialData)
	if err != nil {
		return err
	}

	ciphertext, err := encrypt(plaintext, masterKey)
	if err != nil {
		return err
	}

	// Store salt + ciphertext
	finalData := append(salt, ciphertext...)
	err = os.MkdirAll(filepath.Dir(k.path), 0700)
	if err != nil {
		return err
	}

	err = os.WriteFile(k.path, finalData, 0600)
	if err != nil {
		return err
	}

	k.data = initialData
	k.masterKey = masterKey
	return nil
}

func (k *Keyring) Save() error {
	if k.data == nil || k.masterKey == nil {
		return errors.New("keyring not unlocked")
	}

	// We need the original salt to re-encrypt with the same master key.
	// Or we can generate a new salt and new master key from the same password.
	// But we don't store the password.
	// Simplest: Read the salt from the existing file.
	existingData, err := os.ReadFile(k.path)
	if err != nil {
		return err
	}
	salt := existingData[:saltLen]

	plaintext, err := json.Marshal(k.data)
	if err != nil {
		return err
	}

	ciphertext, err := encrypt(plaintext, k.masterKey)
	if err != nil {
		return err
	}

	finalData := append(salt, ciphertext...)
	return os.WriteFile(k.path, finalData, 0600)
}

func (k *Keyring) GetData() *KeyringData {
	return k.data
}

func (k *Keyring) GetToken() string {
	if k.data == nil {
		return ""
	}
	return k.data.PasetoToken
}

func encrypt(plaintext []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func decrypt(ciphertext []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	if len(ciphertext) < gcm.NonceSize() {
		return nil, errors.New("ciphertext too short")
	}

	nonce, actualCiphertext := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	return gcm.Open(nil, nonce, actualCiphertext, nil)
}
