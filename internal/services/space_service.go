package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"

	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/models"
)

const (
	spacePayloadLength = 5
	// payloadAttempts bounds the retry loop when a generated payload collides.
	payloadAttempts = 10
)

var (
	ErrSpaceNameRequired = errors.New("space name is required")
	ErrInvalidSpaceKind  = errors.New("space kind must be point, area, or object")
	ErrPointNeedsCoords  = errors.New("a point space needs coordinates")
	ErrPointNotAPolygon  = errors.New("a point space cannot have a boundary — use an area instead")
	ErrAreaNeedsExtent   = errors.New("an area space needs a boundary, or coordinates with a radius")
	ErrObjectHasNoGeom   = errors.New("an object space cannot have geometry — it moves")
	ErrSlugTaken         = errors.New("another space in this quest already uses that slug")
	ErrPayloadTaken      = errors.New("another space already uses that scan code")
)

type SpaceInput struct {
	Name     string
	Slug     string
	Kind     game.SpaceKind
	Geometry *game.SpaceGeometry
	Payload  string
	Mobile   bool
}

type SpaceService struct {
	spaceRepo repositories.SpaceRepository
}

func NewSpaceService(spaceRepo repositories.SpaceRepository) *SpaceService {
	return &SpaceService{
		spaceRepo: spaceRepo,
	}
}

func (s *SpaceService) CreateSpace(ctx context.Context, questID string, in SpaceInput) (models.Space, error) {
	if questID == "" {
		return models.Space{}, errors.New("questID cannot be empty")
	}
	if err := ValidateSpaceInput(in); err != nil {
		return models.Space{}, err
	}

	slug, err := s.uniqueSlug(ctx, questID, in, "")
	if err != nil {
		return models.Space{}, err
	}

	payload, err := s.resolvePayload(ctx, in.Payload, "")
	if err != nil {
		return models.Space{}, err
	}

	space := models.Space{
		QuestID:  questID,
		Slug:     slug,
		Name:     strings.TrimSpace(in.Name),
		Kind:     in.Kind,
		Geometry: normaliseGeometry(in),
		Payload:  payload,
		Mobile:   in.Mobile,
	}
	if err := s.spaceRepo.Create(ctx, &space); err != nil {
		return models.Space{}, fmt.Errorf("creating space: %w", err)
	}
	return space, nil
}

func (s *SpaceService) UpdateSpace(ctx context.Context, spaceID string, in SpaceInput) (models.Space, error) {
	space, err := s.spaceRepo.GetByID(ctx, spaceID)
	if err != nil {
		return models.Space{}, fmt.Errorf("finding space: %w", err)
	}
	if vErr := ValidateSpaceInput(in); vErr != nil {
		return models.Space{}, vErr
	}

	slug, err := s.uniqueSlug(ctx, space.QuestID, in, space.ID)
	if err != nil {
		return models.Space{}, err
	}

	// Blank means "leave it alone", not "reissue": rotating the payload would
	// kill every QR already printed and every NFC tag already programmed.
	wanted := in.Payload
	if strings.TrimSpace(wanted) == "" {
		wanted = space.Payload
	}
	payload, err := s.resolvePayload(ctx, wanted, space.ID)
	if err != nil {
		return models.Space{}, err
	}

	space.Slug = slug
	space.Name = strings.TrimSpace(in.Name)
	space.Kind = in.Kind
	space.Geometry = normaliseGeometry(in)
	space.Payload = payload
	space.Mobile = in.Mobile

	if err := s.spaceRepo.Update(ctx, space); err != nil {
		return models.Space{}, fmt.Errorf("updating space: %w", err)
	}
	return *space, nil
}

func (s *SpaceService) GetSpaceByID(ctx context.Context, spaceID string) (models.Space, error) {
	space, err := s.spaceRepo.GetByID(ctx, spaceID)
	if err != nil {
		return models.Space{}, fmt.Errorf("getting space: %w", err)
	}
	return *space, nil
}

func (s *SpaceService) FindSpacesByQuest(ctx context.Context, questID string) ([]models.Space, error) {
	if questID == "" {
		return nil, errors.New("questID cannot be empty")
	}
	spaces, err := s.spaceRepo.FindByQuest(ctx, questID)
	if err != nil {
		return nil, fmt.Errorf("finding spaces: %w", err)
	}
	return spaces, nil
}

func (s *SpaceService) DeleteSpace(ctx context.Context, spaceID string) error {
	if spaceID == "" {
		return errors.New("spaceID cannot be empty")
	}
	if err := s.spaceRepo.Delete(ctx, spaceID); err != nil {
		return fmt.Errorf("deleting space: %w", err)
	}
	return nil
}

// ValidateSpaceInput judges the geometry against the kind. Well-formedness is
// game.SpaceGeometry's business.
func ValidateSpaceInput(in SpaceInput) error {
	if strings.TrimSpace(in.Name) == "" {
		return ErrSpaceNameRequired
	}
	if !in.Kind.Valid() {
		return ErrInvalidSpaceKind
	}

	if err := in.Geometry.Validate(); err != nil {
		return err
	}

	switch in.Kind {
	case game.SpaceKindPoint:
		return validatePointGeometry(in.Geometry)
	case game.SpaceKindArea:
		return validateAreaExtent(in.Geometry)
	case game.SpaceKindObject:
		if !in.Geometry.IsZero() {
			return ErrObjectHasNoGeom
		}
	}
	return nil
}

func validatePointGeometry(g *game.SpaceGeometry) error {
	if g.IsZero() || !g.HasCoordinates() {
		return ErrPointNeedsCoords
	}
	if g.Type == game.GeometryPolygon {
		return ErrPointNotAPolygon
	}
	return nil
}

// A bare centre is not an extent: every position would be equally inside it.
func validateAreaExtent(g *game.SpaceGeometry) error {
	if g.IsZero() {
		return ErrAreaNeedsExtent
	}
	if g.Type == game.GeometryPolygon {
		return nil
	}
	if g.HasCoordinates() && g.Radius > 0 {
		return nil
	}
	return ErrAreaNeedsExtent
}

// Drops geometry the kind cannot carry, so a point re-typed as an object does
// not keep stale coordinates.
func normaliseGeometry(in SpaceInput) *game.SpaceGeometry {
	if in.Kind == game.SpaceKindObject || in.Geometry.IsZero() {
		return nil
	}
	return in.Geometry
}

// excludeID is the space being updated, which may legitimately keep its own slug.
func (s *SpaceService) uniqueSlug(
	ctx context.Context,
	questID string,
	in SpaceInput,
	excludeID string,
) (string, error) {
	base := models.Slugify(in.Slug)
	if base == "" {
		base = models.Slugify(in.Name)
	}
	if base == "" {
		return "", ErrSpaceNameRequired
	}

	candidate := base
	for i := 2; ; i++ {
		existing, err := s.spaceRepo.GetByQuestAndSlug(ctx, questID, candidate)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return candidate, nil
		case err != nil:
			return "", fmt.Errorf("checking slug availability: %w", err)
		case existing.ID == excludeID:
			return candidate, nil
		}
		// An author who typed the slug deserves an error, not a silent rename.
		if in.Slug != "" {
			return "", ErrSlugTaken
		}
		candidate = base + "-" + strconv.Itoa(i)
	}
}

// A scan resolves without knowing the quest, so payloads must be unique across
// the whole table — author-supplied ones included. excludeID keeps a space's own.
func (s *SpaceService) resolvePayload(ctx context.Context, payload, excludeID string) (string, error) {
	if p := strings.ToUpper(strings.TrimSpace(payload)); p != "" {
		free, err := s.payloadIsFree(ctx, p, excludeID)
		if err != nil {
			return "", err
		}
		if !free {
			return "", ErrPayloadTaken
		}
		return p, nil
	}

	for range payloadAttempts {
		candidate := newSpacePayload(spacePayloadLength)
		free, err := s.payloadIsFree(ctx, candidate, excludeID)
		if err != nil {
			return "", err
		}
		if free {
			return candidate, nil
		}
	}
	return "", errors.New("could not generate a free space payload")
}

func (s *SpaceService) payloadIsFree(ctx context.Context, payload, excludeID string) (bool, error) {
	existing, err := s.spaceRepo.GetByPayload(ctx, payload)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return true, nil
	case err != nil:
		return false, fmt.Errorf("checking payload availability: %w", err)
	default:
		return existing.ID == excludeID, nil
	}
}

// Confusing letters such as I and L, O and Q have one pair removed.
func newSpacePayload(length int) string {
	symbols := []rune("ABCDEFGHJKLMNPRSTUVWXYZ")
	b := make([]rune, length)
	for i := range length {
		b[i] = symbols[rand.IntN(len(symbols))] //nolint:gosec // Scan payloads do not need cryptographic randomness
	}
	return string(b)
}
