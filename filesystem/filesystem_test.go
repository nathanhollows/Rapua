package filesystem_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi"
	"github.com/nathanhollows/Rapua/v4/filesystem"
)

func TestOpenFile(t *testing.T) {
	dir := "./" // Tests only have access to the folder they are in
	filename := "test.txt"
	err := os.WriteFile(filepath.Join(dir, filename), []byte("hello world"), 0600)
	if err != nil {
		t.Fatalf("could not create test file: %v", err)
	}
	defer os.Remove(filepath.Join(dir, filename))

	myfs := filesystem.Myfs{Dir: http.Dir(dir)}

	// Test: try to open the file
	f, err := myfs.Open(filename)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	defer f.Close()

	// Check file stats
	fi, err := f.Stat()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if fi.Name() != filename {
		t.Errorf("Expected %s, got %s", filename, fi.Name())
	}
}

func TestOpenDirectory(t *testing.T) {
	dir := "./" // Tests only have access to the folder they are in
	myfs := filesystem.Myfs{Dir: http.Dir(dir)}

	// Test: try to open the directory
	f, err := myfs.Open("/")
	if err == nil {
		t.Fatalf("Expected error, got nil")
	}
	if f != nil {
		t.Errorf("Expected nil file handle, got %v", f)
	}
}

func TestServerInvalidPath(t *testing.T) {
	r := chi.NewRouter()
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Expected no panic, got %v", r)
		}
	}()
	filesystem.FileServer(r, "/invalid", filesystem.Myfs{Dir: http.Dir("/invalid")})
}

func TestFileServer(t *testing.T) {
	r := chi.NewRouter()
	dir := "./" // Tests only have access to the folder they are in
	filename := "test.txt"

	err := os.WriteFile(filepath.Join(dir, filename), []byte("hello world"), 0600)
	if err != nil {
		t.Fatalf("could not create test file: %v", err)
	}
	defer os.Remove(filepath.Join(dir, filename))

	filesystem.FileServer(r, "/", filesystem.Myfs{Dir: http.Dir(dir)})

	// Test: get the file
	req := httptest.NewRequest("GET", "/"+filename, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}
	// Test: Ensure redirect for missing trailing slash
	req = httptest.NewRequest("GET", "/"+filename+"/", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusMovedPermanently {
		t.Errorf("Expected status 301 Moved Permanently, got %d", w.Code)
	}
}
