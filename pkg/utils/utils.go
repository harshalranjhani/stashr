package utils

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"golang.org/x/term"
)

// CompressData compresses data using gzip
func CompressData(data []byte) ([]byte, error) {
	var compressed []byte
	buf := &writableBuffer{buf: &compressed}

	gzWriter := gzip.NewWriter(buf)
	if _, err := gzWriter.Write(data); err != nil {
		return nil, fmt.Errorf("failed to compress data: %w", err)
	}

	if err := gzWriter.Close(); err != nil {
		return nil, fmt.Errorf("failed to close gzip writer: %w", err)
	}

	return compressed, nil
}

// DecompressData decompresses gzip-compressed data
func DecompressData(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(&readableBuffer{buf: data})
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer reader.Close()

	decompressed, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress data: %w", err)
	}

	return decompressed, nil
}

// writableBuffer is a buffer that implements io.Writer
type writableBuffer struct {
	buf *[]byte
}

func (w *writableBuffer) Write(p []byte) (int, error) {
	*w.buf = append(*w.buf, p...)
	return len(p), nil
}

// readableBuffer is a buffer that implements io.Reader
type readableBuffer struct {
	buf []byte
	pos int
}

func (r *readableBuffer) Read(p []byte) (int, error) {
	if r.pos >= len(r.buf) {
		return 0, io.EOF
	}
	n := copy(p, r.buf[r.pos:])
	r.pos += n
	return n, nil
}

// FormatBytes formats bytes as human-readable size
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// GenerateBackupFilename generates a backup filename based on the format
func GenerateBackupFilename(format, manager string) string {
	timestamp := time.Now().Format("20060102_150405")
	return fmt.Sprintf(format, manager, timestamp)
}

// CommandExists checks if a command exists in PATH
func CommandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

// IsCommandAvailable checks if a command is available and executable
func IsCommandAvailable(path string) bool {
	if path == "" {
		return false
	}

	// If path is absolute, check if it exists
	if filepath.IsAbs(path) {
		info, err := os.Stat(path)
		if err != nil {
			return false
		}
		// Check if it's executable
		return info.Mode()&0111 != 0
	}

	// Otherwise, check if it's in PATH
	_, err := exec.LookPath(path)
	return err == nil
}

// RunCommand runs a command and returns its output
func RunCommand(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("command failed: %w (output: %s)", err, string(output))
	}
	return output, nil
}

// RunCommandWithEnv runs a command with environment variables and returns its output
func RunCommandWithEnv(name string, env []string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Env = append(os.Environ(), env...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("command failed: %w (output: %s)", err, string(output))
	}
	return output, nil
}

// FileExists checks if a file exists
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// DirExists checks if a directory exists
func DirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// CreateDirIfNotExists creates a directory if it doesn't exist
func CreateDirIfNotExists(path string, perm os.FileMode) error {
	if !DirExists(path) {
		if err := os.MkdirAll(path, perm); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}
	return nil
}

// GetTempFile creates a temporary file and returns its path
func GetTempFile(prefix string) (*os.File, error) {
	tmpFile, err := os.CreateTemp("", prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	return tmpFile, nil
}

// CleanupTempFile removes a temporary file
func CleanupTempFile(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to cleanup temp file: %w", err)
	}
	return nil
}

// ConfirmPrompt prompts the user for confirmation
func ConfirmPrompt(message string) bool {
	fmt.Printf("%s (y/n): ", message)
	var response string
	fmt.Scanln(&response)
	return response == "y" || response == "Y" || response == "yes" || response == "Yes"
}

// PromptForInput prompts the user for input
func PromptForInput(message string) string {
	fmt.Printf("%s: ", message)
	var input string
	fmt.Scanln(&input)
	return input
}

// PromptForPassword prompts the user for a password (without echo)
func PromptForPassword(message string) (string, error) {
	if message != "" {
		fmt.Print(message)
	}

	// Read password without echoing to terminal
	bytepw, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println() // Print newline after password input

	if err != nil {
		return "", fmt.Errorf("failed to read password: %w", err)
	}

	return string(bytepw), nil
}
