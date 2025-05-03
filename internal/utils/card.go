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
		return "", fmt.Errorf("invalid card number length")
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
		return "", fmt.Errorf("generated card number has incorrect length")
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

func Encrypt(data string, key []byte) (string, error) {
	// Проверка длины ключа
	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		return "", fmt.Errorf("encryption key must be 16, 24, or 32 bytes, got %d", len(key))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	// Генерация IV
	iv := make([]byte, aes.BlockSize)
	_, err = rand.Read(iv)
	if err != nil {
		return "", fmt.Errorf("failed to generate IV: %w", err)
	}

	// Добавление отступов
	dataBytes := []byte(data)
	if len(dataBytes)%aes.BlockSize != 0 {
		padding := aes.BlockSize - len(dataBytes)%aes.BlockSize
		for i := 0; i < padding; i++ {
			dataBytes = append(dataBytes, byte(padding))
		}
	}

	// Шифрование
	ciphertext := make([]byte, len(dataBytes))
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext, dataBytes)

	// Комбинирование IV и шифрованного текста
	final := append(iv, ciphertext...)
	return hex.EncodeToString(final), nil
}

func Decrypt(encryptedData string, key []byte) (string, error) {
	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		return "", fmt.Errorf("decryption key must be 16, 24, or 32 bytes, got %d", len(key))
	}

	// Декодирование hex
	data, err := hex.DecodeString(encryptedData)
	if err != nil {
		return "", fmt.Errorf("failed to decode hex: %w", err)
	}

	if len(data) < aes.BlockSize {
		return "", fmt.Errorf("invalid encrypted data length")
	}

	// Извлечение IV и шифрованного текста
	iv := data[:aes.BlockSize]
	ciphertext := data[aes.BlockSize:]

	// Создание шифра
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	// Расшифровка
	if len(ciphertext)%aes.BlockSize != 0 {
		return "", fmt.Errorf("invalid ciphertext length")
	}

	plaintext := make([]byte, len(ciphertext))
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(plaintext, ciphertext)

	// Удаление отступов
	padding := int(plaintext[len(plaintext)-1])
	if padding > aes.BlockSize || padding == 0 {
		return "", fmt.Errorf("invalid padding")
	}
	for i := len(plaintext) - padding; i < len(plaintext); i++ {
		if int(plaintext[i]) != padding {
			return "", fmt.Errorf("invalid padding bytes")
		}
	}

	return string(plaintext[:len(plaintext)-padding]), nil
}
