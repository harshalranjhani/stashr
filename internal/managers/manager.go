package managers

import (
	"fmt"
)

// Manager represents a password manager interface
type Manager interface {
	// Name returns the name of the password manager
	Name() string

	// IsInstalled checks if the password manager CLI is installed
	IsInstalled() bool

	// IsAuthenticated checks if the user is authenticated
	IsAuthenticated() (bool, error)

	// Export exports the vault data to the specified file
	Export(outputPath string) error

	// GetItemCount returns the number of items in the vault (if available)
	GetItemCount() (int, error)
}

// ManagerNotAuthenticatedError indicates the user is not authenticated
type ManagerNotAuthenticatedError struct {
	Manager string
	Message string
}

func (e *ManagerNotAuthenticatedError) Error() string {
	return fmt.Sprintf("%s: %s", e.Manager, e.Message)
}

// ManagerNotInstalledError indicates the manager CLI is not installed
type ManagerNotInstalledError struct {
	Manager string
	CLIPath string
}

func (e *ManagerNotInstalledError) Error() string {
	return fmt.Sprintf("%s CLI not found at %s", e.Manager, e.CLIPath)
}

// ExportError indicates an error during export
type ExportError struct {
	Manager string
	Err     error
}

func (e *ExportError) Error() string {
	return fmt.Sprintf("%s export failed: %v", e.Manager, e.Err)
}

func (e *ExportError) Unwrap() error {
	return e.Err
}
