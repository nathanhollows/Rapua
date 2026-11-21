package services

import (
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

// DefaultKitPoints is large enough to be seen to count and easy to change.
const DefaultKitPoints = 10

// BlockKit is a section template: blocks pre-arranged for a common task.
type BlockKit string

const (
	KitScan     BlockKit = "scan"
	KitLocation BlockKit = "location"
	KitQuiz     BlockKit = "quiz"
)

// The block types kits are made of. Kit names are what a player is asked to do,
// and only happen to match.
const (
	kitTypeText     = "text"
	kitTypeScan     = "scan"
	kitTypeGeofence = "geofence"
	kitTypeQuiz     = "quiz"
)

type kitStep struct {
	blockType string
	data      map[string][]string
}

const (
	scanKitHelp = "Find the QR code hidden nearby and scan it to prove you were here."
	quizKitHelp = "Look around you, then answer the questions below."
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
			// "generate" so there is a printable image the moment the block exists.
			{blockType: kitTypeScan, data: map[string][]string{"code": {newScanCode()}, "generate": {"0"}}},
		}, true
	case KitLocation:
		return []kitStep{{blockType: kitTypeGeofence}}, true
	case KitQuiz:
		return []kitStep{
			{blockType: kitTypeText, data: contentOf(quizKitHelp)},
			{blockType: kitTypeQuiz},
			{blockType: kitTypeQuiz},
		}, true
	}
	return nil, false
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
