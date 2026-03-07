package static

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/disintegration/imaging"
	"github.com/go-chi/chi"
)

// Image size dimensions.
var imageSizes = map[string]int{
	"small":  640,  // Mobile devices
	"medium": 1024, // Tablets
	"large":  1920, // Desktop/HD displays
}

// ServeResizedImage handles image requests with optional ?size= parameter.
func ServeResizedImage(baseDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract the file path from the URL
		rctx := chi.RouteContext(r.Context())
		_ = strings.TrimPrefix(rctx.RoutePattern(), "/static/uploads/")

		// Get the actual file path from the URL
		urlPath := strings.TrimPrefix(r.URL.Path, "/static/uploads/")
		originalPath := filepath.Join(baseDir, "static", "uploads", urlPath)

		// Security: Block direct access to cache directories
		if isCachePath(urlPath) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		// Check if original file exists
		if _, err := os.Stat(originalPath); os.IsNotExist(err) {
			http.NotFound(w, r)
			return
		}

		// Get size parameter
		sizeParam := r.URL.Query().Get("size")

		// If no size parameter or not an image, serve original
		if sizeParam == "" || !isImageFile(originalPath) {
			http.ServeFile(w, r, originalPath)
			return
		}

		// Validate size parameter
		maxWidth, ok := imageSizes[sizeParam]
		if !ok {
			http.Error(w, "Invalid size parameter. Use: small, medium, or large", http.StatusBadRequest)
			return
		}

		// Generate cached file path
		cachedPath := getCachedPath(baseDir, urlPath, sizeParam)

		// Check if cached version exists
		if _, err := os.Stat(cachedPath); err == nil {
			http.ServeFile(w, r, cachedPath)
			return
		}

		// Generate resized image
		if err := resizeImage(originalPath, cachedPath, maxWidth); err != nil {
			slog.Error("failed to resize image", "path", originalPath, "err", err)
			http.Error(w, "Failed to resize image", http.StatusInternalServerError)
			return
		}

		// Serve the resized image
		http.ServeFile(w, r, cachedPath)
	}
}

// getCachedPath returns the path where the cached resized image should be stored.
func getCachedPath(baseDir, relativePath, size string) string {
	return filepath.Join(baseDir, "static", "uploads", ".cache", size, relativePath)
}

// resizeImage creates a resized version of the image.
func resizeImage(srcPath, dstPath string, maxWidth int) error {
	// Open the source image
	src, err := imaging.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open image: %w", err)
	}

	// Resize the image to fit within maxWidth x maxWidth while maintaining aspect ratio
	resized := imaging.Fit(src, maxWidth, maxWidth, imaging.Lanczos)

	// Create the cache directory if it doesn't exist
	cacheDir := filepath.Dir(dstPath)
	err = os.MkdirAll(cacheDir, 0750)
	if err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Save the resized image
	err = imaging.Save(resized, dstPath)
	if err != nil {
		return fmt.Errorf("failed to save resized image: %w", err)
	}

	return nil
}

// isImageFile checks if the file is an image based on its extension.
func isImageFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp":
		return true
	}
	return false
}

// isCachePath checks if the path is trying to access the cache directory directly.
func isCachePath(path string) bool {
	return strings.Contains(path, "/.cache/")
}
