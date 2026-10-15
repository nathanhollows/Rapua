package templates

import (
	"fmt"
	"time"

	"github.com/nathanhollows/Rapua/v8/internal/services"
	"github.com/nathanhollows/Rapua/v8/models"
)

// journalChip is one filter on the feed, and the kinds it keeps. Nil kinds
// means everything.
type journalChip struct {
	Slug  string
	Label string
	Kinds []services.JournalEntryKind
}

// journalChipsList is the filter set, everything first. The three named chips
// partition the feed; the opening and reset lines ride with everything only,
// because they are the bookends rather than entries in a filter's sense.
func journalChipsList() []journalChip {
	return []journalChip{
		{Slug: "", Label: "Everything"},
		{Slug: "finds", Label: "Finds", Kinds: []services.JournalEntryKind{
			services.JournalFind, services.JournalMiss,
			services.JournalMilestone, services.JournalFinish,
		}},
		{Slug: "photos", Label: "Photos", Kinds: []services.JournalEntryKind{
			services.JournalPhoto,
		}},
		{Slug: "messages", Label: "Messages", Kinds: []services.JournalEntryKind{
			services.JournalMessage,
		}},
	}
}

// filteredEntries keeps the kinds a chip asks for. An unknown or absent value
// reads as everything.
func filteredEntries(entries []services.JournalEntry, show string) []services.JournalEntry {
	for _, chip := range journalChipsList() {
		if chip.Slug != show {
			continue
		}
		if chip.Kinds == nil {
			return entries
		}
		kept := make([]services.JournalEntry, 0, len(entries))
		for _, entry := range entries {
			if slicesContainsKind(chip.Kinds, entry.Kind) {
				kept = append(kept, entry)
			}
		}
		return kept
	}
	return entries
}

func slicesContainsKind(kinds []services.JournalEntryKind, kind services.JournalEntryKind) bool {
	for _, k := range kinds {
		if k == kind {
			return true
		}
	}
	return false
}

func journalChipHref(slug string) string {
	if slug == "" {
		return "/journal"
	}
	return "/journal?show=" + slug
}

// journalTime is how a moment reads in the feed: the clock, local, nothing else.
func journalTime(t time.Time) string {
	return t.Local().Format("3:04pm")
}

// journalPhotoLabel switches on the upload's kind (image or video), not a MIME type.
func journalPhotoLabel(mediaType string) string {
	if mediaType == string(models.MediaTypeImage) {
		return "Took a photo"
	}
	return "Added a file"
}

func journalIsImage(mediaType string) bool {
	return mediaType == string(models.MediaTypeImage)
}

// journalReadLabel is how long a message waited to be cleared. The log records
// a dismissal, and "read" is what a player understands by tapping one.
func journalReadLabel(entry services.JournalEntry) string {
	if !entry.Read {
		return "Unread"
	}
	return "Read in " + journalShortDuration(entry.ReadWait)
}

const minutesPerHour = 60

func journalShortDuration(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "under a minute"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	default:
		hours := int(d.Hours())
		minutes := int(d.Minutes()) - hours*minutesPerHour
		if minutes == 0 {
			return fmt.Sprintf("%dh", hours)
		}
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
}

// journalMilestoneLabel says what a section closed with. "All" only when every
// child in play landed; a band can close a section short of that.
func journalMilestoneLabel(entry services.JournalEntry) string {
	if entry.Children > 0 && entry.Found >= entry.Children {
		return fmt.Sprintf("All %d found", entry.Children)
	}
	return fmt.Sprintf("%d of %d found", entry.Found, entry.Children)
}
