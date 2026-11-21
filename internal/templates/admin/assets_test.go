package templates

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A script is stamped with when it last changed, so a browser holding an older
// copy fetches the new one rather than running it against newer markup.
func TestAssetURL_StampsAFileWithItsModifiedTime(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "static", "js"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "static", "js", "x.js"), []byte("//"), 0o600))
	t.Chdir(dir)

	assert.Regexp(t, `^/static/js/x\.js\?v=\d+$`, assetURL("/static/js/x.js"))
}

// A file that cannot be found is still linked: a missing stamp must not take
// the script away.
func TestAssetURL_LeavesAMissingFileUnstamped(t *testing.T) {
	t.Chdir(t.TempDir())

	assert.Equal(t, "/static/js/gone.js", assetURL("/static/js/gone.js"))
}
