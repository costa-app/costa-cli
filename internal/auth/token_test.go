package auth

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/zalando/go-keyring"
)

func TestTokenDataIsExpiredWithSkew(t *testing.T) {
	tests := []struct {
		token    *TokenData
		name     string
		skew     time.Duration
		expected bool
	}{
		{
			name:     "nil token",
			token:    nil,
			skew:     5 * time.Minute,
			expected: false,
		},
		{
			name: "no expiry",
			token: &TokenData{
				AccessToken: "test",
				ExpiresAt:   nil,
			},
			skew:     5 * time.Minute,
			expected: false,
		},
		{
			name: "expired",
			token: &TokenData{
				AccessToken: "test",
				ExpiresAt:   ptrTime(time.Now().Add(-1 * time.Hour)),
			},
			skew:     5 * time.Minute,
			expected: true,
		},
		{
			name: "within skew window",
			token: &TokenData{
				AccessToken: "test",
				ExpiresAt:   ptrTime(time.Now().Add(3 * time.Minute)),
			},
			skew:     5 * time.Minute,
			expected: true,
		},
		{
			name: "valid and beyond skew",
			token: &TokenData{
				AccessToken: "test",
				ExpiresAt:   ptrTime(time.Now().Add(10 * time.Minute)),
			},
			skew:     5 * time.Minute,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.token.IsExpiredWithSkew(tt.skew)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestTokenDataIsValid(t *testing.T) {
	tests := []struct {
		token    *TokenData
		name     string
		expected bool
	}{
		{
			name:     "nil token",
			token:    nil,
			expected: false,
		},
		{
			name: "empty access token",
			token: &TokenData{
				AccessToken: "",
			},
			expected: false,
		},
		{
			name: "valid token no expiry",
			token: &TokenData{
				AccessToken: "test-token",
				ExpiresAt:   nil,
			},
			expected: true,
		},
		{
			name: "valid token with future expiry",
			token: &TokenData{
				AccessToken: "test-token",
				ExpiresAt:   ptrTime(time.Now().Add(1 * time.Hour)),
			},
			expected: true,
		},
		{
			name: "expired token",
			token: &TokenData{
				AccessToken: "test-token",
				ExpiresAt:   ptrTime(time.Now().Add(-1 * time.Hour)),
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.token.IsValid()
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestSaveAndLoadTokenWithKeyring(t *testing.T) {
	// Use keyring mock for testing
	keyring.MockInit()

	// Setup test HOME so config dir is isolated
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tmpDir)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	// Ensure we're using keyring
	useKeyring = true

	// Create test token
	expiresAt := time.Now().Add(1 * time.Hour)
	token := &Token{
		OAuth: &TokenData{
			AccessToken:  "oauth-access-123",
			RefreshToken: "oauth-refresh-456",
			TokenType:    "Bearer",
			ExpiresAt:    &expiresAt,
		},
		Coding: &TokenData{
			AccessToken: "coding-access-789",
			TokenType:   "Bearer",
			ExpiresAt:   &expiresAt,
		},
	}

	// Save token
	err := SaveToken(token)
	if err != nil {
		t.Fatalf("Failed to save token: %v", err)
	}

	// Verify metadata file exists
	metadataPath, _ := GetMetadataPath()
	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		t.Fatalf("Metadata file was not created")
	}

	// Load token
	loaded, err := LoadToken()
	if err != nil {
		t.Fatalf("Failed to load token: %v", err)
	}

	// Verify OAuth token
	if loaded.OAuth == nil {
		t.Fatal("OAuth token is nil")
	}
	if loaded.OAuth.AccessToken != token.OAuth.AccessToken {
		t.Errorf("OAuth access token mismatch: got %s, want %s", loaded.OAuth.AccessToken, token.OAuth.AccessToken)
	}
	if loaded.OAuth.RefreshToken != token.OAuth.RefreshToken {
		t.Errorf("OAuth refresh token mismatch: got %s, want %s", loaded.OAuth.RefreshToken, token.OAuth.RefreshToken)
	}
	if loaded.OAuth.TokenType != token.OAuth.TokenType {
		t.Errorf("OAuth token type mismatch: got %s, want %s", loaded.OAuth.TokenType, token.OAuth.TokenType)
	}

	// Verify Coding token
	if loaded.Coding == nil {
		t.Fatal("Coding token is nil")
	}
	if loaded.Coding.AccessToken != token.Coding.AccessToken {
		t.Errorf("Coding access token mismatch: got %s, want %s", loaded.Coding.AccessToken, token.Coding.AccessToken)
	}

	// Test deletion
	err = DeleteToken()
	if err != nil {
		t.Fatalf("Failed to delete token: %v", err)
	}

	// Verify metadata file is deleted
	if _, err := os.Stat(metadataPath); !os.IsNotExist(err) {
		t.Error("Metadata file was not deleted")
	}
}

func TestSaveAndLoadTokenWithFileFallback(t *testing.T) {
	// Disable keyring for this test
	useKeyring = false
	defer func() { useKeyring = true }()

	// Setup test HOME so config dir is isolated
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tmpDir)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	// Create test token
	expiresAt := time.Now().Add(1 * time.Hour)
	token := &Token{
		OAuth: &TokenData{
			AccessToken:  "oauth-access-file",
			RefreshToken: "oauth-refresh-file",
			TokenType:    "Bearer",
			ExpiresAt:    &expiresAt,
		},
		Coding: &TokenData{
			AccessToken: "coding-access-file",
			TokenType:   "Bearer",
			ExpiresAt:   &expiresAt,
		},
	}

	// Save token
	err := SaveToken(token)
	if err != nil {
		t.Fatalf("Failed to save token to file: %v", err)
	}

	// Verify token file exists
	tokenPath, _ := GetTokenPath()
	if _, err := os.Stat(tokenPath); os.IsNotExist(err) {
		t.Fatalf("Token file was not created")
	}

	// Load token
	loaded, err := LoadToken()
	if err != nil {
		t.Fatalf("Failed to load token from file: %v", err)
	}

	// Verify tokens
	if loaded.OAuth.AccessToken != token.OAuth.AccessToken {
		t.Errorf("OAuth access token mismatch: got %s, want %s", loaded.OAuth.AccessToken, token.OAuth.AccessToken)
	}
	if loaded.Coding.AccessToken != token.Coding.AccessToken {
		t.Errorf("Coding access token mismatch: got %s, want %s", loaded.Coding.AccessToken, token.Coding.AccessToken)
	}
}

func TestIsLoggedIn(t *testing.T) {
	// Use keyring mock for testing
	keyring.MockInit()

	// Setup test HOME so config dir is isolated
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tmpDir)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	// Ensure we're using keyring
	useKeyring = true

	// Initially should not be logged in
	if IsLoggedIn() {
		t.Error("Expected IsLoggedIn to return false initially")
	}

	// Save a token
	token := &Token{
		OAuth: &TokenData{
			AccessToken: "test",
			TokenType:   "Bearer",
		},
	}
	err := SaveToken(token)
	if err != nil {
		t.Fatalf("Failed to save token: %v", err)
	}

	// Now should be logged in
	if !IsLoggedIn() {
		t.Error("Expected IsLoggedIn to return true after saving token")
	}

	// Delete token
	err = DeleteToken()
	if err != nil {
		t.Fatalf("Failed to delete token: %v", err)
	}

	// Should not be logged in anymore
	if IsLoggedIn() {
		t.Error("Expected IsLoggedIn to return false after deleting token")
	}
}

func TestGetConfigDir(t *testing.T) {
	configDir, err := GetConfigDir()
	if err != nil {
		t.Fatalf("GetConfigDir failed: %v", err)
	}

	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".config", "costa")
	if configDir != expected {
		t.Errorf("Expected config dir %s, got %s", expected, configDir)
	}
}

// Helper function
func ptrTime(t time.Time) *time.Time {
	return &t
}
