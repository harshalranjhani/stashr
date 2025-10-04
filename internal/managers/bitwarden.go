package managers

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/harshalranjhani/stashr/pkg/utils"
)

// Bitwarden represents the Bitwarden password manager
type Bitwarden struct {
	CLIPath string
	Email   string
}

// NewBitwarden creates a new Bitwarden manager instance
func NewBitwarden(cliPath, email string) *Bitwarden {
	return &Bitwarden{
		CLIPath: cliPath,
		Email:   email,
	}
}

// Name returns the name of the password manager
func (b *Bitwarden) Name() string {
	return "bitwarden"
}

// IsInstalled checks if the Bitwarden CLI is installed
func (b *Bitwarden) IsInstalled() bool {
	return utils.IsCommandAvailable(b.CLIPath)
}

// IsAuthenticated checks if the user is authenticated
func (b *Bitwarden) IsAuthenticated() (bool, error) {
	if !b.IsInstalled() {
		return false, &ManagerNotInstalledError{
			Manager: b.Name(),
			CLIPath: b.CLIPath,
		}
	}

	// Run 'bw status' to check authentication status
	output, err := utils.RunCommand(b.CLIPath, "status")
	if err != nil {
		return false, fmt.Errorf("failed to check status: %w", err)
	}

	// Parse JSON output
	var status struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(output, &status); err != nil {
		return false, fmt.Errorf("failed to parse status: %w", err)
	}

	// Check if status is "unlocked"
	if status.Status == "unlocked" {
		return true, nil
	}

	// If locked, return error with helpful message
	if status.Status == "locked" {
		return false, &ManagerNotAuthenticatedError{
			Manager: b.Name(),
			Message: "vault is locked. Please unlock with: bw unlock",
		}
	}

	// If unauthenticated, return error
	return false, &ManagerNotAuthenticatedError{
		Manager: b.Name(),
		Message: "not logged in. Please login with: bw login",
	}
}

// Export exports the Bitwarden vault to the specified file
func (b *Bitwarden) Export(outputPath string) error {
	if !b.IsInstalled() {
		return &ManagerNotInstalledError{
			Manager: b.Name(),
			CLIPath: b.CLIPath,
		}
	}

	// Check authentication
	authenticated, err := b.IsAuthenticated()
	if err != nil {
		return err
	}
	if !authenticated {
		return &ManagerNotAuthenticatedError{
			Manager: b.Name(),
			Message: "not authenticated",
		}
	}

	// Get session token from environment
	sessionToken := os.Getenv("BW_SESSION")

	var cmd *exec.Cmd
	if sessionToken != "" {
		// Use session token
		cmd = exec.Command(b.CLIPath, "export", "--format", "json", "--output", outputPath, "--session", sessionToken)
	} else {
		// Try without session token (user might be unlocked)
		cmd = exec.Command(b.CLIPath, "export", "--format", "json", "--output", outputPath)
	}

	// Run export command
	output, err := cmd.CombinedOutput()
	if err != nil {
		return &ExportError{
			Manager: b.Name(),
			Err:     fmt.Errorf("export failed: %w (output: %s)", err, string(output)),
		}
	}

	// Verify the file was created
	if !utils.FileExists(outputPath) {
		return &ExportError{
			Manager: b.Name(),
			Err:     fmt.Errorf("export file was not created"),
		}
	}

	return nil
}

// GetItemCount returns the number of items in the vault
func (b *Bitwarden) GetItemCount() (int, error) {
	if !b.IsInstalled() {
		return 0, &ManagerNotInstalledError{
			Manager: b.Name(),
			CLIPath: b.CLIPath,
		}
	}

	// Run 'bw list items' to get all items
	output, err := utils.RunCommand(b.CLIPath, "list", "items")
	if err != nil {
		// If command fails, return 0 (we can't get count)
		return 0, nil
	}

	// Parse JSON array
	var items []interface{}
	if err := json.Unmarshal(output, &items); err != nil {
		// If parsing fails, return 0
		return 0, nil
	}

	return len(items), nil
}

// Unlock prompts the user to unlock the vault
func (b *Bitwarden) Unlock() error {
	if !b.IsInstalled() {
		return &ManagerNotInstalledError{
			Manager: b.Name(),
			CLIPath: b.CLIPath,
		}
	}

	fmt.Println("Please unlock your Bitwarden vault:")
	cmd := exec.Command(b.CLIPath, "unlock")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to unlock vault: %w", err)
	}

	return nil
}

// Login prompts the user to login
func (b *Bitwarden) Login() error {
	if !b.IsInstalled() {
		return &ManagerNotInstalledError{
			Manager: b.Name(),
			CLIPath: b.CLIPath,
		}
	}

	fmt.Println("Please login to Bitwarden:")
	var cmd *exec.Cmd
	if b.Email != "" {
		cmd = exec.Command(b.CLIPath, "login", b.Email)
	} else {
		cmd = exec.Command(b.CLIPath, "login")
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to login: %w", err)
	}

	return nil
}

// GetStatus returns the current status of Bitwarden
func (b *Bitwarden) GetStatus() (string, error) {
	if !b.IsInstalled() {
		return "", &ManagerNotInstalledError{
			Manager: b.Name(),
			CLIPath: b.CLIPath,
		}
	}

	output, err := utils.RunCommand(b.CLIPath, "status")
	if err != nil {
		return "", fmt.Errorf("failed to get status: %w", err)
	}

	var status struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(output, &status); err != nil {
		return "", fmt.Errorf("failed to parse status: %w", err)
	}

	return strings.ToUpper(status.Status[:1]) + status.Status[1:], nil
}
