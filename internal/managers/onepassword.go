package managers

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/harshalranjhani/credstash/pkg/utils"
)

// OnePassword represents the 1Password password manager
type OnePassword struct {
	CLIPath string
	Account string
}

// NewOnePassword creates a new 1Password manager instance
func NewOnePassword(cliPath, account string) *OnePassword {
	return &OnePassword{
		CLIPath: cliPath,
		Account: account,
	}
}

// Name returns the name of the password manager
func (o *OnePassword) Name() string {
	return "1password"
}

// IsInstalled checks if the 1Password CLI is installed
func (o *OnePassword) IsInstalled() bool {
	return utils.IsCommandAvailable(o.CLIPath)
}

// IsAuthenticated checks if the user is authenticated
func (o *OnePassword) IsAuthenticated() (bool, error) {
	if !o.IsInstalled() {
		return false, &ManagerNotInstalledError{
			Manager: o.Name(),
			CLIPath: o.CLIPath,
		}
	}

	// Run 'op whoami' to check authentication
	var cmd *exec.Cmd
	if o.Account != "" {
		cmd = exec.Command(o.CLIPath, "whoami", "--account", o.Account)
	} else {
		cmd = exec.Command(o.CLIPath, "whoami")
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		// If whoami fails, user is not signed in
		return false, &ManagerNotAuthenticatedError{
			Manager: o.Name(),
			Message: fmt.Sprintf("not signed in. Please sign in with: op signin (output: %s)", string(output)),
		}
	}

	return true, nil
}

// Export exports all 1Password vaults to the specified file (metadata only)
func (o *OnePassword) Export(outputPath string) error {
	return o.exportItems(outputPath, false, nil)
}

// ExportFull exports all 1Password vaults with full details including passwords
func (o *OnePassword) ExportFull(outputPath string, progressCallback func(current, total int, itemTitle string)) error {
	return o.exportItems(outputPath, true, progressCallback)
}

// exportItems is the internal export function that handles both metadata and full exports
func (o *OnePassword) exportItems(outputPath string, fullExport bool, progressCallback func(current, total int, itemTitle string)) error {
	if !o.IsInstalled() {
		return &ManagerNotInstalledError{
			Manager: o.Name(),
			CLIPath: o.CLIPath,
		}
	}

	// Check authentication
	authenticated, err := o.IsAuthenticated()
	if err != nil {
		return err
	}
	if !authenticated {
		return &ManagerNotAuthenticatedError{
			Manager: o.Name(),
			Message: "not authenticated",
		}
	}

	// Get all vaults
	vaults, err := o.listVaults()
	if err != nil {
		return &ExportError{
			Manager: o.Name(),
			Err:     fmt.Errorf("failed to list vaults: %w", err),
		}
	}

	if len(vaults) == 0 {
		return &ExportError{
			Manager: o.Name(),
			Err:     fmt.Errorf("no vaults found"),
		}
	}

	var allItems []map[string]interface{}

	if fullExport {
		// Full export: Get complete details for each item (including passwords)
		for _, vault := range vaults {
			items, err := o.listItemsInVault(vault.ID)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to list items in vault %s: %v\n", vault.Name, err)
				continue
			}

			// Get full details for each item
			totalItems := len(items)
			for idx, item := range items {
				itemID, ok := item["id"].(string)
				if !ok {
					continue
				}

				// Get full item details including password fields
				fullItem, err := o.getItemDetails(itemID)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to get details for item %s: %v\n", itemID, err)
					continue
				}

				allItems = append(allItems, fullItem)

				// Call progress callback if provided
				if progressCallback != nil {
					title, _ := fullItem["title"].(string)
					progressCallback(idx+1, totalItems, title)
				}
			}
		}
	} else {
		// Quick export: Just metadata (current behavior)
		for _, vault := range vaults {
			items, err := o.listItemsInVault(vault.ID)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to export vault %s: %v\n", vault.Name, err)
				continue
			}
			allItems = append(allItems, items...)
		}
	}

	// Marshal to JSON
	jsonData, err := json.MarshalIndent(allItems, "", "  ")
	if err != nil {
		return &ExportError{
			Manager: o.Name(),
			Err:     fmt.Errorf("failed to marshal items: %w", err),
		}
	}

	// Write to file
	if err := os.WriteFile(outputPath, jsonData, 0600); err != nil {
		return &ExportError{
			Manager: o.Name(),
			Err:     fmt.Errorf("failed to write export file: %w", err),
		}
	}

	return nil
}

// GetItemCount returns the total number of items across all vaults
func (o *OnePassword) GetItemCount() (int, error) {
	if !o.IsInstalled() {
		return 0, &ManagerNotInstalledError{
			Manager: o.Name(),
			CLIPath: o.CLIPath,
		}
	}

	// Get all vaults
	vaults, err := o.listVaults()
	if err != nil {
		return 0, err
	}

	totalCount := 0
	for _, vault := range vaults {
		items, err := o.listItemsInVault(vault.ID)
		if err != nil {
			continue
		}
		totalCount += len(items)
	}

	return totalCount, nil
}

// Vault represents a 1Password vault
type Vault struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// listVaults lists all available vaults
func (o *OnePassword) listVaults() ([]Vault, error) {
	var cmd *exec.Cmd
	if o.Account != "" {
		cmd = exec.Command(o.CLIPath, "vault", "list", "--format", "json", "--account", o.Account)
	} else {
		cmd = exec.Command(o.CLIPath, "vault", "list", "--format", "json")
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to list vaults: %w (output: %s)", err, string(output))
	}

	var vaults []Vault
	if err := json.Unmarshal(output, &vaults); err != nil {
		return nil, fmt.Errorf("failed to parse vaults: %w", err)
	}

	return vaults, nil
}

// listItemsInVault lists all items in a specific vault
func (o *OnePassword) listItemsInVault(vaultID string) ([]map[string]interface{}, error) {
	var cmd *exec.Cmd
	if o.Account != "" {
		cmd = exec.Command(o.CLIPath, "item", "list", "--vault", vaultID, "--format", "json", "--account", o.Account)
	} else {
		cmd = exec.Command(o.CLIPath, "item", "list", "--vault", vaultID, "--format", "json")
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to list items: %w (output: %s)", err, string(output))
	}

	var items []map[string]interface{}
	if err := json.Unmarshal(output, &items); err != nil {
		return nil, fmt.Errorf("failed to parse items: %w", err)
	}

	return items, nil
}

// getItemDetails gets full details for a specific item including passwords and sensitive fields
func (o *OnePassword) getItemDetails(itemID string) (map[string]interface{}, error) {
	var cmd *exec.Cmd
	if o.Account != "" {
		cmd = exec.Command(o.CLIPath, "item", "get", itemID, "--format", "json", "--account", o.Account)
	} else {
		cmd = exec.Command(o.CLIPath, "item", "get", itemID, "--format", "json")
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to get item: %w (output: %s)", err, string(output))
	}

	var item map[string]interface{}
	if err := json.Unmarshal(output, &item); err != nil {
		return nil, fmt.Errorf("failed to parse item: %w", err)
	}

	return item, nil
}

// SignIn prompts the user to sign in
func (o *OnePassword) SignIn() error {
	if !o.IsInstalled() {
		return &ManagerNotInstalledError{
			Manager: o.Name(),
			CLIPath: o.CLIPath,
		}
	}

	fmt.Println("Please sign in to 1Password:")
	var cmd *exec.Cmd
	if o.Account != "" {
		cmd = exec.Command(o.CLIPath, "signin", "--account", o.Account)
	} else {
		cmd = exec.Command(o.CLIPath, "signin")
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to sign in: %w", err)
	}

	return nil
}

// GetUserInfo returns information about the signed-in user
func (o *OnePassword) GetUserInfo() (string, error) {
	if !o.IsInstalled() {
		return "", &ManagerNotInstalledError{
			Manager: o.Name(),
			CLIPath: o.CLIPath,
		}
	}

	var cmd *exec.Cmd
	if o.Account != "" {
		cmd = exec.Command(o.CLIPath, "whoami", "--account", o.Account)
	} else {
		cmd = exec.Command(o.CLIPath, "whoami")
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get user info: %w", err)
	}

	return string(output), nil
}
