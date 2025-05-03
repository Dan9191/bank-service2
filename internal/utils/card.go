package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// GenerateCardNumber generates a valid card number using Luhn algorithm
func GenerateCardNumber(prefix string, length int) (string, error) {
	if len(prefix) >= length || len(prefix) < 1 {
		return "", fmt.Errorf("invalid prefix length for card number")
	}

	// Fill with random digits
	digits := make([]int, length)
	for i := 0; i < len(prefix); i++ {
		d, err := strconv.Atoi(string(prefix[i]))
		if err != nil {
			return "", fmt.Errorf("invalid prefix digit: %v", err)
		}
		digits[i] = d
	}
	for i := len(prefix); i < length-1; i++ {
		var b [1]byte
		_, err := rand.Read(b[:])
		if err != nil {
			return "", fmt.Errorf("failed to generate random digit: %v", err)
		}
		digits[i] = int(b[0] % 10)
	}

	// Calculate check digit using Luhn algorithm
	sum := 0
	isEven := false
	for i := length - 1; i >= 0; i-- {
		d := digits[i]
		if isEven {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
		isEven = !isEven
	}
	checkDigit := (10 - (sum % 10)) % 10
	digits[length-1] = checkDigit

	// Convert to string
	var sb strings.Builder
	for _, d := range digits {
		sb.WriteString(strconv.Itoa(d))
	}
	return sb.String(), nil
}

// GenerateExpiryDate generates an expiry date (MM/YY)
func GenerateExpiryDate() string {
	now := time.Now()
	year := now.Year() + 4 // Valid for 4 years
	month := now.Month()
	return fmt.Sprintf("%02d/%02d", month, year%100)
}

// GenerateCVV generates a 3-digit CVV
func GenerateCVV() string {
	var b [2]byte
	_, err := rand.Read(b[:])
	if err != nil {
		return "000" // Fallback in case of error
	}
	num := int(b[0])<<8 | int(b[1])
	return fmt.Sprintf("%03d", num%1000)
}

// GenerateHMAC creates an HMAC for card data
func GenerateHMAC(cardNumber, expiryDate, cvv, secret string) string {
	data := cardNumber + expiryDate + cvv
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(data))
	return fmt.Sprintf("%x", h.Sum(nil))
}

// Encrypt encrypts data using AES-GCM
func Encrypt(plaintext, key string) (string, error) {
	keyBytes, err := hex.DecodeString(key)
	if err != nil {
		return "", fmt.Errorf("invalid key: %v", err)
	}
	if len(keyBytes) != 32 {
		return "", fmt.Errorf("key must be 32 bytes")
	}

	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %v", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %v", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %v", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return hex.EncodeToString(ciphertext), nil
}
