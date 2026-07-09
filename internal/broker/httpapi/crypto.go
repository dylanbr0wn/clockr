package httpapi

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// tokenPayload is the short-lived Google token material sealed into a handoff
// record. It is never persisted in plaintext columns.
type tokenPayload struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	TokenType    string    `json:"token_type,omitempty"`
	Expiry       time.Time `json:"expiry,omitempty"`
}

func hashHandoffCode(code string) string {
	sum := sha256.Sum256([]byte(code))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func handoffAAD(stateID, desktopSessionID, handoffChallenge string) []byte {
	return []byte(stateID + "|" + desktopSessionID + "|" + handoffChallenge)
}

func encryptTokenPayload(secret string, aad []byte, payload tokenPayload) ([]byte, error) {
	plain, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal token payload: %w", err)
	}
	key := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(key[:])
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
	return gcm.Seal(nonce, nonce, plain, aad), nil
}

func decryptTokenPayload(secret string, aad, ciphertext []byte) (tokenPayload, error) {
	if len(ciphertext) == 0 {
		return tokenPayload{}, fmt.Errorf("empty token payload")
	}
	key := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return tokenPayload{}, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return tokenPayload{}, err
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return tokenPayload{}, fmt.Errorf("ciphertext too short")
	}
	nonce, sealed := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plain, err := gcm.Open(nil, nonce, sealed, aad)
	if err != nil {
		return tokenPayload{}, err
	}
	var payload tokenPayload
	if err := json.Unmarshal(plain, &payload); err != nil {
		return tokenPayload{}, fmt.Errorf("unmarshal token payload: %w", err)
	}
	return payload, nil
}
