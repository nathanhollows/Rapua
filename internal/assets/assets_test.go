package assets_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nathanhollows/Rapua/v8/internal/assets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A script is stamped with when it last changed, so a browser holding an older
// copy fetches the new one rather than running it against newer markup.
func TestURL_StampsAFileWithItsModifiedTime(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "static", "js"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "static", "js", "x.js"), []byte("//"), 0o600))
	t.Chdir(dir)

	assert.Regexp(t, `^/static/js/x\.js\?v=\d+$`, assets.URL("/static/js/x.js"))
}

// The stamp follows the file: touching it changes the URL, leaving it does not.
func TestURL_ChangesWhenTheFileDoes(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "static"), 0o750))
	file := filepath.Join(dir, "static", "x.js")
	require.NoError(t, os.WriteFile(file, []byte("//"), 0o600))
	t.Chdir(dir)

	first := assets.URL("/static/x.js")
	assert.Equal(t, first, assets.URL("/static/x.js"), "untouched, the same URL")

	later := mustTime(t, 2_000_000_000)
	require.NoError(t, os.Chtimes(file, later, later))
	assert.NotEqual(t, first, assets.URL("/static/x.js"), "a rebuilt file is fetched again")
}

// A file that cannot be found is still linked: a missing stamp must not take
// the script away.
func TestURL_LeavesAMissingFileUnstamped(t *testing.T) {
	t.Chdir(t.TempDir())

	assert.Equal(t, "/static/js/gone.js", assets.URL("/static/js/gone.js"))
}

func mustTime(t *testing.T, unix int64) time.Time {
	t.Helper()
	return time.Unix(unix, 0)
}
