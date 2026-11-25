package blocks_test

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/nathanhollows/Rapua/v8/blocks"
	templates "github.com/nathanhollows/Rapua/v8/internal/templates/blocks"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeState struct {
	complete bool
	points   int
	data     json.RawMessage
}

func (s *fakeState) GetBlockID() string              { return "b" }
func (s *fakeState) GetPlayerID() string             { return "ABCDEF" }
func (s *fakeState) GetQuestID() string              { return "q" }
func (s *fakeState) GetPlayerData() json.RawMessage  { return s.data }
func (s *fakeState) SetPlayerData(d json.RawMessage) { s.data = d }
func (s *fakeState) IsComplete() bool                { return s.complete }
func (s *fakeState) SetComplete(c bool)              { s.complete = c }
func (s *fakeState) GetPointsAwarded() int           { return s.points }
func (s *fakeState) SetPointsAwarded(int)            {}

func player(t *testing.T, c templ.Component) string {
	t.Helper()
	require.NotNil(t, c)
	var buf bytes.Buffer
	require.NoError(t, c.Render(context.Background(), &buf))
	return buf.String()
}

func open() *fakeState { return &fakeState{data: json.RawMessage(`{}`)} }

var withPoints = models.QuestSettings{EnablePoints: true}

func newQuiz(multiple, retry bool) *blocks.QuizBlock {
	return &blocks.QuizBlock{
		BaseBlock: blocks.BaseBlock{ID: "quiz", Points: 10}, Question: "What glares?",
		MultipleChoice: multiple, RetryEnabled: retry,
		Options: []blocks.QuizOption{
			{ID: "option_0", Text: "The library", IsCorrect: true},
			{ID: "option_1", Text: "The cafe"},
		},
		UnlockedContent: "Well done.",
	}
}

// A heading's icon is a sticker, sized with the title, and a title alone has none.
func TestHeader_IconIsASizedSticker(t *testing.T) {
	small := &blocks.HeaderBlock{TitleText: "Look", Icon: "compass", TitleSize: "small"}
	html := player(t, templates.RenderPlayerView(withPoints, small, open()))

	assert.Contains(t, html, `class="qb-head sm"`)
	assert.Contains(t, html, `class="qb-sticker sm"`)
	assert.Contains(t, html, "qb-hop", "poking it makes it hop")

	plain := player(t, templates.RenderPlayerView(withPoints, &blocks.HeaderBlock{TitleText: "Look"}, open()))
	assert.NotContains(t, plain, "qb-sticker")
}

// Each callout tone has its own icon, in a chip.
func TestAlert_ToneChoosesTheIcon(t *testing.T) {
	for style, icon := range map[string]string{"info": `d="M12 16v-4"`, "success": `d="m9 12 2 2 4-4"`, "warning": `d="m21.73 18-8-14`, "error": `d="m15 9-6 6"`} {
		html := player(
			t,
			templates.RenderPlayerView(withPoints, &blocks.AlertBlock{Content: "**Heads up.**", Style: style}, open()),
		)
		assert.Contains(t, html, `class="qb-ai"`, style)
		assert.Contains(t, html, icon, style)
		assert.Contains(t, html, "<strong>Heads up.</strong>", style)
	}
}

// A link out says that it is one: an arrow on the button, a chip on an image.
func TestLinks_SayTheyOpenANewTab(t *testing.T) {
	button := player(
		t,
		templates.RenderPlayerView(
			withPoints,
			&blocks.ButtonBlock{Text: "Open the map", Link: "https://example.test", Style: "primary"},
			open(),
		),
	)
	assert.Contains(t, button, "Open the map")
	assert.NotContains(t, button, "<small>", "the arrow says it, so there is no sentence under the button")
	assert.Contains(t, button, "(opens in a new tab)", "still said to a screen reader")
	assert.Contains(t, button, `rel="noopener"`)

	linked := player(
		t,
		templates.RenderPlayerView(
			withPoints,
			&blocks.ImageBlock{URL: "https://example.test/a.png", Caption: "Cap", Link: "https://example.test"},
			open(),
		),
	)
	assert.Contains(t, linked, "Opens in a new tab")

	plain := player(
		t,
		templates.RenderPlayerView(
			withPoints,
			&blocks.ImageBlock{URL: "https://example.test/a.png", Caption: "Cap"},
			open(),
		),
	)
	assert.NotContains(t, plain, "Opens in a new tab", "a plain image is not a link")
}

// A divider with no title is a bare dotted line; one with a title says it, in plain text.
func TestDivider_BareLineOrPlainTitle(t *testing.T) {
	bare := player(t, templates.RenderPlayerView(withPoints, &blocks.DividerBlock{}, open()))
	assert.Contains(t, bare, `<div class="qb-div" role="separator"></div>`, "nothing in it, so the line runs unbroken")

	titled := player(t, templates.RenderPlayerView(withPoints, &blocks.DividerBlock{Title: "Part two"}, open()))
	assert.Contains(t, titled, `<span class="qb-div-lbl">Part two</span>`)
}

// A reveal says it can be opened, and one with no title still does.
func TestToggleText_HintsThatItOpens(t *testing.T) {
	html := player(
		t,
		templates.RenderPlayerView(
			withPoints,
			&blocks.ToggleTextBlock{Title: "Where is the key?", Content: "Under the pot."},
			open(),
		),
	)
	assert.Contains(t, html, "Where is the key?")
	assert.Contains(t, html, "<small>Tap to read</small>")
	assert.Contains(t, html, "Under the pot.")

	untitled := player(
		t,
		templates.RenderPlayerView(withPoints, &blocks.ToggleTextBlock{Content: "Under the pot."}, open()),
	)
	assert.Contains(t, untitled, "Tap to read")
	assert.NotContains(t, untitled, "<small>")
}

// The waiting start button is not pressable, and does not pretend to be.
func TestStartButton_WaitingIsNotAButton(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(
		t,
		templates.StartGameButtonScheduled(blocks.StartGameButtonBlock{}).Render(context.Background(), &buf),
	)

	assert.Contains(t, buf.String(), "Waiting for the game to start")
	assert.Contains(t, buf.String(), "qb-wait")
	assert.NotContains(t, buf.String(), "<button")
}

// The quiz card: one radio group so only one answer can be taken, and a button
// that names what is missing.
func TestQuiz_OpenCardIsARadioGroupWithAButtonThatWaits(t *testing.T) {
	html := player(t, templates.RenderPlayerView(withPoints, newQuiz(false, true), open()))

	assert.Equal(
		t,
		2,
		strings.Count(html, `type="radio" name="quiz_option"`),
		"every option shares one name, so choosing one clears the others",
	)
	assert.NotContains(t, html, `type="checkbox"`)
	assert.Contains(t, html, "Pick one answer.")
	assert.Regexp(
		t,
		`<button[^>]*type="submit"[^>]*\sdisabled[\s>=]`,
		html,
		"nothing is picked, so there is nothing to check",
	)
	assert.Contains(t, html, "Pick an answer")
	assert.Contains(t, html, "10 pts")
	assert.NotContains(t, html, "qb-chip earned")
}

func TestQuiz_MultipleChoiceUsesCheckboxes(t *testing.T) {
	html := player(t, templates.RenderPlayerView(withPoints, newQuiz(true, true), open()))

	assert.Equal(t, 2, strings.Count(html, `type="checkbox" name="quiz_option"`))
	assert.Contains(t, html, "Pick all that apply.")
	assert.NotContains(t, html, `type="radio"`)
}

// A finished quiz says in words what became of each option, and shows the chip
// earned.
func TestQuiz_FinishedJudgesOnlyWhatWasPicked(t *testing.T) {
	right := &fakeState{
		complete: true,
		points:   10,
		data:     json.RawMessage(`{"selected_options":["option_0"],"attempts":1,"is_correct":true}`),
	}
	html := player(t, templates.RenderPlayerView(withPoints, newQuiz(false, true), right))

	assert.Contains(t, html, "That&#39;s right")
	assert.NotContains(t, html, "Not this one")
	assert.Contains(t, html, "+10 pts")
	assert.Contains(t, html, "qb-chip earned")
	assert.Contains(t, html, "Well done.", "what the answer unlocked")
	assert.Contains(t, html, `class="q-unlocked prose"`)
	assert.NotContains(t, html, "<form", "nothing left to submit")

	wrong := &fakeState{complete: true, data: json.RawMessage(`{"selected_options":["option_1"],"attempts":1}`)}
	html = player(t, templates.RenderPlayerView(withPoints, newQuiz(false, false), wrong))
	assert.Contains(t, html, "Not this one")
	assert.Contains(t, html, "The right answer", "a right answer they missed is shown once it is over")
	assert.Contains(t, html, "Done", "finished with no points to show")
}

// Pick every right answer and nothing else: only what was picked is judged, so
// the options left alone stay plain.
func TestQuiz_FinishedMultipleChoiceLeavesUnpickedOptionsPlain(t *testing.T) {
	quiz := &blocks.QuizBlock{
		BaseBlock: blocks.BaseBlock{
			ID:     "primes",
			Points: 2,
		},
		Question:       "Pick every prime.",
		MultipleChoice: true,
		RetryEnabled:   true,
		Options: []blocks.QuizOption{
			{ID: "option_0", Text: "2", IsCorrect: true}, {ID: "option_1", Text: "3", IsCorrect: true},
			{ID: "option_2", Text: "4"}, {ID: "option_3", Text: "9"},
		},
	}
	state := &fakeState{
		complete: true,
		points:   2,
		data:     json.RawMessage(`{"selected_options":["option_0","option_1"],"attempts":1,"is_correct":true}`),
	}

	html := player(t, templates.RenderPlayerView(withPoints, quiz, state))

	assert.Equal(t, 2, strings.Count(html, "is-ok"), "the two right ones that were picked")
	assert.NotContains(t, html, "is-no", "4 and 9 were not picked, so they are not marked wrong")
	assert.NotContains(t, html, "Not this one")
}

// A wrong attempt that can be retried says so, and only the update shakes.
func TestQuiz_WrongAttemptShakesOnlyWhenJustAnswered(t *testing.T) {
	attempt := &fakeState{data: json.RawMessage(`{"selected_options":["option_1"],"attempts":2,"is_correct":false}`)}

	update := player(t, templates.RenderPlayerUpdate(withPoints, newQuiz(false, true), attempt))
	assert.Contains(t, update, "Not quite. Have another look, then try again.")
	assert.Contains(t, update, "Attempt 2")
	assert.Contains(t, update, "is-fresh")
	assert.Contains(t, update, "is-wrong")

	reload := player(t, templates.RenderPlayerView(withPoints, newQuiz(false, true), attempt))
	assert.Contains(t, reload, "Not quite.")
	assert.NotContains(t, reload, "is-fresh", "a page reload is not a fresh answer")
}

// Points and completion are one pill.
func TestChip_PointsThenDone(t *testing.T) {
	noPoints := player(t, templates.RenderPlayerView(models.QuestSettings{}, newQuiz(false, true), open()))
	assert.NotContains(t, noPoints, "qb-chip", "no points, and not done: nothing to say")

	done := player(
		t,
		templates.RenderPlayerView(
			models.QuestSettings{},
			newQuiz(false, true),
			&fakeState{
				complete: true,
				data:     json.RawMessage(`{"selected_options":["option_0"],"attempts":1,"is_correct":true}`),
			},
		),
	)
	assert.Contains(t, done, "Done")
	assert.NotRegexp(t, `\d+ pts`, done)

	penalty := newQuiz(false, true)
	penalty.Points = -5
	assert.Contains(t, player(t, templates.RenderPlayerView(withPoints, penalty, open())), "-5 pts")
}

// The to-do list counts what is ticked, and locks once it is done.
func TestChecklist_CountsAndLocks(t *testing.T) {
	list := &blocks.ChecklistBlock{
		BaseBlock: blocks.BaseBlock{ID: "todo", Points: 20},
		Content:   "Before you set off",
		List: []blocks.ChecklistItem{
			{ID: "a", Description: "Charge your phone"},
			{ID: "b", Description: "Find shoes"},
			{ID: "c", Description: "Pack a pen"},
		},
	}

	partly := player(
		t,
		templates.RenderPlayerView(withPoints, list, &fakeState{data: json.RawMessage(`{"checked_items":["a"]}`)}),
	)
	assert.Contains(t, partly, ">To do<", "the kind, in the card's head")
	assert.Contains(t, partly, "1 of 3 done.")
	assert.Equal(t, 1, strings.Count(partly, " checked"), "only the ticked one is checked")
	assert.NotContains(t, partly, " disabled")
	assert.Contains(t, partly, `hx-swap="outerHTML"`, "a tick swaps the card, not the inside of it")

	done := player(
		t,
		templates.RenderPlayerView(
			withPoints,
			list,
			&fakeState{complete: true, points: 20, data: json.RawMessage(`{"checked_items":["a","b","c"]}`)},
		),
	)
	assert.Contains(t, done, "All done.")
	assert.Equal(t, 3, strings.Count(done, " disabled"))
	assert.Contains(t, done, "+20 pts")
}

func newChoice(multi bool) *blocks.ChoiceBlock {
	return &blocks.ChoiceBlock{
		BaseBlock: blocks.BaseBlock{ID: "way"}, Prompt: "Which way?", MultiSelect: multi,
		Options: []blocks.ChoiceOption{{Label: "Left", Sets: "left"}, {Label: "Right", Sets: "right"}},
	}
}

// A choice has no right answer: nothing on it is green or red, and a pick stays,
// locked, once made.
func TestChoice_OpenThenLockedWithTheChosenRows(t *testing.T) {
	first := player(t, templates.RenderPlayerView(withPoints, newChoice(false), open()))
	assert.Equal(
		t,
		2,
		strings.Count(first, `type="radio" name="choice"`),
		"one group, so choosing one clears the others",
	)
	assert.Contains(t, first, "Pick one.")
	assert.Regexp(t, `<button[^>]*type="submit"[^>]*\sdisabled[\s>=]`, first)
	assert.NotContains(t, first, `required`, "the button says what is missing instead of a popup")

	multi := player(t, templates.RenderPlayerView(withPoints, newChoice(true), open()))
	assert.Equal(t, 2, strings.Count(multi, `type="checkbox" name="choice"`))

	chosen := newChoice(false)
	done := player(
		t,
		templates.RenderPlayerView(
			withPoints,
			chosen,
			&fakeState{complete: true, data: json.RawMessage(`{"chosen":["right"]}`)},
		),
	)
	assert.Contains(t, done, "You chose:")
	assert.Contains(t, done, ">Right<")
	assert.NotContains(t, done, ">Left<", "only what was chosen is kept")
	assert.NotContains(t, done, "is-ok")
	assert.NotContains(t, done, "is-no")
}

func newPincode(code string) *blocks.PincodeBlock {
	return &blocks.PincodeBlock{
		BaseBlock:       blocks.BaseBlock{ID: "lock", Points: 2},
		Prompt:          "Enter the code.",
		Pincode:         code,
		UnlockedContent: "Click.",
	}
}

// One box per digit, whatever the length, and long codes wrap to two rows.
func TestPincode_OneBoxPerDigitWrappingPastSix(t *testing.T) {
	short := player(t, templates.RenderPlayerView(withPoints, newPincode("4417"), open()))
	assert.Equal(t, 4, strings.Count(short, `name="pincode"`))
	assert.Contains(t, short, "--per:4")
	assert.Contains(t, short, "Enter all 4 digits")
	assert.Regexp(t, `<button[^>]*type="submit"[^>]*\sdisabled[\s>=]`, short, "off until every box is filled")

	long := player(t, templates.RenderPlayerView(withPoints, newPincode("20260315"), open()))
	assert.Equal(t, 8, strings.Count(long, `name="pincode"`))
	assert.Contains(t, long, "--per:4", "eight digits are two rows of four")

	odd := player(t, templates.RenderPlayerView(withPoints, newPincode("1234567"), open()))
	assert.Contains(t, odd, "--per:4", "seven wrap to four and three")
}

// A wrong guess says so in generic words, since the author wrote no hint, and only
// the answer shakes.
func TestPincode_WrongGuessSaysSoAndShakesOnlyWhenJustAnswered(t *testing.T) {
	attempt := &fakeState{data: json.RawMessage(`{"attempts":1,"guesses":["1234"]}`)}

	update := player(t, templates.RenderPlayerUpdate(withPoints, newPincode("4417"), attempt))
	assert.Contains(t, update, "That is not the right code. Check the digits and try again.")
	assert.Contains(t, update, "is-wrong")
	assert.Contains(t, update, `hx-swap-oob="true"`)

	reload := player(t, templates.RenderPlayerView(withPoints, newPincode("4417"), attempt))
	assert.NotContains(t, reload, "That is not the right code")
	assert.NotContains(t, reload, "is-wrong")
}

func TestPincode_SolvedShowsTheDigitsAndWhatItUnlocked(t *testing.T) {
	html := player(
		t,
		templates.RenderPlayerView(
			withPoints,
			newPincode("4417"),
			&fakeState{complete: true, points: 2, data: json.RawMessage(`{"attempts":1}`)},
		),
	)

	assert.Contains(t, html, `class="qb-digits ok"`)
	assert.Contains(t, html, `value="4"`)
	assert.Contains(t, html, `class="q-unlocked prose"`)
	assert.NotContains(t, html, "<form")
	assert.Contains(t, html, "+2 pts")
}

// A clue is the one block whose points can be a cost, and it asks twice.
func TestClue_CostsAndRevealsInPlainText(t *testing.T) {
	clue := &blocks.ClueBlock{
		BaseBlock:       blocks.BaseBlock{ID: "clue", Points: -2},
		DescriptionText: "Stuck?",
		ClueText:        "Look left.",
		ButtonLabel:     "Show me",
	}

	html := player(t, templates.RenderPlayerView(withPoints, clue, open()))
	assert.Contains(t, html, "-2 pts")
	assert.Contains(t, html, "Revealing it takes 2 points.")
	assert.Contains(t, html, "Show me")
	assert.Contains(t, html, "Are you sure? Tap again", "the second tap is asked for")
	assert.NotContains(t, html, "Look left.", "not given away before it is asked for")

	done := player(
		t,
		templates.RenderPlayerView(
			withPoints,
			clue,
			&fakeState{complete: true, points: -2, data: json.RawMessage(`{}`)},
		),
	)
	assert.Contains(t, done, "Look left.")
	assert.Contains(t, done, "is-lit")
	assert.Contains(t, done, "-2 pts", "what it cost, in the pill")
	assert.NotContains(t, done, "<form")
}

// The scan card says what to do, and a wrong code says so without shouting.
func TestScan_HintsThenSaysWrongThenIsSolved(t *testing.T) {
	scan := &blocks.ScanBlock{BaseBlock: blocks.BaseBlock{ID: "scan", Points: 3}, Prompt: "Find the code.", AllowTyping: true}

	first := player(t, templates.RenderPlayerView(withPoints, scan, open()))
	assert.Contains(t, first, "Scan it, or type the code.")
	assert.NotContains(t, first, "not the right code")

	wrong := player(t, templates.RenderPlayerUpdate(withPoints, scan, open()))
	assert.Contains(t, wrong, "That is not the right code. Have another look.")
	assert.Contains(t, wrong, "is-wrong")

	solved := player(
		t,
		templates.RenderPlayerView(
			withPoints,
			scan,
			&fakeState{complete: true, points: 3, data: json.RawMessage(`{}`)},
		),
	)
	assert.Contains(t, solved, "Code scanned.")
	assert.Contains(t, solved, `class="qb-msg ok"`, "a message the block gives, not an answer")
	assert.NotContains(t, solved, "<form")
}

// The first save can be unchanged, but changing a name already set needs a change.
func TestTeamName_FirstSaveIsOpenAndAnUpdateWaitsForAChange(t *testing.T) {
	block := blocks.TeamNameChangerBlock{BaseBlock: blocks.BaseBlock{ID: "name"}}

	var form bytes.Buffer
	require.NoError(t, templates.TeamNameChangerForm(block, "The Otters").Render(context.Background(), &form))
	assert.Contains(t, form.String(), `value="The Otters"`)
	assert.Contains(t, form.String(), "Set your team name!")
	assert.NotRegexp(
		t,
		`<button[^>]*type="submit"[^>]*\sdisabled[\s>=]`,
		form.String(),
		"a player who likes the name given can keep it",
	)

	var change bytes.Buffer
	require.NoError(
		t,
		templates.TeamNameChangerComplete(block, "The Otters", true).Render(context.Background(), &change),
	)
	assert.Regexp(
		t,
		`<button[^>]*type="submit"[^>]*\sdisabled[\s>=]`,
		change.String(),
		"nothing to save until the name is different",
	)
	assert.Contains(t, change.String(), "Update")
	assert.Contains(t, change.String(), "qb-chip earned")

	var fixed bytes.Buffer
	require.NoError(
		t,
		templates.TeamNameChangerComplete(block, "The Otters", false).Render(context.Background(), &fixed),
	)
	assert.Contains(t, fixed.String(), `<div class="qb-name">The Otters</div>`)
	assert.NotContains(t, fixed.String(), "<form")
}

func newFreeText() *blocks.FreeTextBlock {
	return &blocks.FreeTextBlock{BaseBlock: blocks.BaseBlock{ID: "words", Points: 5}, Prompt: "What do you notice?"}
}

// Free text is not final: what was sent comes back as a quote with a way to edit it,
// and saving needs a change.
func TestFreeText_QuoteWithAnEditFormBehindIt(t *testing.T) {
	first := player(t, templates.RenderPlayerView(withPoints, newFreeText(), open()))
	assert.Contains(t, first, "There is no wrong answer.")
	assert.Contains(t, first, "Type your answer here…")
	assert.Regexp(t, `<button[^>]*type="submit"[^>]*\sdisabled[\s>=]`, first, "off until something is written")
	assert.NotContains(t, first, "Edit answer")

	sent := &fakeState{complete: true, points: 5, data: json.RawMessage(`{"response":"He looks like my uncle"}`)}
	done := player(t, templates.RenderPlayerView(withPoints, newFreeText(), sent))
	assert.Contains(t, done, `<blockquote class="qb-quote">He looks like my uncle</blockquote>`)
	assert.Contains(t, done, "Edit answer")
	assert.Contains(t, done, ">He looks like my uncle</textarea>", "the edit form starts from what was sent")
	assert.Contains(t, done, "Save changes")
	assert.Regexp(
		t,
		`<form[^>]*ft-edit-words[^>]*class="qb-form hidden"|<form[^>]*class="qb-form hidden"[^>]*ft-edit-words`,
		done,
		"hidden until asked for",
	)
	assert.Contains(t, done, "+5 pts")
}

func TestRating_OneRadioPerStarThenLockedStars(t *testing.T) {
	rating := &blocks.RatingBlock{BaseBlock: blocks.BaseBlock{ID: "stars"}, Prompt: "How grumpy?"}

	html := player(t, templates.RenderPlayerView(withPoints, rating, open()))
	assert.Equal(t, 5, strings.Count(html, `type="radio" name="rating"`))
	assert.Contains(t, html, "Pick a star rating")
	assert.Regexp(t, `<button[^>]*type="submit"[^>]*\sdisabled[\s>=]`, html)

	done := player(
		t,
		templates.RenderPlayerView(
			withPoints,
			rating,
			&fakeState{complete: true, data: json.RawMessage(`{"rating":4}`)},
		),
	)
	assert.Equal(t, 4, strings.Count(done, `class="qb-star on"`), "four stars are filled")
	assert.Equal(t, 1, strings.Count(done, `class="qb-star"`), "and one is not")
	assert.Contains(t, done, "You gave 4 out of 5 stars")
	assert.NotContains(t, done, `type="radio"`)
	assert.Contains(t, done, "Thanks for rating.")
}

// The password has no hint field, so a miss says so in generic words.
func TestPassword_GenericMissThenASolvedRow(t *testing.T) {
	pw := &blocks.PasswordBlock{
		BaseBlock:       blocks.BaseBlock{ID: "pw", Points: 2},
		Prompt:          "What is the password?",
		Answer:          "swordfish",
		UnlockedContent: "The drawer opens.",
	}

	first := player(t, templates.RenderPlayerView(withPoints, pw, open()))
	assert.Contains(t, first, "Type the password")
	assert.NotContains(t, first, "not the password")

	miss := player(t, templates.RenderPlayerUpdate(withPoints, pw, open()))
	assert.Contains(t, miss, "That is not the password. Have another look and try again.")
	assert.Contains(t, miss, "is-wrong")
	assert.NotContains(t, miss, "plaque", "it cannot know what the author's clue was")

	solved := player(
		t,
		templates.RenderPlayerView(withPoints, pw, &fakeState{complete: true, points: 2, data: json.RawMessage(`{}`)}),
	)
	assert.Contains(t, solved, "swordfish")
	assert.Contains(t, solved, `class="qb-opt locked is-ok"`, "an accepted answer looks like a right answer")
	assert.Contains(t, solved, "That's right")
	assert.NotContains(t, solved, "qb-msg", "it is not a message")
	assert.Contains(t, solved, "The drawer opens.")
	assert.NotContains(t, solved, "<form")
}

func TestPhoto_ThumbnailsHaveRemoveButtonsUntilItIsFinished(t *testing.T) {
	photo := &blocks.PhotoBlock{
		BaseBlock: blocks.BaseBlock{ID: "snap", Points: 4},
		Prompt:    "Pose with the statue.",
		MaxImages: 2,
	}
	one := &fakeState{data: json.RawMessage(`{"images":["https://example.test/a.jpg"]}`)}

	html := player(t, templates.RenderPlayerView(withPoints, photo, one))
	assert.Contains(t, html, "Add up to 2.")
	assert.Contains(t, html, `aria-label="Remove this photo"`)
	assert.Contains(t, html, "Take a photo")
	assert.Contains(t, html, "(1 of 2)")
	assert.Contains(t, html, `id="fileInput-snap"`, "the upload script finds its input by this id")

	two := &fakeState{
		complete: true,
		points:   4,
		data:     json.RawMessage(`{"images":["https://example.test/a.jpg","https://example.test/b.jpg"]}`),
	}
	done := player(t, templates.RenderPlayerView(withPoints, photo, two))
	assert.Equal(t, 2, strings.Count(done, `alt="Uploaded photo"`))
	assert.NotContains(t, done, "Remove this photo")
	assert.NotContains(t, done, "Take a photo")
}

func newSorting(scheme string) *blocks.SortingBlock {
	return &blocks.SortingBlock{
		BaseBlock: blocks.BaseBlock{ID: "ages", Points: 6}, Content: "Oldest first.", ScoringScheme: scheme,
		Items: []blocks.SortingItem{
			{
				ID:          "a",
				Description: "Stone",
				Position:    1,
			},
			{ID: "b", Description: "Bronze", Position: 2},
			{ID: "c", Description: "Iron", Position: 3},
		},
	}
}

// Drag by the handle: no arrow buttons, and a wrong order says how many are right.
func TestSorting_HandleNotArrowsAndACountOfWhatIsRight(t *testing.T) {
	retry := newSorting(blocks.RetryUntilCorrect)
	start := player(
		t,
		templates.RenderPlayerView(
			withPoints,
			retry,
			&fakeState{data: json.RawMessage(`{"shuffle_order":["c","a","b"]}`)},
		),
	)
	assert.Equal(t, 3, strings.Count(start, `class="qb-grip"`))
	assert.NotContains(t, start, "Move up")
	assert.NotContains(t, start, "Move down")
	assert.Contains(t, start, "Drag the handle to move one.")
	assert.Contains(t, start, "Check order")
	assert.Contains(t, start, `hx-swap="outerHTML"`)

	wrong := &fakeState{
		data: json.RawMessage(
			`{"shuffle_order":["c","a","b"],"player_order":["a","c","b"],"attempts":1,"is_correct":false}`,
		),
	}
	update := player(t, templates.RenderPlayerUpdate(withPoints, retry, wrong))
	assert.Contains(t, update, "Not quite. 1 of 3 are in the right place.")
	assert.Contains(t, update, "is-wrong")

	right := &fakeState{
		complete: true,
		points:   6,
		data: json.RawMessage(
			`{"shuffle_order":["c","a","b"],"player_order":["a","b","c"],"attempts":2,"is_correct":true}`,
		),
	}
	done := player(t, templates.RenderPlayerView(withPoints, retry, right))
	assert.Equal(t, 3, strings.Count(done, "is-ok"))
	assert.NotContains(t, done, `class="qb-grip"`, "nothing to drag once it is right")
	assert.Contains(t, done, "+6 pts")
}

func newGeofence(mode blocks.GeofenceMapMode) *blocks.GeofenceBlock {
	return &blocks.GeofenceBlock{
		BaseBlock: blocks.BaseBlock{ID: "geo", Points: 5},
		Prompt:    "Stand beside the statue.",
		MapMode:   mode,
	}
}

// The three modes differ only in what the card shows.
func TestGeofence_ModesChooseTheMapAndTheHint(t *testing.T) {
	bare := player(t, templates.RenderPlayerView(withPoints, newGeofence(blocks.GeofenceMapButton), open()))
	assert.Contains(t, bare, "Your phone checks where you are.")
	assert.NotContains(t, bare, "geofence-player-map-geo", "a bare button has no map")
	assert.Contains(t, bare, "Check where I am")

	mapOnly := player(t, templates.RenderPlayerView(withPoints, newGeofence(blocks.GeofenceMapShow), open()))
	assert.Contains(t, mapOnly, `id="geofence-player-map-geo"`)
	assert.Contains(t, mapOnly, "The map shows where you are, not where to go.")

	area := player(t, templates.RenderPlayerView(withPoints, newGeofence(blocks.GeofenceMapArea), open()))
	assert.Contains(t, area, "The marked area on the map is where you need to be.")

	solved := player(
		t,
		templates.RenderPlayerView(
			withPoints,
			newGeofence(blocks.GeofenceMapButton),
			&fakeState{complete: true, points: 5, data: json.RawMessage(`{}`)},
		),
	)
	assert.Contains(t, solved, "You made it.")
	assert.Contains(t, solved, `class="qb-msg ok"`)
	assert.NotContains(t, solved, "geofence-live-geo")
}

// A miss is answered in rounded steps that widen with distance, so a few readings
// from false positions cannot pin a hidden shape down.
func TestGeofence_MissSaysRoughlyHowFarInCoarseSteps(t *testing.T) {
	cases := map[string]string{
		"12":    "You are very close.",
		"60":    "You are about 50 m away.",
		"130":   "You are about 125 m away.",
		"320":   "You are about 300 m away.",
		"1250":  "You are about 1300 m away.",
		"3200":  "You are about 3.0 km away.",
		"118.7": "You are about 125 m away.",
	}
	for metres, want := range cases {
		state := &fakeState{data: json.RawMessage(`{"attempts":1,"metres_away":` + metres + `}`)}
		html := player(t, templates.RenderPlayerUpdate(withPoints, newGeofence(blocks.GeofenceMapButton), state))
		assert.Contains(t, html, want, metres+" m")
	}

	// the exact figure is never on the page
	state := &fakeState{data: json.RawMessage(`{"attempts":1,"metres_away":118.7}`)}
	html := player(t, templates.RenderPlayerUpdate(withPoints, newGeofence(blocks.GeofenceMapButton), state))
	assert.NotContains(t, html, "118")

	// inside, there is nothing to say about distance
	inside := &fakeState{complete: true, points: 5, data: json.RawMessage(`{"attempts":2,"metres_away":0}`)}
	done := player(t, templates.RenderPlayerUpdate(withPoints, newGeofence(blocks.GeofenceMapButton), inside))
	assert.NotContains(t, done, "away.")
	assert.Contains(t, done, "+5 pts")
	assert.Contains(t, done, `hx-swap-oob="true"`)
}

// A saved order from before the items had ids names nothing the block has now, so
// the block starts again rather than showing an empty list.
func TestSorting_StaleOrderFromBeforeItemsHadIDsStartsAgain(t *testing.T) {
	stale := &fakeState{
		data: json.RawMessage(`{"shuffle_order":["","","",""],"player_order":["","","",""],"attempts":1}`),
	}

	html := player(t, templates.RenderPlayerView(withPoints, newSorting(blocks.RetryUntilCorrect), stale))

	assert.Equal(t, 3, strings.Count(html, `class="qb-grip"`), "every item is drawn")
	for _, text := range []string{">Stone<", ">Bronze<", ">Iron<"} {
		assert.Contains(t, html, text)
	}
}

// Three items with distinct ids are three different rows.
func TestSorting_EachItemIsItsOwnRow(t *testing.T) {
	html := player(t, templates.RenderPlayerView(withPoints, newSorting(blocks.RetryUntilCorrect), open()))

	for _, text := range []string{">Stone<", ">Bronze<", ">Iron<"} {
		assert.Equal(t, 1, strings.Count(html, text), text)
	}
}

// Every rating is out of five, in one row: no scale to choose and nothing to wrap.
func TestRating_AlwaysFiveStarsInOneRow(t *testing.T) {
	rating := &blocks.RatingBlock{BaseBlock: blocks.BaseBlock{ID: "stars"}, Prompt: "Rate it."}

	html := player(t, templates.RenderPlayerView(withPoints, rating, open()))

	assert.Equal(t, 5, strings.Count(html, `type="radio" name="rating"`))
	assert.NotContains(t, html, "--per:", "the row is always five, so nothing sets a row length")
	assert.NotContains(t, html, "Tap a star", "no hint: it is plain enough")
}

// A quiz answered right shows the right options as picked even when the saved
// selection names nothing the block has now, as one saved before items had ids does.
func TestQuiz_AnsweredRightShowsTheRightOptionsWhateverWasSaved(t *testing.T) {
	quiz := &blocks.QuizBlock{
		BaseBlock:    blocks.BaseBlock{ID: "fruit", Points: 3},
		Question:     "Which is a fruit?",
		RetryEnabled: true,
		Options: []blocks.QuizOption{
			{ID: "option_0", Text: "Apple", IsCorrect: true},
			{ID: "option_1", Text: "Carrot"},
		},
	}
	stale := &fakeState{
		complete: true,
		points:   3,
		data:     json.RawMessage(`{"selected_options":[""],"attempts":1,"is_correct":true}`),
	}

	html := player(t, templates.RenderPlayerView(withPoints, quiz, stale))

	assert.Equal(t, 1, strings.Count(html, "is-ok"), "the right option is shown as picked")
	assert.NotContains(t, html, "is-miss", "it was not missed: the answer was right")
	assert.NotContains(t, html, "is-no")
}

// Done is green wherever a finished block shows what was chosen.
func TestChoice_FinishedRowsUseTheSameMarkAsEveryOtherFinishedRow(t *testing.T) {
	done := player(
		t,
		templates.RenderPlayerView(
			withPoints,
			newChoice(false),
			&fakeState{complete: true, data: json.RawMessage(`{"chosen":["right"]}`)},
		),
	)

	assert.Contains(t, done, `class="qb-opt locked chosen"`)
}
