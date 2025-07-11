package services

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"

	"github.com/nathanhollows/Rapua/v3/db"
	"github.com/nathanhollows/Rapua/v3/models"
	"github.com/nathanhollows/Rapua/v3/repositories"
)

type gameManagerService struct {
	transactor           db.Transactor
	locationService      LocationService
	userService          UserService
	teamService          TeamService
	markerRepo           repositories.MarkerRepository
	clueRepo             repositories.ClueRepository
	instanceRepo         repositories.InstanceRepository
	instanceSettingsRepo repositories.InstanceSettingsRepository
	instanceService      InstanceService
}

// TODO: Split this service into smaller services.
type GameManagerService interface {
	// Team & Location Management
	LoadTeams(ctx context.Context, teams *[]models.Team) error
	CreateLocation(ctx context.Context, user *models.User, data map[string]string) (models.Location, error)
	SaveLocation(ctx context.Context, location *models.Location, lat, lng, name string) error

	// Marker & Validation
	ValidateLocationMarker(user *models.User, id string) bool
	ValidateLocationID(user *models.User, id string) bool

	// Settings & Utilities
	UpdateSettings(ctx context.Context, settings *models.InstanceSettings, form url.Values) error
	DismissQuickstart(ctx context.Context, instanceID string) error
}

func NewGameManagerService(
	transactor db.Transactor,
	locationService LocationService,
	userService UserService,
	teamService TeamService,
	markerRepo repositories.MarkerRepository,
	clueRepo repositories.ClueRepository,
	instanceRepo repositories.InstanceRepository,
	instanceSettingsRepo repositories.InstanceSettingsRepository,
	instanceService InstanceService,
) GameManagerService {
	return &gameManagerService{
		transactor:           transactor,
		locationService:      locationService,
		userService:          userService,
		teamService:          teamService,
		markerRepo:           markerRepo,
		clueRepo:             clueRepo,
		instanceRepo:         instanceRepo,
		instanceSettingsRepo: instanceSettingsRepo,
		instanceService:      instanceService,
	}
}

func (s *gameManagerService) LoadTeams(ctx context.Context, teams *[]models.Team) error {
	for i := range *teams {
		err := s.teamService.LoadRelation(ctx, &(*teams)[i], "Scans")
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *gameManagerService) SaveLocation(ctx context.Context, location *models.Location, lat, lng, name string) error {
	if lat == "" || lng == "" {
		return errors.New("latitude and longitude are required")
	}

	latFloat, err := strconv.ParseFloat(lat, 64)
	if err != nil {
		return err
	}
	lngFloat, err := strconv.ParseFloat(lng, 64)
	if err != nil {
		return err
	}

	err = s.locationService.UpdateCoords(ctx, location, latFloat, lngFloat)
	if err != nil {
		return fmt.Errorf("updating location coordinates: %w", err)
	}

	err = s.locationService.UpdateName(ctx, location, name)
	if err != nil {
		return fmt.Errorf("updating location name: %w", err)
	}

	return nil
}

func (s *gameManagerService) CreateLocation(
	ctx context.Context,
	user *models.User,
	data map[string]string,
) (models.Location, error) {
	// Extract input values
	name := data["name"]
	latStr := data["latitude"]
	lngStr := data["longitude"]
	pointsStr := data["points"]
	markerCode := data["marker"]

	var (
		lat float64
		lng float64
		err error
	)

	// Parse latitude / longitude if provided
	if latStr != "" && lngStr != "" {
		lat, err = strconv.ParseFloat(latStr, 64)
		if err != nil {
			return models.Location{}, fmt.Errorf("invalid latitude: %w", err)
		}
		lng, err = strconv.ParseFloat(lngStr, 64)
		if err != nil {
			return models.Location{}, fmt.Errorf("invalid longitude: %w", err)
		}
	}

	// Parse points (default = 10)
	points := 10
	if pointsStr != "" {
		points, err = strconv.Atoi(pointsStr)
		if err != nil {
			return models.Location{}, fmt.Errorf("invalid points value: %w", err)
		}
	}

	// If no marker code given, create a location directly
	if markerCode == "" {
		return s.locationService.CreateLocation(
			ctx,
			user.CurrentInstanceID,
			name,
			lat,
			lng,
			points,
		)
	}

	// Otherwise, verify that marker code exists in markers not already in the user’s current instance
	instanceIDs, err := s.instanceService.FindInstanceIDsForUser(ctx, user.ID)
	if err != nil {
		return models.Location{}, fmt.Errorf("getting instance IDs for user: %w", err)
	}

	markers, err := s.markerRepo.FindNotInInstance(ctx, user.CurrentInstanceID, instanceIDs)
	if err != nil {
		return models.Location{}, fmt.Errorf("finding markers not in instance: %w", err)
	}

	// Check if the requested marker code exists among returned markers
	markerExists := false
	for _, m := range markers {
		if m.Code == markerCode {
			markerExists = true
			break
		}
	}
	if !markerExists {
		return models.Location{}, errors.New("marker does not exist")
	}

	// Finally, create location from marker
	return s.locationService.CreateLocationFromMarker(
		ctx,
		user.CurrentInstanceID,
		name,
		points,
		markerCode,
	)
}

func (s *gameManagerService) ValidateLocationMarker(user *models.User, id string) bool {
	for _, loc := range user.CurrentInstance.Locations {
		if loc.MarkerID == id {
			return true
		}
	}
	return false
}

func (s *gameManagerService) ValidateLocationID(user *models.User, id string) bool {
	for _, loc := range user.CurrentInstance.Locations {
		if loc.ID == id {
			return true
		}
	}
	return false
}

// UpdateSettings parses the form values and updates the instance settings.
func (s *gameManagerService) UpdateSettings(ctx context.Context, settings *models.InstanceSettings, form url.Values) error {
	// Navigation mode
	navMode, err := models.ParseNavigationMode(form.Get("navigationMode"))
	if err != nil {
		return fmt.Errorf("parsing navigation mode: %w", err)
	}
	settings.NavigationMode = navMode

	// Completion method
	completionMethod, err := models.ParseCompletionMethod(form.Get("completionMethod"))
	if err != nil {
		return fmt.Errorf("parsing completion method: %w", err)
	}
	settings.CompletionMethod = completionMethod

	// Navigation method
	navMethod, err := models.ParseNavigationMethod(form.Get("navigationMethod"))
	if err != nil {
		return fmt.Errorf("parsing navigation method: %w", err)
	}
	settings.NavigationMethod = navMethod

	// Show team count
	showTeamCount := form.Has("showTeamCount")
	settings.ShowTeamCount = showTeamCount

	// Max locations
	maxLoc := form.Get("maxLocations")
	if maxLoc != "" {
		maxLocInt, err := strconv.Atoi(form.Get("maxLocations"))
		if err != nil {
			return fmt.Errorf("parsing max locations: %w", err)
		}
		settings.MaxNextLocations = maxLocInt
	}

	// Enable points
	enablePoints := form.Has("enablePoints")
	settings.EnablePoints = enablePoints

	// Enable Bonus Points
	enableBonusPoints := form.Has("enableBonusPoints")
	settings.EnableBonusPoints = enableBonusPoints

	// Save settings
	if err := s.instanceSettingsRepo.Update(ctx, settings); err != nil {
		return fmt.Errorf("updating settings: %w", err)
	}

	return nil
}

// DismissQuickstart marks the user as having dismissed the quickstart.
func (s *gameManagerService) DismissQuickstart(ctx context.Context, instanceID string) error {
	return s.instanceRepo.DismissQuickstart(ctx, instanceID)
}
