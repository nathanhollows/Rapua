package blocks

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const (
	scanBlockType = "scan"
	// Enough to see what a player tried without letting a camera fill the row.
	maxRecordedScans = 10
)

// MatchRule decides how a scanned value is compared with the expected one.
type MatchRule string

// contains accepts a value carrying the code, so a QR encoding a URL matches the
// code printed beneath it. It ignores case as well, since a value long enough to
// wrap a code is rarely one anybody retypes exactly.
const (
	MatchExact           MatchRule = "exact"
	MatchCaseInsensitive MatchRule = "ci"
	MatchContains        MatchRule = "contains"
)

func (m MatchRule) known() bool {
	return m == MatchCaseInsensitive || m == MatchExact || m == MatchContains
}

// ScanModality is how the player produced the value. Not recoverable later.
type ScanModality string

const (
	ModalityCamera ScanModality = "camera"
	ModalityTyped  ScanModality = "typed"
)

// ScanCode is one value that satisfies the block.
type ScanCode struct {
	Value string `json:"value"`
}

type ScanBlock struct {
	BaseBlock
	Prompt string     `json:"prompt"`
	Codes  []ScanCode `json:"codes"`
	Match  MatchRule  `json:"match"`
	// AllowTyping is off unless set: a typed code can be passed along in a message.
	AllowTyping bool `json:"allow_typing,omitempty"`
}

type scanBlockData struct {
	Attempts int          `json:"attempts"`
	Scans    []string     `json:"scans"`
	Modality ScanModality `json:"modality,omitempty"`
}

func (b *ScanBlock) GetID() string      { return b.ID }
func (b *ScanBlock) GetType() string    { return scanBlockType }
func (b *ScanBlock) GetOwnerID() string { return b.OwnerID }
func (b *ScanBlock) GetName() string    { return "Scan" }
func (b *ScanBlock) GetDescription() string {
	return "Players scan a QR code or barcode to proceed."
}
func (b *ScanBlock) GetOrder() int  { return b.Order }
func (b *ScanBlock) GetPoints() int { return b.Points }
func (b *ScanBlock) GetIconSVG() string {
	return `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-scan-qr-code"><path d="M17 12v4a1 1 0 0 1-1 1h-4"/><path d="M17 3h2a2 2 0 0 1 2 2v2"/><path d="M17 8V7"/><path d="M21 17v2a2 2 0 0 1-2 2h-2"/><path d="M3 7V5a2 2 0 0 1 2-2h2"/><path d="M7 17h.01"/><path d="M7 21H5a2 2 0 0 1-2-2v-2"/><rect x="7" y="7" width="5" height="5" rx="1"/></svg>`
}

func (b *ScanBlock) GetAdminData() interface{} {
	return &b
}

func (b *ScanBlock) GetData() json.RawMessage {
	data, _ := json.Marshal(b)
	return data
}

func (b *ScanBlock) ParseData() error {
	return json.Unmarshal(b.Data, b)
}

func (b *ScanBlock) UpdateBlockData(input map[string][]string) error {
	if input["points"] != nil {
		points, err := strconv.Atoi(input["points"][0])
		if err != nil {
			return errors.New("points must be an integer")
		}
		b.Points = points
	}

	b.Codes = parseScanCodes(input)

	if input["prompt"] != nil {
		b.Prompt = input["prompt"][0]
	}

	b.AllowTyping = len(input["allow_typing"]) > 0 && input["allow_typing"][0] == "on"
	b.Match = MatchExact
	if input["match"] != nil && strings.TrimSpace(input["match"][0]) != "" {
		rule := MatchRule(strings.TrimSpace(input["match"][0]))
		if !rule.known() {
			return fmt.Errorf("match must be one of exact, ci, contains: got %q", rule)
		}
		b.Match = rule
	}
	return nil
}

func (b *ScanBlock) ToYAML() map[string]any {
	codes := make([]map[string]any, 0, len(b.Codes))
	for _, c := range b.Codes {
		codes = append(codes, map[string]any{"value": c.Value})
	}

	m := map[string]any{
		"codes": codes,
		"match": string(b.matchRule()),
	}
	if b.Prompt != "" {
		m["prompt"] = b.Prompt
	}
	if b.AllowTyping {
		m["allow_typing"] = true
	}
	return m
}

func (b *ScanBlock) RequiresValidation() bool { return true }

// Defaults rather than failing closed, so an imported block still compares.
func (b *ScanBlock) matchRule() MatchRule {
	if b.Match.known() {
		return b.Match
	}
	return MatchExact
}

// Accepts is Matches, and also refuses a typed code when the block only takes scans.
func (b *ScanBlock) Accepts(scanned string, modality ScanModality) bool {
	return b.Matches(scanned) && (modality != ModalityTyped || b.AllowTyping)
}

// Matches compares server-side; the client never says whether it was correct.
// Any one of the block's codes satisfies it.
func (b *ScanBlock) Matches(scanned string) bool {
	scanned = strings.TrimSpace(scanned)
	link := scanLinkCode(scanned)
	for _, c := range b.Codes {
		expected := strings.TrimSpace(c.Value)
		if expected == "" {
			continue
		}
		if matches(b.matchRule(), scanned, expected) || (link != "" && matches(b.matchRule(), link, expected)) {
			return true
		}
	}
	return false
}

// scanLinkCode reads the code out of a Rapua scan link, SITE_URL/s/CODE, which
// is what printed v7 posters encode. Contains would match those too, but it
// also matches a short code inside a longer one.
func scanLinkCode(scanned string) string {
	i := strings.LastIndex(scanned, "/s/")
	if i < 0 {
		return ""
	}
	code, _, _ := strings.Cut(scanned[i+len("/s/"):], "?")
	return strings.TrimSuffix(code, "/")
}

func matches(rule MatchRule, scanned, expected string) bool {
	switch rule {
	case MatchExact:
		return scanned == expected
	case MatchContains:
		return strings.Contains(strings.ToLower(scanned), strings.ToLower(expected))
	case MatchCaseInsensitive:
		return strings.EqualFold(scanned, expected)
	default:
		return scanned == expected
	}
}

// parseScanCodes reads the repeated code fields.
//
// No codes is allowed: the editor saves on every keystroke, so rejecting it here
// would discard the prompt an author is still typing. A block with no codes
// matches nothing, which fails closed at play time.
func parseScanCodes(input map[string][]string) []ScanCode {
	codes := make([]ScanCode, 0, len(input["code"]))
	for _, raw := range input["code"] {
		if value := strings.TrimSpace(raw); value != "" {
			codes = append(codes, ScanCode{Value: value})
		}
	}
	return codes
}

// recordScan keeps the most recent attempts and drops a repeat of the one before
// it. A camera held at the wrong sign posts the same value every frame, and this
// list is persisted state.
func recordScan(scans []string, scanned string) []string {
	if len(scans) > 0 && scans[len(scans)-1] == scanned {
		return scans
	}
	scans = append(scans, scanned)
	if len(scans) > maxRecordedScans {
		scans = scans[len(scans)-maxRecordedScans:]
	}
	return scans
}

func (b *ScanBlock) ValidatePlayerInput(state PlayerState, input map[string][]string) (PlayerState, error) {
	scanned, modality, err := scannedValue(input)
	if err != nil {
		return state, err
	}
	if modality == ModalityTyped && !b.AllowTyping {
		return state, errors.New("this code has to be scanned")
	}

	newPlayerData := scanBlockData{}
	if state.GetPlayerData() != nil {
		if parseErr := json.Unmarshal(state.GetPlayerData(), &newPlayerData); parseErr != nil {
			return state, fmt.Errorf("parse player data: %w", parseErr)
		}
	}

	newPlayerData.Attempts++
	newPlayerData.Scans = recordScan(newPlayerData.Scans, scanned)
	newPlayerData.Modality = modality

	playerData, err := json.Marshal(newPlayerData)
	if err != nil {
		return state, errors.New("error saving player data")
	}
	state.SetPlayerData(playerData)

	if !b.Matches(scanned) {
		return state, nil
	}

	state.SetComplete(true)
	state.SetPointsAwarded(b.Points)
	return state, nil
}

// The camera and the text box post different fields, which is the only place the
// modality survives.
func scannedValue(input map[string][]string) (string, ScanModality, error) {
	if v := input["scanned"]; len(v) > 0 && strings.TrimSpace(v[0]) != "" {
		return v[0], ModalityCamera, nil
	}
	if v := input["code"]; len(v) > 0 && strings.TrimSpace(v[0]) != "" {
		return v[0], ModalityTyped, nil
	}
	return "", "", errors.New("scan a code or type it in")
}

// Submission returns the code that did not match, and how it was read.
func (b *ScanBlock) Submission(input map[string][]string, state PlayerState) json.RawMessage {
	if state.IsComplete() {
		return nil
	}
	scanned, modality, err := scannedValue(input)
	if err != nil {
		return nil
	}
	return marshalSubmission(map[string]any{"scanned": reportText(scanned), "modality": modality})
}
