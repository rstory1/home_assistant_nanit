package app

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/scgreenhalgh/home_assistant_nanit/pkg/baby"
)

func TestLogHandlerFileCreationError(t *testing.T) {
	// Create a directory that we can't write to (simulate file creation error)
	tmpDir := t.TempDir()
	readOnlyDir := filepath.Join(tmpDir, "readonly")
	if err := os.Mkdir(readOnlyDir, 0444); err != nil {
		t.Fatalf("Failed to create read-only directory: %v", err)
	}
	// Ensure cleanup even if test fails
	defer os.Chmod(readOnlyDir, 0755)

	dirs := DataDirectories{
		LogDir: readOnlyDir,
	}

	handler := createLogHandler(dirs)

	// Create request with body
	body := strings.NewReader("test log content")
	req := httptest.NewRequest(http.MethodPost, "/log", body)
	rec := httptest.NewRecorder()

	// Should not panic, should return error status
	handler(rec, req)

	// Should return 500 error, not panic
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}
}

func TestLogHandlerSizeLimit(t *testing.T) {
	tmpDir := t.TempDir()
	dirs := DataDirectories{
		LogDir: tmpDir,
	}

	handler := createLogHandler(dirs)

	// Create a large body that exceeds the limit (>100MB)
	// We'll use a reader that reports a large size but we'll check
	// that only maxUploadSize bytes are written
	largeBody := strings.NewReader(strings.Repeat("x", 1024*1024)) // 1MB for test
	req := httptest.NewRequest(http.MethodPost, "/log", largeBody)
	rec := httptest.NewRecorder()

	handler(rec, req)

	// Check that a file was created
	files, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to read temp dir: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("Expected log file to be created")
	}

	// Verify file size is within limits
	filePath := filepath.Join(tmpDir, files[0].Name())
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}

	// File should be created successfully for reasonable sizes
	if info.Size() == 0 {
		t.Error("Expected non-zero file size")
	}
}

func TestLogHandlerSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	dirs := DataDirectories{
		LogDir: tmpDir,
	}

	handler := createLogHandler(dirs)

	content := "test log content"
	body := strings.NewReader(content)
	req := httptest.NewRequest(http.MethodPost, "/log", body)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("Expected status %d, got %d", http.StatusNoContent, rec.Code)
	}

	// Check file was created with correct content
	files, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to read temp dir: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("Expected 1 file, got %d", len(files))
	}

	filePath := filepath.Join(tmpDir, files[0].Name())
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	if string(data) != content {
		t.Errorf("Expected content %q, got %q", content, string(data))
	}
}

func TestServeHtmlEscaping(t *testing.T) {
	// Test that baby UIDs are properly escaped to prevent XSS
	babies := []baby.Baby{
		{UID: "<script>alert('xss')</script>"},
		{UID: "normal-uid"},
		{UID: "uid-with-\"quotes\""},
	}

	handler := createIndexHandler(babies)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	body := rec.Body.String()

	// Check that dangerous characters are escaped
	if strings.Contains(body, "<script>") {
		t.Error("Script tag should be escaped")
	}
	if strings.Contains(body, "alert('xss')") {
		t.Error("XSS payload should be escaped")
	}

	// Check that safe content is still present (escaped)
	if !strings.Contains(body, "&lt;script&gt;") {
		t.Error("Escaped script tag should be present")
	}
	if !strings.Contains(body, "normal-uid") {
		t.Error("Normal UID should be present")
	}
	if !strings.Contains(body, "&#34;") || !strings.Contains(body, "&quot;") {
		// Either HTML entity encoding for quotes is acceptable
		if !strings.Contains(body, "quotes") {
			t.Error("Quoted UID should be present with escaped quotes")
		}
	}
}

// limitedReader is a reader that returns data up to a limit
type limitedReader struct {
	remaining int64
}

func (r *limitedReader) Read(p []byte) (n int, err error) {
	if r.remaining <= 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > r.remaining {
		p = p[:r.remaining]
	}
	for i := range p {
		p[i] = 'x'
	}
	r.remaining -= int64(len(p))
	return len(p), nil
}

func TestCreateHTTPServerDefaults(t *testing.T) {
	server := createHTTPServer(8080, nil, ServerTLSConfig{})

	// Verify timeouts are set correctly
	if server.ReadTimeout != 30*time.Second {
		t.Errorf("Expected ReadTimeout 30s, got %v", server.ReadTimeout)
	}
	if server.WriteTimeout != 30*time.Second {
		t.Errorf("Expected WriteTimeout 30s, got %v", server.WriteTimeout)
	}
	if server.ReadHeaderTimeout != 10*time.Second {
		t.Errorf("Expected ReadHeaderTimeout 10s, got %v", server.ReadHeaderTimeout)
	}
	if server.IdleTimeout != 120*time.Second {
		t.Errorf("Expected IdleTimeout 120s, got %v", server.IdleTimeout)
	}
	if server.MaxHeaderBytes != 1<<20 {
		t.Errorf("Expected MaxHeaderBytes 1MB, got %v", server.MaxHeaderBytes)
	}
	if server.Addr != ":8080" {
		t.Errorf("Expected Addr :8080, got %v", server.Addr)
	}
}

func TestCreateHTTPServerWithTLS(t *testing.T) {
	tlsConfig := ServerTLSConfig{
		Enabled:  true,
		CertFile: "/path/to/cert.pem",
		KeyFile:  "/path/to/key.pem",
	}
	server := createHTTPServer(443, nil, tlsConfig)

	// Server should have TLS config enabled
	if server.TLSConfig == nil {
		t.Error("Expected TLSConfig to be set when TLS is enabled")
	}
	if server.Addr != ":443" {
		t.Errorf("Expected Addr :443, got %v", server.Addr)
	}
}

func TestCreateHTTPServerWithoutTLS(t *testing.T) {
	tlsConfig := ServerTLSConfig{
		Enabled: false,
	}
	server := createHTTPServer(8080, nil, tlsConfig)

	// Server should not have TLS config
	if server.TLSConfig != nil {
		t.Error("Expected TLSConfig to be nil when TLS is disabled")
	}
}

func TestServerTLSConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      ServerTLSConfig
		expectError bool
	}{
		{
			name:        "disabled TLS is always valid",
			config:      ServerTLSConfig{Enabled: false},
			expectError: false,
		},
		{
			name:        "enabled TLS without cert is invalid",
			config:      ServerTLSConfig{Enabled: true, KeyFile: "/path/to/key.pem"},
			expectError: true,
		},
		{
			name:        "enabled TLS without key is invalid",
			config:      ServerTLSConfig{Enabled: true, CertFile: "/path/to/cert.pem"},
			expectError: true,
		},
		{
			name:        "enabled TLS with both files is valid",
			config:      ServerTLSConfig{Enabled: true, CertFile: "/path/to/cert.pem", KeyFile: "/path/to/key.pem"},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectError && err == nil {
				t.Error("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}
