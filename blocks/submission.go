package blocks

import "encoding/json"

// maxReportedText caps a reported answer. What a player types reaches an
// append-only table that is read whole on every view of the run, so a long
// value is paid for forever: past this, the detail is not worth the row. The
// cap truncates rather than refuses, because the attempt happened either way.
const maxReportedText = 512

// marshalSubmission encodes what a submission carried, for a block's
// game.SubmissionReporter method. A marshal failure drops the detail rather than
// the event: the attempt still happened.
func marshalSubmission(submitted any) json.RawMessage {
	encoded, err := json.Marshal(submitted)
	if err != nil {
		return nil
	}
	return encoded
}

// reportText is a submitted string as it goes into the stream. Truncation
// happens on the value, never on the encoded JSON, which would leave the corpus
// holding a blob nothing can read.
func reportText(value string) string {
	if len(value) <= maxReportedText {
		return value
	}
	return value[:maxReportedText]
}
