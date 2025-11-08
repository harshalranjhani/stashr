package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"

	"github.com/harshalranjhani/stashr/pkg/utils"
)

// GoogleDrive represents a Google Drive storage backend
type GoogleDrive struct {
	CredentialsPath string
	FolderID        string
	service         *drive.Service
}

// NewGoogleDrive creates a new Google Drive storage backend
func NewGoogleDrive(credentialsPath, folderID string) *GoogleDrive {
	return &GoogleDrive{
		CredentialsPath: credentialsPath,
		FolderID:        folderID,
	}
}

// Name returns the name of the storage backend
func (g *GoogleDrive) Name() string {
	return "Google Drive"
}

// IsAvailable checks if Google Drive is available (credentials exist and valid)
func (g *GoogleDrive) IsAvailable() (bool, error) {
	// Check if credentials file exists
	if !utils.FileExists(g.CredentialsPath) {
		return false, &StorageUnavailableError{
			Storage: g.Name(),
			Reason:  fmt.Sprintf("credentials file not found at %s", g.CredentialsPath),
		}
	}

	// Try to initialize the service
	if err := g.initService(); err != nil {
		return false, &StorageUnavailableError{
			Storage: g.Name(),
			Reason:  fmt.Sprintf("failed to initialize service: %v", err),
		}
	}

	return true, nil
}

// initService initializes the Google Drive service
func (g *GoogleDrive) initService() error {
	if g.service != nil {
		return nil // Already initialized
	}

	ctx := context.Background()

	// Read credentials file
	credData, err := os.ReadFile(g.CredentialsPath)
	if err != nil {
		return fmt.Errorf("failed to read credentials file: %w", err)
	}

	// Parse credentials
	config, err := google.ConfigFromJSON(credData, drive.DriveFileScope)
	if err != nil {
		return fmt.Errorf("failed to parse credentials: %w", err)
	}

	// Get token file path
	tokenPath := g.getTokenPath()

	// Get client
	client, err := g.getClient(ctx, config, tokenPath)
	if err != nil {
		return fmt.Errorf("failed to get client: %w", err)
	}

	// Create Drive service
	service, err := drive.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return fmt.Errorf("failed to create Drive service: %w", err)
	}

	g.service = service
	return nil
}

// getTokenPath returns the path to the token file
func (g *GoogleDrive) getTokenPath() string {
	dir := filepath.Dir(g.CredentialsPath)
	return filepath.Join(dir, "gdrive-token.json")
}

// getClient retrieves an OAuth2 client
func (g *GoogleDrive) getClient(ctx context.Context, config *oauth2.Config, tokenPath string) (*http.Client, error) {
	// Try to load token from file
	token, err := g.loadToken(tokenPath)
	if err != nil {
		// If token doesn't exist or is invalid, get a new one
		token, err = g.getTokenFromWeb(ctx, config)
		if err != nil {
			return nil, fmt.Errorf("failed to get token: %w", err)
		}
		// Save token for future use
		if err := g.saveToken(tokenPath, token); err != nil {
			return nil, fmt.Errorf("failed to save token: %w", err)
		}
	}

	return config.Client(ctx, token), nil
}

// loadToken loads a token from a file
func (g *GoogleDrive) loadToken(path string) (*oauth2.Token, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	token := &oauth2.Token{}
	if err := json.NewDecoder(file).Decode(token); err != nil {
		return nil, err
	}

	return token, nil
}

// saveToken saves a token to a file
func (g *GoogleDrive) saveToken(path string, token *oauth2.Token) error {
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer file.Close()

	return json.NewEncoder(file).Encode(token)
}

// getTokenFromWeb requests a token from the web
func (g *GoogleDrive) getTokenFromWeb(ctx context.Context, config *oauth2.Config) (*oauth2.Token, error) {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser:\n%v\n\n", authURL)
	fmt.Print("Enter authorization code: ")

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		return nil, fmt.Errorf("failed to read authorization code: %w", err)
	}

	token, err := config.Exchange(ctx, authCode)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange authorization code: %w", err)
	}

	return token, nil
}

// Upload uploads a file to Google Drive
func (g *GoogleDrive) Upload(filename string, data []byte) error {
	if err := g.initService(); err != nil {
		return &UploadError{
			Storage: g.Name(),
			File:    filename,
			Err:     err,
		}
	}

	// Create file metadata
	file := &drive.File{
		Name: filename,
	}

	// If folder ID is specified, set parent
	if g.FolderID != "" {
		file.Parents = []string{g.FolderID}
	}

	// Create file reader
	reader := strings.NewReader(string(data))

	// Upload file
	_, err := g.service.Files.Create(file).Media(reader).Do()
	if err != nil {
		return &UploadError{
			Storage: g.Name(),
			File:    filename,
			Err:     fmt.Errorf("failed to upload file: %w", err),
		}
	}

	return nil
}

// Download downloads a file from Google Drive
func (g *GoogleDrive) Download(filename string) ([]byte, error) {
	if err := g.initService(); err != nil {
		return nil, &DownloadError{
			Storage: g.Name(),
			File:    filename,
			Err:     err,
		}
	}

	// Find file by name
	query := fmt.Sprintf("name='%s' and trashed=false", filename)
	if g.FolderID != "" {
		query += fmt.Sprintf(" and '%s' in parents", g.FolderID)
	}

	fileList, err := g.service.Files.List().Q(query).Fields("files(id)").Do()
	if err != nil {
		return nil, &DownloadError{
			Storage: g.Name(),
			File:    filename,
			Err:     fmt.Errorf("failed to list files: %w", err),
		}
	}

	if len(fileList.Files) == 0 {
		return nil, &DownloadError{
			Storage: g.Name(),
			File:    filename,
			Err:     fmt.Errorf("file not found"),
		}
	}

	// Get file content
	response, err := g.service.Files.Get(fileList.Files[0].Id).Download()
	if err != nil {
		return nil, &DownloadError{
			Storage: g.Name(),
			File:    filename,
			Err:     fmt.Errorf("failed to download file: %w", err),
		}
	}
	defer response.Body.Close()

	data, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, &DownloadError{
			Storage: g.Name(),
			File:    filename,
			Err:     fmt.Errorf("failed to read file content: %w", err),
		}
	}

	return data, nil
}

// List lists all backup files in Google Drive
func (g *GoogleDrive) List() ([]BackupFile, error) {
	if err := g.initService(); err != nil {
		return nil, err
	}

	// Build query
	query := "trashed=false"
	if g.FolderID != "" {
		query += fmt.Sprintf(" and '%s' in parents", g.FolderID)
	}

	// List files
	fileList, err := g.service.Files.List().
		Q(query).
		Fields("files(id, name, size, modifiedTime)").
		OrderBy("modifiedTime desc").
		Do()
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	var backups []BackupFile
	for _, file := range fileList.Files {
		// Skip hidden/system files (e.g., ._ files, .DS_Store)
		if shouldIgnoreFile(file.Name) {
			continue
		}

		modTime, _ := time.Parse(time.RFC3339, file.ModifiedTime)
		backups = append(backups, BackupFile{
			Name:         file.Name,
			Size:         file.Size,
			ModifiedTime: modTime,
			Location:     file.Id,
			StorageType:  g.Name(),
		})
	}

	return backups, nil
}

// Delete deletes a file from Google Drive
func (g *GoogleDrive) Delete(filename string) error {
	if err := g.initService(); err != nil {
		return err
	}

	// Find file by name
	query := fmt.Sprintf("name='%s' and trashed=false", filename)
	if g.FolderID != "" {
		query += fmt.Sprintf(" and '%s' in parents", g.FolderID)
	}

	fileList, err := g.service.Files.List().Q(query).Fields("files(id)").Do()
	if err != nil {
		return fmt.Errorf("failed to list files: %w", err)
	}

	if len(fileList.Files) == 0 {
		return fmt.Errorf("file not found")
	}

	// Delete file
	if err := g.service.Files.Delete(fileList.Files[0].Id).Do(); err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

// CreateBackupFolder creates a dedicated backup folder in Google Drive
func (g *GoogleDrive) CreateBackupFolder(folderName string) (string, error) {
	if err := g.initService(); err != nil {
		return "", err
	}

	// Create folder metadata
	folder := &drive.File{
		Name:     folderName,
		MimeType: "application/vnd.google-apps.folder",
	}

	// Create folder
	createdFolder, err := g.service.Files.Create(folder).Fields("id").Do()
	if err != nil {
		return "", fmt.Errorf("failed to create folder: %w", err)
	}

	return createdFolder.Id, nil
}

// GetFolderInfo returns information about the backup folder
func (g *GoogleDrive) GetFolderInfo() (*drive.File, error) {
	if err := g.initService(); err != nil {
		return nil, err
	}

	if g.FolderID == "" {
		return nil, fmt.Errorf("folder ID not set")
	}

	folder, err := g.service.Files.Get(g.FolderID).Fields("id, name, createdTime").Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get folder info: %w", err)
	}

	return folder, nil
}

// CleanOldBackups applies retention policy and deletes old backups
func (g *GoogleDrive) CleanOldBackups(keepLast int) error {
	backups, err := g.List()
	if err != nil {
		return err
	}

	return ApplyRetentionPolicy(backups, keepLast, g.Delete)
}

// TestConnection tests the connection to Google Drive
func (g *GoogleDrive) TestConnection() error {
	if err := g.initService(); err != nil {
		return err
	}

	// Try to get user info
	about, err := g.service.About.Get().Fields("user").Do()
	if err != nil {
		return fmt.Errorf("failed to get user info: %w", err)
	}

	if about.User == nil {
		return fmt.Errorf("user info not available")
	}

	return nil
}
