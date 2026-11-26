package templates //nolint:testpackage // uses the package's tree and viewOf helpers

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/require"
)

// TestQuestScanPreview writes a browsable copy of the quest page with the scan
// button when RAPUA_PREVIEW names a path. RAPUA_PREVIEW_OPEN opens the sheet.
func TestQuestScanPreview(t *testing.T) {
	path := os.Getenv("RAPUA_PREVIEW")
	if path == "" {
		t.Skip("set RAPUA_PREVIEW to write a browsable copy of the page")
	}
	rows := tree(
		obj("root", "", "Welcome Week"),
		obj("library", "root", "The Library"),
		obj("quad", "root", "The Quad"),
		obj("cafe", "root", "The Café"),
		obj("gym", "root", "The Gym"),
		obj("lab", "root", "The Lab"),
	)
	view := viewOf(rows[1:], rows)
	view.CanDiscover = true

	var out strings.Builder
	page := AppLayout(QuestPage(ObjectivesParams{Run: models.Run{Code: "PJXP"}, View: view}),
		AppChrome{Active: "quest"}, "Quest", nil)
	require.NoError(t, page.Render(context.Background(), &out))

	html := out.String()
	if os.Getenv("RAPUA_PREVIEW_OPEN") != "" {
		html = strings.Replace(html, "</body>",
			`<script>document.getElementById("scan-sheet").showModal()</script></body>`, 1)
	}
	require.NoError(t, os.WriteFile(path, []byte(html), 0o600))
}
