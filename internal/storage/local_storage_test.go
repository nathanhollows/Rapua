package storage_test

import (
	"context"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nathanhollows/Rapua/v5/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockFile struct {
	reader *strings.Reader
}

// mockMultipartFile creates a mock multipart.File for testing.
func mockMultipartFile(content string) multipart.File {
	return &mockFile{reader: strings.NewReader(content)}
}

func (m *mockFile) Read(p []byte) (n int, err error) {
	return m.reader.Read(p)
}

func (m *mockFile) ReadAt(p []byte, off int64) (n int, err error) {
	return m.reader.ReadAt(p, off)
}

func (m *mockFile) Seek(offset int64, whence int) (int64, error) {
	return m.reader.Seek(offset, whence)
}

func (m *mockFile) Close() error {
	return nil
}

func TestLocalStorage_Upload(t *testing.T) {
	basePath := "./test_uploads"
	_ = os.RemoveAll(basePath) // Clean up before test
	storage := storage.NewLocalStorage(basePath)

	tests := []struct {
		name      string
		file      multipart.File
		filename  string
		expectErr bool
	}{
		{
			name:      "Valid file upload",
			file:      mockMultipartFile("test content"),
			filename:  "test.txt",
			expectErr: false,
		},
		{
			name:      "Invalid file name",
			file:      mockMultipartFile("test content"),
			filename:  "../../invalid.txt",
			expectErr: false, // Should be cleaned automatically
		},
		{
			name:      "Empty file",
			file:      mockMultipartFile(""),
			filename:  "empty.txt",
			expectErr: false,
		},
		{
			name:      "File already exists",
			file:      mockMultipartFile("duplicate content"),
			filename:  "duplicate.txt",
			expectErr: false, // Duplicate files are allowed and should be renamed
		},
		{
			name:      "File with special characters",
			file:      mockMultipartFile("special chars"),
			filename:  "sp#ec%ial&.txt",
			expectErr: false,
		},
		{
			name:      "Context cancellation",
			file:      mockMultipartFile("cancel test"),
			filename:  "cancel.txt",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
			defer cancel()
			if tt.name == "Context cancellation" {
				cancel()
			}

			filePaths, _, err := storage.Upload(ctx, tt.file, tt.filename)

			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.NotEmpty(t, filePaths)

				for _, path := range filePaths {
					_, statErr := os.Stat("./" + filepath.Clean(path))
					require.NoError(t, statErr)
				}
			}
		})
	}

	_ = os.RemoveAll(basePath) // Clean up after test
}

func TestLocalStorage_Concurrency(t *testing.T) {
	basePath := "./test_uploads"
	_ = os.RemoveAll(basePath)
	storage := storage.NewLocalStorage(basePath)

	var wg sync.WaitGroup
	for i := range 10 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _, err := storage.Upload(context.Background(), mockMultipartFile("content"), "concurrent.txt")
			assert.NoError(t, err)
		}(i)
	}
	wg.Wait()
	_ = os.RemoveAll(basePath)
}

func TestLocalStorage_Type(t *testing.T) {
	storage := storage.NewLocalStorage("./test_uploads")
	assert.Equal(t, "local", storage.Type())
}
