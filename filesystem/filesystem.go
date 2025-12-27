// Package filesystem handles open files and folder directories.
// This implementation specifically disallows directory listings.
package filesystem

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi"
)

// FileSystem interface to allow access to local files.
type FileSystem interface {
	Open(name string) (http.File, error)
}

// Myfs is the accessible directory.
type Myfs struct {
	Dir http.Dir
}

// Open returns the file if available.
// It will not return directory listings.
func (m Myfs) Open(name string) (http.File, error) {
	f, err := m.Dir.Open(name)
	if err != nil {
		return nil, err
	}

	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}

	if fi.IsDir() {
		// Return a response that would have occurred if directory didn't exist
		return m.Dir.Open("does-not-exist")
	}
	return f, nil
}

// FileServer serves the requested file.
func FileServer(r chi.Router, path string, root http.FileSystem) {
	if strings.ContainsAny(path, "{}*") {
		panic("FileServer does not permit URL parameters")
	}

	if path != "/" && path[len(path)-1] != '/' {
		r.Get(path, http.RedirectHandler(path+"/", http.StatusMovedPermanently).ServeHTTP)
		path += "/"
	}
	path += "*"

	r.Get(path, func(w http.ResponseWriter, r *http.Request) {
		rctx := chi.RouteContext(r.Context())
		pathPrefix := strings.TrimSuffix(rctx.RoutePattern(), "/*")
		fs := http.StripPrefix(pathPrefix, http.FileServer(root))
		fs.ServeHTTP(w, r)
	})
}
