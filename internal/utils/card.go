package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// GenerateCardNumber generates a card number with the specified prefix and length
func GenerateCardNumber(prefix string, length int) (string, error) {
	if length < len(prefix) || length > 19 {
		return "", fmt.Errorf("invalid card number length: %d", length)
	}

	// Generate random digits
	digits := make([]byte, length-len(prefix))
	_, err := rand.Read(digits)
	if err != nil {
		return "", fmt.Errorf("failed to generate random digits: %w", err)
	}

	// Convert to string and ensure valid digits
	var builder strings.Builder
	builder.WriteString(prefix)
	for _, b := range digits {
		digit := b%10 + '0' // Convert to ASCII digit
		builder.WriteByte(digit)
	}

	cardNumber := builder.String()

	// Ensure length is exact
	if len(cardNumber) != length {
		return "", fmt.Errorf("generated card number has incorrect length: got %d, want %d", len(cardNumber), length)
	}

	return cardNumber, nil
}

// GenerateExpiryDate generates a card expiry date (MM/YY)
func GenerateExpiryDate() string {
	now := time.Now()
	year := now.Year() + 3 // Cards valid for 3 years
	month := now.Month()
	return fmt.Sprintf("%02d/%02d", month, year%100)
}

// GenerateCVV generates a 3-digit CVV code
func GenerateCVV() string {
	b := make([]byte, 3)
	rand.Read(b)
	return fmt.Sprintf("%03d", (int(b[0])%10)*100+(int(b[1])%10)*10+int(b[2])%10)
}

// GenerateHMAC generates an HMAC for card details
func GenerateHMAC(cardNumber, expiryDate, cvv, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	data := cardNumber + expiryDate + cvv
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

// Encrypt encrypts a string using AES with PKCS#5/PKCS#7 padding
func Encrypt(data string, key []byte) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("input data is empty")
	}
	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		return "", fmt.Errorf("encryption key must be 16, 24, or 32 bytes, got %d", len(key))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	// Generate IV
	iv := make([]byte, aes.BlockSize)
	_, err = rand.Read(iv)
	if err != nil {
		return "", fmt.Errorf("failed to generate IV: %w", err)
	}

	// Add PKCS#5/PKCS#7 padding
	dataBytes := []byte(data)
	padding := aes.BlockSize - len(dataBytes)%aes.BlockSize
	for i := 0; i < padding; i++ {
		dataBytes = append(dataBytes, byte(padding))
	}

	// Encrypt
	ciphertext := make([]byte, len(dataBytes))
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext, dataBytes)

	// Combine IV and ciphertext
	final := append(iv, ciphertext...)
	return hex.EncodeToString(final), nil
}

// Decrypt decrypts a hex-encoded string using AES with PKCS#5/PKCS#7 padding
func Decrypt(encryptedData string, key []byte) (string, error) {
	if len(encryptedData) == 0 {
		return "", fmt.Errorf("encrypted data is empty")
	}
	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		return "", fmt.Errorf("decryption key must be 16, 24, or 32 bytes, got %d", len(key))
	}

	// Decode hex
	data, err := hex.DecodeString(encryptedData)
	if err != nil {
		return "", fmt.Errorf("failed to decode hex: %w", err)
	}

	if len(data) < aes.BlockSize {
		return "", fmt.Errorf("encrypted data too short: %d bytes", len(data))
	}

	// Extract IV and ciphertext
	iv := data[:aes.BlockSize]
	ciphertext := data[aes.BlockSize:]

	if len(ciphertext) == 0 {
		return "", fmt.Errorf("ciphertext is empty")
	}
	if len(ciphertext)%aes.BlockSize != 0 {
		return "", fmt.Errorf("invalid ciphertext length: %d bytes", len(ciphertext))
	}

	// Create cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	// Decrypt
	plaintext := make([]byte, len(ciphertext))
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(plaintext, ciphertext)

	// Remove PKCS#5/PKCS#7 padding
	padding := int(plaintext[len(plaintext)-1])
	if padding > aes.BlockSize || padding == 0 {
		return "", fmt.Errorf("invalid padding value: %d", padding)
	}
	for i := len(plaintext) - padding; i < len(plaintext); i++ {
		if int(plaintext[i]) != padding {
			return "", fmt.Errorf("invalid padding bytes: expected %d, got %d at position %d", padding, plaintext[i], i)
		}
	}

	return string(plaintext[:len(plaintext)-padding]), nil
}
