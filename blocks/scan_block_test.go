package blocks_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func scanBlock(match blocks.MatchRule, values ...string) *blocks.ScanBlock {
	codes := make([]blocks.ScanCode, 0, len(values))
	for _, v := range values {
		codes = append(codes, blocks.ScanCode{Value: v})
	}
	return &blocks.ScanBlock{Match: match, Codes: codes}
}

func TestScanBlock_Matches(t *testing.T) {
	cases := []struct {
		name    string
		match   blocks.MatchRule
		scanned string
		want    bool
	}{
		{name: "ci accepts a retyped code", match: blocks.MatchCaseInsensitive, scanned: "abcde", want: true},
		{name: "ci trims surrounding space", match: blocks.MatchCaseInsensitive, scanned: "  ABCDE ", want: true},
		{name: "ci rejects a different code", match: blocks.MatchCaseInsensitive, scanned: "FGHJK", want: false},
		{name: "exact rejects a case change", match: blocks.MatchExact, scanned: "abcde", want: false},
		{name: "exact accepts the same bytes", match: blocks.MatchExact, scanned: "ABCDE", want: true},
		{
			name: "contains accepts a URL carrying it", match: blocks.MatchContains,
			scanned: "https://rapua.nz/s/ABCDE", want: true,
		},
		{
			name: "contains rejects another code", match: blocks.MatchContains,
			scanned: "https://rapua.nz/s/FGHJK", want: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, scanBlock(c.match, "ABCDE").Matches(c.scanned))
		})
	}
}

// Several signs can satisfy one block, so any code counts.
func TestScanBlock_AnyCodeMatches(t *testing.T) {
	b := scanBlock(blocks.MatchCaseInsensitive, "ABCDE", "FGHJK")
	assert.True(t, b.Matches("ABCDE"))
	assert.True(t, b.Matches("fghjk"))
	assert.False(t, b.Matches("LMNPR"))
}

// An imported block without a rule must still compare, not reject every scan.
func TestScanBlock_UnsetMatchRuleDefaultsToExact(t *testing.T) {
	b := scanBlock("", "ABCDE")
	assert.True(t, b.Matches("ABCDE"))
	assert.False(t, b.Matches("abcde"))
}

func TestScanBlock_UnknownMatchRuleDefaultsToExact(t *testing.T) {
	b := scanBlock(blocks.MatchRule("fuzzy"), "ABCDE")
	assert.True(t, b.Matches("ABCDE"))
	assert.False(t, b.Matches("abcde"))
}

// A block with no codes would otherwise be a sign everyone passes.
func TestScanBlock_NoCodesNeverMatches(t *testing.T) {
	b := &blocks.ScanBlock{}
	assert.False(t, b.Matches(""))
	assert.False(t, b.Matches("anything"))
}

func TestScanBlock_BlankCodeNeverMatches(t *testing.T) {
	b := scanBlock(blocks.MatchCaseInsensitive, "   ")
	assert.False(t, b.Matches(""))
	assert.False(t, b.Matches("   "))
}

// Walking to a sign and being told the code are only distinguishable here.
func TestScanBlock_RecordsModality(t *testing.T) {
	cases := []struct {
		name  string
		input map[string][]string
		want  blocks.ScanModality
	}{
		{name: "camera", input: map[string][]string{"scanned": {"ABCDE"}}, want: blocks.ModalityCamera},
		{name: "typed", input: map[string][]string{"code": {"ABCDE"}}, want: blocks.ModalityTyped},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b := scanBlock(blocks.MatchCaseInsensitive, "ABCDE")
			state, err := b.ValidatePlayerInput(&blocks.MockPlayerState{}, c.input)
			require.NoError(t, err)
			assert.True(t, state.IsComplete())

			var data struct {
				Modality blocks.ScanModality `json:"modality"`
				Attempts int                 `json:"attempts"`
			}
			require.NoError(t, json.Unmarshal(state.GetPlayerData(), &data))
			assert.Equal(t, c.want, data.Modality)
			assert.Equal(t, 1, data.Attempts)
		})
	}
}

func TestScanBlock_WrongCodeDoesNotComplete(t *testing.T) {
	b := scanBlock(blocks.MatchCaseInsensitive, "ABCDE")
	state, err := b.ValidatePlayerInput(&blocks.MockPlayerState{}, map[string][]string{"scanned": {"FGHJK"}})
	require.NoError(t, err)
	assert.False(t, state.IsComplete())

	var data struct {
		Scans []string `json:"scans"`
	}
	require.NoError(t, json.Unmarshal(state.GetPlayerData(), &data))
	assert.Equal(t, []string{"FGHJK"}, data.Scans, "a wrong scan is still recorded")
}

// A camera held at the wrong sign posts every frame, and this list is persisted.
func TestScanBlock_RecordedScansAreBounded(t *testing.T) {
	b := scanBlock(blocks.MatchExact, "ABCDE")
	state := blocks.PlayerState(&blocks.MockPlayerState{})

	for i := range 40 {
		var err error
		// Alternate so the consecutive-repeat guard does not absorb everything.
		state, err = b.ValidatePlayerInput(state, map[string][]string{
			"scanned": {fmt.Sprintf("WRONG%d", i%2)},
		})
		require.NoError(t, err)
	}

	var data struct {
		Attempts int      `json:"attempts"`
		Scans    []string `json:"scans"`
	}
	require.NoError(t, json.Unmarshal(state.GetPlayerData(), &data))
	assert.Equal(t, 40, data.Attempts, "every attempt still counts")
	assert.LessOrEqual(t, len(data.Scans), 10, "the stored list is capped")
}

// A camera posts the same value every frame while it stays pointed at one sign.
func TestScanBlock_ConsecutiveRepeatsAreNotStored(t *testing.T) {
	b := scanBlock(blocks.MatchExact, "ABCDE")
	state := blocks.PlayerState(&blocks.MockPlayerState{})

	for range 5 {
		var err error
		state, err = b.ValidatePlayerInput(state, map[string][]string{"scanned": {"WRONG"}})
		require.NoError(t, err)
	}

	var data struct {
		Attempts int      `json:"attempts"`
		Scans    []string `json:"scans"`
	}
	require.NoError(t, json.Unmarshal(state.GetPlayerData(), &data))
	assert.Equal(t, []string{"WRONG"}, data.Scans)
	assert.Equal(t, 5, data.Attempts)
}

func TestScanBlock_EmptyInputErrors(t *testing.T) {
	b := scanBlock(blocks.MatchCaseInsensitive, "ABCDE")
	_, err := b.ValidatePlayerInput(&blocks.MockPlayerState{}, map[string][]string{})
	require.Error(t, err)

	_, err = b.ValidatePlayerInput(&blocks.MockPlayerState{}, map[string][]string{"code": {"   "}})
	require.Error(t, err, "whitespace is not a scan")
}

// Generate is off unless the author asks for it.
func TestScanBlock_MintsOnlyFlaggedCodes(t *testing.T) {
	b := &blocks.ScanBlock{Codes: []blocks.ScanCode{
		{Value: "ABCDE", Generate: true},
		{Value: "9780143567592"},
		{Value: "FGHJK", Generate: true},
	}}
	assert.Equal(t, []string{"ABCDE", "FGHJK"}, b.MintedCodes())
}

func TestScanBlock_MintsNothingByDefault(t *testing.T) {
	assert.Empty(t, scanBlock(blocks.MatchCaseInsensitive, "ABCDE").MintedCodes())
	assert.Empty(t, (&blocks.ScanBlock{}).MintedCodes())
}

func TestScanBlock_UpdateBlockData(t *testing.T) {
	var b blocks.ScanBlock
	require.NoError(t, b.UpdateBlockData(map[string][]string{
		"code":     {"  ABCDE ", "9780143567592"},
		"generate": {"0"},
		"prompt":   {"Find the totara"},
		"match":    {"ci"},
		"points":   {"5"},
	}))

	require.Len(t, b.Codes, 2)
	assert.Equal(t, "ABCDE", b.Codes[0].Value, "surrounding space would never survive a scan")
	assert.True(t, b.Codes[0].Generate)
	assert.False(t, b.Codes[1].Generate, "an unticked row is not generated")
	assert.Equal(t, blocks.MatchCaseInsensitive, b.Match)
	assert.Equal(t, 5, b.GetPoints())
}

func TestScanBlock_UpdateBlockDataSkipsBlankRows(t *testing.T) {
	var b blocks.ScanBlock
	require.NoError(t, b.UpdateBlockData(map[string][]string{"code": {"ABCDE", "  ", ""}}))
	require.Len(t, b.Codes, 1)
}

func TestScanBlock_UpdateBlockDataRejectsBadInput(t *testing.T) {
	cases := map[string]map[string][]string{
		"unknown match":     {"code": {"ABCDE"}, "match": {"fuzzy"}},
		"points not an int": {"code": {"ABCDE"}, "points": {"lots"}},
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			var b blocks.ScanBlock
			require.Error(t, b.UpdateBlockData(input))
		})
	}
}

// The editor saves on every keystroke, so rejecting an empty code list would
// discard the prompt an author is still typing.
func TestScanBlock_UpdateBlockDataAcceptsNoCodes(t *testing.T) {
	cases := map[string]map[string][]string{
		"no code field":    {"prompt": {"Find it"}},
		"only blank codes": {"code": {"   ", ""}},
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			b := blocks.ScanBlock{}
			require.NoError(t, b.UpdateBlockData(input))
			assert.Empty(t, b.Codes)
			assert.False(t, b.Matches("anything"), "no codes fails closed at play time")
		})
	}
}

func TestScanBlock_UpdateBlockDataKeepsPromptWithoutCodes(t *testing.T) {
	var b blocks.ScanBlock
	require.NoError(t, b.UpdateBlockData(map[string][]string{"prompt": {"Find the totara"}}))
	assert.Equal(t, "Find the totara", b.Prompt)
}

func TestScanBlock_UpdateBlockDataDefaultsMatch(t *testing.T) {
	var b blocks.ScanBlock
	require.NoError(t, b.UpdateBlockData(map[string][]string{"code": {"ABCDE"}}))
	assert.Equal(t, blocks.MatchExact, b.Match)
}

func TestScanBlock_ToYAML(t *testing.T) {
	b := &blocks.ScanBlock{Codes: []blocks.ScanCode{
		{Value: "ABCDE", Generate: true},
		{Value: "9780143567592"},
	}}
	m := b.ToYAML()

	assert.Equal(t, "exact", m["match"])

	codes, ok := m["codes"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, codes, 2)
	assert.Equal(t, map[string]any{"value": "ABCDE", "generate": true}, codes[0])
	assert.Equal(t, map[string]any{"value": "9780143567592"}, codes[1], "the default is not restated")
}
