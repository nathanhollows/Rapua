package services_test

import (
	"archive/zip"
	"os"
	"testing"

	"github.com/nathanhollows/Rapua/v6/internal/services"
)

func TestCreateQRCodeImage(t *testing.T) {
	assetGen := services.NewAssetGenerator()

	tests := []struct {
		name      string
		path      string
		content   string
		options   []services.QRCodeOption
		expectErr bool
		cleanupFn func() // Function to clean up generated files if necessary.
	}{
		{
			name:      "Default options",
			path:      "default.png",
			content:   "Hello, QR!",
			expectErr: false,
			cleanupFn: func() { os.Remove("default.png") }, // Cleanup generated file after test
		},
		{
			name:    "Invalid format",
			path:    "invalid_format.qr",
			content: "Hello, QR!",
			options: []services.QRCodeOption{
				assetGen.WithQRFormat("bmp"),
			},
			expectErr: true,
		},
		{
			name:    "Valid PNG format",
			path:    "valid.png",
			content: "Hello, QR-1!",
			options: []services.QRCodeOption{
				assetGen.WithQRFormat("png"),
			},
			expectErr: false,
			cleanupFn: func() { os.Remove("valid.png") },
		},
		{
			name:    "Valid SVG format",
			path:    "valid.svg",
			content: "Hello, QR-2!",
			options: []services.QRCodeOption{
				assetGen.WithQRFormat("svg"),
			},
			expectErr: false,
			cleanupFn: func() { os.Remove("valid.svg") },
		},
		{
			name:    "Custom Colors",
			path:    "colored.svg",
			content: "Hello, QR-3!",
			options: []services.QRCodeOption{
				assetGen.WithQRFormat("svg"),
				assetGen.WithQRForeground("#FF0000"),
				assetGen.WithQRBackground("#00FF00"),
			},
			expectErr: false,
			cleanupFn: func() { os.Remove("colored.svg") },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := assetGen.CreateQRCodeImage(tt.path, tt.content, tt.options...)

			if (err != nil) != tt.expectErr {
				t.Fatalf("Expected error: %v, got: %v", tt.expectErr, err)
			}

			if tt.cleanupFn != nil {
				tt.cleanupFn()
			}
		})
	}
}
func createTestFile(path, content string) error {
	_, err := os.Create(path)
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0600)
}

func cleanupTestFiles(files ...string) {
	for _, file := range files {
		os.Remove(file)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func TestCreateArchive(t *testing.T) {
	assetGen := services.NewAssetGenerator()

	tests := []struct {
		name      string
		files     []string
		expectErr bool
		cleanupFn func()
	}{
		{
			name: "Successful archive creation",
			files: []string{
				"test1.txt",
				"test2.txt",
			},
			expectErr: false,
			cleanupFn: func() {
				cleanupTestFiles("test1.txt", "test2.txt")
			},
		},
		{
			name:      "Non-existent file",
			files:     []string{"non_existent.txt"},
			expectErr: true,
			cleanupFn: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			if tt.name == "Successful archive creation" {
				for _, file := range tt.files {
					if err := createTestFile(file, "content"); err != nil {
						t.Fatalf("Setup failed: %v", err)
					}
				}
			}

			// Execute
			err := os.MkdirAll("assets/codes", 0755)
			if err != nil {
				t.Fatalf("Failed to create assets directory: %v", err)
			}
			archivePath, err := assetGen.CreateArchive(tt.files)

			// Check expectation
			if (err != nil) != tt.expectErr {
				t.Fatalf("Expected error: %v, got: %v", tt.expectErr, err)
			}

			if !tt.expectErr && archivePath == "" {
				t.Fatalf("Expected valid archive path, got empty")
			}

			// Verify archive content if successful
			if !tt.expectErr {
				if archivePath != "" && !fileExists(archivePath) {
					t.Fatalf("Archive file not created: %s", archivePath)
				}

				// Open the zip file for verification
				r, err := zip.OpenReader(archivePath)
				if err != nil {
					t.Fatalf("Failed to open archive: %v", err)
				}
				defer r.Close()

				expectedFiles := make(map[string]bool)
				for _, f := range tt.files {
					expectedFiles[f] = false
				}

				// Check the files in the archive
				for _, f := range r.File {
					if _, ok := expectedFiles[f.Name]; ok {
						expectedFiles[f.Name] = true
					}
				}

				for fileName, found := range expectedFiles {
					if !found {
						t.Errorf("File %s not found in archive", fileName)
					}
				}
			}

			// Cleanup
			os.Remove(archivePath)
			os.RemoveAll("assets")
			if tt.cleanupFn != nil {
				tt.cleanupFn()
			}
		})
	}
}
