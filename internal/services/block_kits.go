package services

import (
	"cmp"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"maps"
	"math/big"
	"strconv"

	"github.com/nathanhollows/Rapua/v8/blocks"
)

// ErrUnknownBlockKit is returned for a kit name with no steps.
var ErrUnknownBlockKit = errors.New("unknown section template")

// ErrPageNotEmpty is returned when a page template is asked for on a page that
// already has blocks. A page template builds a whole page, so on a page with
// content it would add a second start button or a second heading.
var ErrPageNotEmpty = errors.New("this page already has blocks")

// DefaultKitPoints is large enough to be seen to count and easy to change.
const DefaultKitPoints = 10

// BlockKit is a section template: blocks pre-arranged for a common task.
type BlockKit string

const (
	KitScan     BlockKit = "scan"
	KitLocation BlockKit = "location"
	KitQuiz     BlockKit = "quiz"

	// Page kits fill the start and finish pages, which belong to no objective.
	// They are what an empty page offers, so a quest that arrives without
	// content (an import with `start: []`) is a few clicks from a playable one.
	KitStartScavenger BlockKit = "start_scavenger"
	KitStartWelcome   BlockKit = "start_welcome"
	KitStartRules     BlockKit = "start_rules"
	KitFinishCongrats BlockKit = "finish_congrats"
	KitFinishSurvey   BlockKit = "finish_survey"
)

// pageContext is the page a page kit is for.
func (k BlockKit) pageContext() blocks.BlockContext {
	if k == KitFinishCongrats || k == KitFinishSurvey {
		return blocks.ContextFinish
	}
	return blocks.ContextStart
}

// IsPageKit reports whether a kit fills the start or finish page.
func (k BlockKit) IsPageKit() bool {
	switch k {
	case KitStartScavenger, KitStartWelcome, KitStartRules, KitFinishCongrats, KitFinishSurvey:
		return true
	case KitScan, KitLocation, KitQuiz:
		return false
	}
	return false
}

// The block types kits are made of. Kit names are what a player is asked to do,
// and only happen to match.
const (
	kitTypeText     = "text"
	kitTypeScan     = "scan"
	kitTypeGeofence = "geofence"
	kitTypeQuiz     = "quiz"

	kitTypeHeader      = "header"
	kitTypeGameStatus  = "game_status"
	kitTypeDivider     = "divider"
	kitTypeTeamName    = "team_name"
	kitTypeStartButton = "start_button"
	kitTypeRating      = "rating"
	kitTypeFreeText    = "free_text"
)

// kitTitleToken stands in for the quest's name in a page kit's header, which
// only the caller knows.
const kitTitleToken = "{{quest name}}"

// kitDefaultTitle is the header when the quest has no name to give.
const kitDefaultTitle = "Welcome"

type kitStep struct {
	blockType string
	data      map[string][]string
}

const (
	scanKitHelp = "Find the QR code hidden nearby and scan it to prove you were here."
	quizKitHelp = "Look around you, then answer the questions below."
)

const (
	startWelcomeHelp = "Read this first. When you are ready, press the button below to begin."

	startRulesHelp = "- Stay together, stay aware of your surroundings, and keep to public paths and open areas.\n" +
		"- Do not climb, enter closed areas, or touch anything the clues do not ask you to.\n" +
		"- Look after the place and the people in it. Leave everything as you found it.\n" +
		"- Keep your phone charged and your sound low. You will need it to scan, answer and take photos.\n" +
		"- You can stop at any time. If you need help, ask a member of staff."

	finishSurveyHelp = "Thanks for playing. Tell us how it went. It only takes a moment, " +
		"and it helps make the next game better."
)

func contentOf(text string) map[string][]string {
	return map[string][]string{"content": {text}}
}

// kitSteps is a function, not a table, so each scan kit gets a code of its own.
func kitSteps(kit BlockKit) ([]kitStep, bool) {
	switch kit {
	case KitScan:
		return []kitStep{
			{blockType: kitTypeText, data: contentOf(scanKitHelp)},
			{blockType: kitTypeScan, data: map[string][]string{"code": {newScanCode()}}},
		}, true
	case KitLocation:
		return []kitStep{{blockType: kitTypeGeofence}}, true
	case KitQuiz:
		return []kitStep{
			{blockType: kitTypeText, data: contentOf(quizKitHelp)},
			{blockType: kitTypeQuiz},
			{blockType: kitTypeQuiz},
		}, true
	case KitStartScavenger:
		return append(startOpening(),
			kitStep{blockType: kitTypeDivider, data: map[string][]string{"title": {"How to play"}}},
			kitStep{blockType: kitTypeText, data: contentOf(startInstructionsContent)},
			kitStep{blockType: kitTypeDivider, data: map[string][]string{"title": {"Team Info"}}},
			teamNameStep(), startButtonStep(),
		), true
	case KitStartWelcome:
		return append(startOpening(),
			kitStep{blockType: kitTypeText, data: contentOf(startWelcomeHelp)},
			startButtonStep(),
		), true
	case KitStartRules:
		return append(startOpening(),
			kitStep{blockType: kitTypeDivider, data: map[string][]string{"title": {"Before you begin"}}},
			kitStep{blockType: kitTypeText, data: contentOf(startRulesHelp)},
			teamNameStep(), startButtonStep(),
		), true
	case KitFinishCongrats:
		return finishOpening(finishCongratulationsContent), true
	case KitFinishSurvey:
		return append(finishOpening(finishSurveyHelp),
			kitStep{blockType: kitTypeRating, data: map[string][]string{
				"prompt": {"How much did you enjoy the game?"},
			}},
			kitStep{blockType: kitTypeFreeText, data: map[string][]string{
				"prompt":      {"What was the best part, and what would you change?"},
				"placeholder": {"Tell us anything. Short is fine."},
			}},
		), true
	}
	return nil, false
}

// startOpening is how every start page begins: what the game is called, and
// whether it is open yet.
func startOpening() []kitStep {
	return []kitStep{
		{blockType: kitTypeHeader, data: map[string][]string{
			"icon": {"map-pin-check-inside"}, "title": {kitTitleToken}, "size": {"large"},
		}},
		{blockType: kitTypeGameStatus, data: map[string][]string{
			"closed_message":    {"This game is not yet open."},
			"scheduled_message": {"This game will start soon."},
			"show_countdown":    {"true"},
		}},
	}
}

func finishOpening(message string) []kitStep {
	return []kitStep{
		{blockType: kitTypeHeader, data: map[string][]string{
			"icon": {"party-popper"}, "title": {"Congratulations!"}, "size": {"large"},
		}},
		{blockType: kitTypeText, data: contentOf(message)},
	}
}

// teamNameStep and startButtonStep spell out every field, because a checkbox a
// block never hears about is read as unchecked.
func teamNameStep() kitStep {
	return kitStep{blockType: kitTypeTeamName, data: map[string][]string{
		"prompt": {"Set your team name"}, "allow_changing": {"true"},
	}}
}

func startButtonStep() kitStep {
	return kitStep{blockType: kitTypeStartButton, data: map[string][]string{
		"scheduled_text": {"Game starts soon..."}, "active_text": {"Start Game"}, "style": {"primary"},
	}}
}

// scanCodeAlphabet drops i, l, o, 0 and 1: they read as each other when printed.
const scanCodeAlphabet = "abcdefghjkmnpqrstuvwxyz23456789"

const (
	scanCodeGroupLen = 4
	scanCodeGroups   = 2
)

// newScanCode matches the form the editor's "Mint a QR" makes.
func newScanCode() string {
	out := make([]byte, 0, scanCodeGroupLen*scanCodeGroups+scanCodeGroups-1)
	for i := range scanCodeGroupLen * scanCodeGroups {
		if i > 0 && i%scanCodeGroupLen == 0 {
			out = append(out, '-')
		}
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(scanCodeAlphabet))))
		if err != nil {
			// Carrying on without randomness would mint guessable codes.
			panic(fmt.Errorf("making a scan code: %w", err))
		}
		out = append(out, scanCodeAlphabet[n.Int64()])
	}
	return string(out)
}

// CreateKit adds a kit's blocks all or none. Pass points 0 for an unscored game.
func (s *BlockService) CreateKit(
	ctx context.Context, ownerID string, blockContext blocks.BlockContext, kit BlockKit, points int,
) ([]blocks.Block, error) {
	return s.createKit(ctx, ownerID, blockContext, kit, points, "")
}

// CreatePageKit fills a start or finish page, naming the game in its header.
// Page blocks carry no points, so a survey cannot skew the leaderboard.
func (s *BlockService) CreatePageKit(
	ctx context.Context, questID string, blockContext blocks.BlockContext, kit BlockKit, questName string,
) ([]blocks.Block, error) {
	if !kit.IsPageKit() {
		return nil, fmt.Errorf("%w: %q is not a page template", ErrUnknownBlockKit, kit)
	}
	// Header and text are valid on both pages, so the blocks alone would not stop
	// a finish kit landing on the start page.
	if want := kit.pageContext(); blockContext != want {
		return nil, fmt.Errorf("%w: %q belongs on the %q page", ErrBlockNotValidForContext, kit, want)
	}
	// The editor only offers these on an empty page, but a request can be made by
	// hand, and the page would be left with two of everything.
	existing, err := s.FindByOwnerIDAndContext(ctx, questID, blockContext)
	if err != nil {
		return nil, fmt.Errorf("checking the page is empty: %w", err)
	}
	if len(existing) > 0 {
		return nil, fmt.Errorf("%w: %q needs an empty %q page", ErrPageNotEmpty, kit, blockContext)
	}
	return s.createKit(ctx, questID, blockContext, kit, 0, questName)
}

func (s *BlockService) createKit(
	ctx context.Context, ownerID string, blockContext blocks.BlockContext, kit BlockKit, points int, title string,
) ([]blocks.Block, error) {
	steps, ok := kitSteps(kit)
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownBlockKit, kit)
	}
	if ownerID == "" {
		return nil, errors.New("ownerID cannot be empty")
	}

	list := make([]blocks.Block, 0, len(steps))
	for _, step := range steps {
		if !blocks.CanBlockBeUsedInContext(step.blockType, blockContext) {
			return nil, fmt.Errorf(
				"%w: block type %q cannot be used in context %q",
				ErrBlockNotValidForContext, step.blockType, blockContext,
			)
		}
		block, err := blocks.CreateFromBaseBlock(blocks.BaseBlock{Type: step.blockType, OwnerID: ownerID})
		if err != nil {
			return nil, fmt.Errorf("making a %s block: %w", step.blockType, err)
		}
		data := map[string][]string{}
		maps.Copy(data, step.data)
		if len(data["title"]) == 1 && data["title"][0] == kitTitleToken {
			data["title"] = []string{cmp.Or(title, kitDefaultTitle)}
		}
		if block.RequiresValidation() && points > 0 {
			data["points"] = []string{strconv.Itoa(points)}
		}
		if len(data) > 0 {
			if err = block.UpdateBlockData(data); err != nil {
				return nil, fmt.Errorf("filling in a %s block: %w", step.blockType, err)
			}
		}
		list = append(list, block)
	}
	return s.blockRepo.CreateMany(ctx, list, ownerID, blockContext)
}
