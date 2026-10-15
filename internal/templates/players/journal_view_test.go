package templates_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/nathanhollows/Rapua/v8/internal/services"
	templates "github.com/nathanhollows/Rapua/v8/internal/templates/players"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An upload event records the upload's own kind, image or video, so a photo
// entry labels itself and draws its thumbnail from that rather than from a
// file-type prefix.
func TestJournal_PhotoEntriesDrawTheirKind(t *testing.T) {
	render := func(mediaType string) string {
		view := services.JournalView{Entries: []services.JournalEntry{{
			Kind:      services.JournalPhoto,
			Time:      time.Now(),
			MediaURL:  "/uploads/x.jpg",
			MediaType: mediaType,
		}}}
		var page bytes.Buffer
		require.NoError(t, templates.Journal(templates.JournalParams{View: view}).
			Render(context.Background(), &page))
		return page.String()
	}

	photo := render(string(models.MediaTypeImage))
	assert.Contains(t, photo, "Took a photo")
	assert.Contains(t, photo, "<img")

	video := render(string(models.MediaTypeVideo))
	assert.Contains(t, video, "Added a file")
	assert.NotContains(t, video, "<img")
}
