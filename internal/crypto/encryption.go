package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"golang.org/x/crypto/pbkdf2"
)

const (
	// Magic bytes for encrypted files: "PWBK"
	fileMagic = "PWBK"
	// Version of the encryption format
	fileVersion = uint16(1)
	// Algorithm identifier for AES-256-GCM
	algorithmAES256GCM = uint16(1)
	// Salt length in bytes
	saltLength = 32
	// Nonce length for GCM
	nonceLength = 12
	// Key derivation iterations
	pbkdf2Iterations = 100000
	// Key length for AES-256
	keyLength = 32
)

// EncryptedFileHeader represents the header of an encrypted file
type EncryptedFileHeader struct {
	Magic     [4]byte  // "PWBK"
	Version   uint16   // File format version
	Algorithm uint16   // Encryption algorithm identifier
	Reserved  [8]byte  // Reserved for future use
	Salt      [32]byte // Salt for key derivation
	Nonce     [12]byte // Nonce for GCM
}

// GenerateKey generates a new encryption key from a password
func GenerateKey(password string, salt []byte) []byte {
	return pbkdf2.Key([]byte(password), salt, pbkdf2Iterations, keyLength, sha256.New)
}

// GenerateSalt generates a random salt
func GenerateSalt() ([]byte, error) {
	salt := make([]byte, saltLength)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("failed to generate salt: %w", err)
	}
	return salt, nil
}

// Encrypt encrypts data using AES-256-GCM with the provided password
func Encrypt(plaintext []byte, password string) ([]byte, error) {
	// Generate a random salt
	salt, err := GenerateSalt()
	if err != nil {
		return nil, err
	}

	// Derive key from password
	key := GenerateKey(password, salt)

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate nonce
	nonce := make([]byte, nonceLength)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt data
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	// Build header
	header := EncryptedFileHeader{
		Version:   fileVersion,
		Algorithm: algorithmAES256GCM,
	}
	copy(header.Magic[:], fileMagic)
	copy(header.Salt[:], salt)
	copy(header.Nonce[:], nonce)

	// Combine header and ciphertext
	result := make([]byte, 0, len(header.Magic)+2+2+len(header.Reserved)+len(header.Salt)+len(header.Nonce)+len(ciphertext))
	result = append(result, header.Magic[:]...)
	result = append(result, byte(header.Version>>8), byte(header.Version))
	result = append(result, byte(header.Algorithm>>8), byte(header.Algorithm))
	result = append(result, header.Reserved[:]...)
	result = append(result, header.Salt[:]...)
	result = append(result, header.Nonce[:]...)
	result = append(result, ciphertext...)

	// Clear sensitive data
	clearBytes(key)

	return result, nil
}

// Decrypt decrypts data using AES-256-GCM with the provided password
func Decrypt(ciphertext []byte, password string) ([]byte, error) {
	// Check minimum length
	minLength := 4 + 2 + 2 + 8 + 32 + 12 + 16 // header + minimum ciphertext with auth tag
	if len(ciphertext) < minLength {
		return nil, fmt.Errorf("ciphertext too short")
	}

	// Parse header
	offset := 0

	// Check magic
	magic := ciphertext[offset : offset+4]
	offset += 4
	if string(magic) != fileMagic {
		return nil, fmt.Errorf("invalid file format: bad magic bytes")
	}

	// Read version
	version := binary.BigEndian.Uint16(ciphertext[offset : offset+2])
	offset += 2
	if version != fileVersion {
		return nil, fmt.Errorf("unsupported file version: %d", version)
	}

	// Read algorithm
	algorithm := binary.BigEndian.Uint16(ciphertext[offset : offset+2])
	offset += 2
	if algorithm != algorithmAES256GCM {
		return nil, fmt.Errorf("unsupported algorithm: %d", algorithm)
	}

	// Skip reserved bytes
	offset += 8

	// Read salt
	salt := ciphertext[offset : offset+saltLength]
	offset += saltLength

	// Read nonce
	nonce := ciphertext[offset : offset+nonceLength]
	offset += nonceLength

	// Remaining bytes are the actual ciphertext
	encryptedData := ciphertext[offset:]

	// Derive key from password
	key := GenerateKey(password, salt)
	defer clearBytes(key)

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Decrypt data
	plaintext, err := gcm.Open(nil, nonce, encryptedData, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w (incorrect password or corrupted data)", err)
	}

	return plaintext, nil
}

// EncryptFile encrypts a file and writes it to the output path
func EncryptFile(inputPath, outputPath, password string) error {
	// Read input file
	plaintext, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("failed to read input file: %w", err)
	}

	// Encrypt data
	ciphertext, err := Encrypt(plaintext, password)
	if err != nil {
		return err
	}

	// Write output file
	if err := os.WriteFile(outputPath, ciphertext, 0600); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	return nil
}

// DecryptFile decrypts a file and writes it to the output path
func DecryptFile(inputPath, outputPath, password string) error {
	// Read input file
	ciphertext, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("failed to read input file: %w", err)
	}

	// Decrypt data
	plaintext, err := Decrypt(ciphertext, password)
	if err != nil {
		return err
	}

	// Write output file
	if err := os.WriteFile(outputPath, plaintext, 0600); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	return nil
}

// GetOrCreateEncryptionKey gets or creates an encryption key file
func GetOrCreateEncryptionKey(keyPath, password string) error {
	// Check if key file exists
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		// Generate a random key
		key := make([]byte, 32)
		if _, err := io.ReadFull(rand.Reader, key); err != nil {
			return fmt.Errorf("failed to generate key: %w", err)
		}

		// Encrypt the key with the password
		encryptedKey, err := Encrypt(key, password)
		if err != nil {
			return fmt.Errorf("failed to encrypt key: %w", err)
		}

		// Write to file with restrictive permissions
		if err := os.WriteFile(keyPath, encryptedKey, 0600); err != nil {
			return fmt.Errorf("failed to write key file: %w", err)
		}

		// Clear sensitive data
		clearBytes(key)
	}

	return nil
}

// LoadEncryptionKey loads and decrypts an encryption key file
func LoadEncryptionKey(keyPath, password string) ([]byte, error) {
	// Read key file
	encryptedKey, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read key file: %w", err)
	}

	// Decrypt key
	key, err := Decrypt(encryptedKey, password)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt key: %w", err)
	}

	return key, nil
}

// clearBytes securely clears a byte slice
func clearBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
