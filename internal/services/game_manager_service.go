package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/nathanhollows/Rapua/db"
	"github.com/nathanhollows/Rapua/internal/flash"
	"github.com/nathanhollows/Rapua/models"
	"github.com/nathanhollows/Rapua/repositories"
	"github.com/uptrace/bun"
)

type GameManagerService struct {
	transactor           db.Transactor
	locationService      LocationService
	userService          UserService
	teamService          TeamService
	markerRepo           repositories.MarkerRepository
	clueRepo             repositories.ClueRepository
	instanceRepo         repositories.InstanceRepository
	instanceSettingsRepo repositories.InstanceSettingsRepository
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
) GameManagerService {
	return GameManagerService{
		transactor:           transactor,
		locationService:      locationService,
		userService:          userService,
		teamService:          teamService,
		markerRepo:           markerRepo,
		clueRepo:             clueRepo,
		instanceRepo:         instanceRepo,
		instanceSettingsRepo: instanceSettingsRepo,
	}
}

func (s *GameManagerService) CreateInstance(ctx context.Context, name string, user *models.User) (response ServiceResponse) {
	response = ServiceResponse{}
	response.Data = make(map[string]interface{})

	if name == "" {
		response.AddFlashMessage(*flash.NewError("Name is required"))
		response.Error = errors.New("name is required")
	}

	instance := &models.Instance{
		Name:   name,
		UserID: user.ID,
	}

	if err := s.instanceRepo.Create(ctx, instance); err != nil {
		response.AddFlashMessage(*flash.NewError("Error saving instance"))
		response.Error = fmt.Errorf("saving instance: %w", err)
		return response
	}

	user.CurrentInstanceID = instance.ID
	err := s.userService.UpdateUser(ctx, user)
	if err != nil {
		response.AddFlashMessage(*flash.NewError("Error updating user"))
		response.Error = fmt.Errorf("updating user: %w", err)
		return response
	}

	settings := &models.InstanceSettings{
		InstanceID: instance.ID,
	}
	if err := s.instanceSettingsRepo.Create(ctx, settings); err != nil {
		response.AddFlashMessage(*flash.NewError("Error saving settings"))
		response.Error = fmt.Errorf("saving settings: %w", err)
		return response
	}

	response.Data["instanceID"] = instance.ID
	return response
}

func (s *GameManagerService) SwitchInstance(ctx context.Context, user *models.User, instanceID string) (*models.Instance, error) {
	instance, err := s.instanceRepo.GetByID(ctx, instanceID)
	if err != nil {
		return nil, errors.New("instance not found")
	}

	// Make sure the user has permission to switch to this instance
	if instance.UserID != user.ID {
		return nil, errors.New("user does not have permission to switch to this instance")
	}

	user.CurrentInstanceID = instance.ID
	if s.userService == nil {
		return nil, errors.New("user service is nil")
	}
	err = s.userService.UpdateUser(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("updating user: %w", err)
	}

	return instance, nil
}

// Duplicate an instance
// This will create a new instance with the same name and locations
// The teams will not be duplicated
func (s *GameManagerService) DuplicateInstance(ctx context.Context, user *models.User, id, name string) (response ServiceResponse) {
	response = ServiceResponse{}
	oldInstance, err := s.instanceRepo.GetByID(ctx, id)
	if err != nil {
		response.AddFlashMessage(*flash.NewError("Instance not found"))
		response.Error = fmt.Errorf("finding instance: %w", err)
		return response
	}

	locations, err := s.locationService.FindByInstance(ctx, oldInstance.ID)
	if err != nil {
		response.AddFlashMessage(*flash.NewError("Error finding locations"))
		response.Error = fmt.Errorf("finding locations: %w", err)
		return response
	}

	newInstance := &models.Instance{
		Name:   name,
		UserID: user.ID,
	}

	if err := s.instanceRepo.Create(ctx, newInstance); err != nil {
		response.AddFlashMessage(*flash.NewError("Error saving new instance"))
		response.Error = fmt.Errorf("saving new instance: %w", err)
		return response
	}

	// Copy locations
	for _, location := range locations {
		_, err := s.locationService.DuplicateLocation(ctx, &location, newInstance.ID)
		if err != nil {
			response.AddFlashMessage(*flash.NewError("Error saving location: " + location.Name))
			response.Error = fmt.Errorf("saving location: %w", err)
			return response
		}
	}

	// Copy settings
	settings := oldInstance.Settings
	settings.InstanceID = newInstance.ID
	if err := s.instanceSettingsRepo.Create(ctx, &settings); err != nil {
		response.AddFlashMessage(*flash.NewError("Error saving settings"))
		response.Error = fmt.Errorf("saving settings: %w", err)
		return response
	}

	response.Data = make(map[string]interface{})
	response.Data["instanceID"] = newInstance.ID
	return response
}

func (s *GameManagerService) LoadTeams(ctx context.Context, teams *[]models.Team) error {
	for i := range *teams {
		err := s.teamService.LoadRelation(ctx, &(*teams)[i], "Scans")
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *GameManagerService) DeleteInstance(ctx context.Context, user *models.User, instanceID, confirmName string) (response ServiceResponse) {
	response = ServiceResponse{}
	// Check if the user has permission to delete the instance
	instance, err := s.instanceRepo.GetByID(ctx, instanceID)
	if err != nil {
		response.AddFlashMessage(*flash.NewError("Instance not found"))
		response.Error = fmt.Errorf("finding instance: %w", err)
		return response
	}

	if user.ID != instance.UserID {
		response.AddFlashMessage(*flash.NewError("You do not have permission to delete this instance"))
		response.Error = errors.New("you do not have permission to delete this instance")
		return response
	}

	// If the name does not match the confirmation, return an error
	if confirmName != instance.Name {
		response.AddFlashMessage(*flash.NewError("Instance name does not match confirmation"))
		response.Error = errors.New("instance name does not match confirmation")
		return response
	}

	// Check if the user is currently using this instance
	if user.CurrentInstanceID == instance.ID {
		user.CurrentInstanceID = ""
		err := s.userService.UpdateUser(ctx, user)
		if err != nil {
			response.AddFlashMessage(*flash.NewError("Error updating user"))
			response.Error = fmt.Errorf("updating user: %w", err)
			return response
		}
	}

	// Start transaction
	tx, err := s.transactor.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		tx.Rollback()
		response.AddFlashMessage(*flash.NewError("Error starting transaction"))
		response.Error = fmt.Errorf("starting transaction: %w", err)
		return response
	}

	err = s.instanceRepo.Delete(ctx, tx, instanceID)
	if err != nil {
		tx.Rollback()
		response.AddFlashMessage(*flash.NewError("Error deleting instance"))
		response.Error = fmt.Errorf("deleting instance: %w", err)
		return response
	}

	err = s.instanceSettingsRepo.Delete(ctx, tx, instanceID)
	if err != nil {
		tx.Rollback()
		response.AddFlashMessage(*flash.NewError("Error deleting settings"))
		response.Error = fmt.Errorf("deleting settings: %w", err)
		return response
	}

	err = s.teamService.DeleteByInstanceID(ctx, tx, instanceID)
	if err != nil {
		tx.Rollback()
		response.AddFlashMessage(*flash.NewError("Error deleting teams"))
		response.Error = fmt.Errorf("deleting teams: %w", err)
		return response
	}

	err = tx.Commit()
	if err != nil {
		tx.Rollback()
		response.AddFlashMessage(*flash.NewError("Error committing transaction"))
		response.Error = fmt.Errorf("committing transaction: %w", err)
		return response
	}

	response.AddFlashMessage(*flash.NewSuccess("Instance deleted!"))
	return response
}

func (s *GameManagerService) SaveLocation(ctx context.Context, location *models.Location, lat, lng, name string) error {
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

func (s *GameManagerService) CreateLocation(
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
	instanceIDs, err := s.GetInstanceIDsForUser(ctx, user.ID)
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

func (s *GameManagerService) isMarkerShared(ctx context.Context, markerID, instanceID string) (bool, error) {
	shared, err := s.markerRepo.IsShared(ctx, markerID)
	if err != nil {
		return false, fmt.Errorf("checking if marker is shared: %w", err)
	}
	return shared, nil
}

func (s *GameManagerService) ValidateLocationMarker(user *models.User, id string) bool {
	for _, loc := range user.CurrentInstance.Locations {
		if loc.MarkerID == id {
			return true
		}
	}
	return false
}

func (s *GameManagerService) ValidateLocationID(user *models.User, id string) bool {
	for _, loc := range user.CurrentInstance.Locations {
		if loc.ID == id {
			return true
		}
	}
	return false
}

func (s *GameManagerService) GetQRCodePathAndContent(action, id, name, extension string) (string, string) {
	content := os.Getenv("SITE_URL")
	path := "assets/codes/"
	name = strings.Trim(name, " ")
	re := regexp.MustCompile(`[^\d\p{Latin} -]`)
	name = re.ReplaceAllString(name, "")
	if action == "in" {
		content = content + "/s/" + id
		path = path + extension + "/" + id + " " + name + "." + extension
	} else {
		content = content + "/o/" + id
		path = path + extension + "/" + id + " " + name + " Check Out." + extension
	}
	return path, content
}

// UpdateSettings parses the form values and updates the instance settings
func (s *GameManagerService) UpdateSettings(ctx context.Context, settings *models.InstanceSettings, form url.Values) error {
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

// StartGame starts the game immediately
func (s *GameManagerService) StartGame(ctx context.Context, user *models.User) (response ServiceResponse) {
	return s.SetStartTime(ctx, user, time.Now())
}

// StopGame stops the game immediately
func (s *GameManagerService) StopGame(ctx context.Context, user *models.User) (response ServiceResponse) {
	return s.SetEndTime(ctx, user, time.Now())
}

// SetStartTime sets the game start time to the given time
func (s *GameManagerService) SetStartTime(ctx context.Context, user *models.User, time time.Time) (response ServiceResponse) {
	response = ServiceResponse{}

	// Check if the game is already active
	if user.CurrentInstance.GetStatus() == models.Active {
		response.AddFlashMessage(*flash.NewInfo("Game is already active"))
		response.Error = errors.New("game is already active")
		return response
	}

	// Update the start time
	user.CurrentInstance.StartTime = bun.NullTime{Time: time}
	if !user.CurrentInstance.EndTime.After(time) {
		user.CurrentInstance.EndTime = bun.NullTime{}
	}

	if err := s.instanceRepo.Update(ctx, &user.CurrentInstance); err != nil {
		response.AddFlashMessage(*flash.NewError("Error updating instance"))
		response.Error = fmt.Errorf("updating instance with new time: %w", err)
		return response
	}

	msg := flash.NewSuccess("Game scheduled to start at " + time.Format("2006-01-02 15:04:05"))
	response.AddFlashMessage(*msg)
	return response
}

// SetEndTime sets the game end time to the given time
func (s *GameManagerService) SetEndTime(ctx context.Context, user *models.User, time time.Time) (response ServiceResponse) {
	response = ServiceResponse{}

	// Check if the game is already closed
	if user.CurrentInstance.GetStatus() == models.Closed {
		response.AddFlashMessage(*flash.NewError("Game is already over"))
		response.Error = errors.New("game is already closed")
		return response
	}

	// Make sure the end time is after the start time
	if !user.CurrentInstance.StartTime.IsZero() && time.Before(user.CurrentInstance.StartTime.Time) {
		response.AddFlashMessage(*flash.NewError("End time must be after start time"))
		response.Error = errors.New("end time must be after start time")
		return response
	}

	// Update the end time
	user.CurrentInstance.EndTime = bun.NullTime{Time: time}
	if err := s.instanceRepo.Update(ctx, &user.CurrentInstance); err != nil {
		response.AddFlashMessage(*flash.NewError("Error updating instance"))
		response.Error = fmt.Errorf("updating instance with new time: %w", err)
		return response
	}

	msg := flash.NewSuccess("Game scheduled to end at " + time.Format("2006-01-02 15:04:05"))
	response.AddFlashMessage(*msg)
	return response
}

// ScheduleGame schedules the game to start and/or end at a specific time
// Expects a form with set_start, utc_start_date, utc_start_time, set_end, utc_end_date, and utc_end_time
func (s *GameManagerService) ScheduleGame(ctx context.Context, user *models.User, start time.Time, end time.Time) (response ServiceResponse) {
	response = ServiceResponse{}

	instance := user.CurrentInstance

	// Check if the end time is before the start time
	if !instance.EndTime.IsZero() && instance.EndTime.Time.Before(instance.StartTime.Time) {
		response.AddFlashMessage(*flash.NewError("End time must be after start time"))
		response.Error = errors.New("end time must be after start time")
		return response
	}

	instance.StartTime = bun.NullTime{Time: start}
	instance.EndTime = bun.NullTime{Time: end}

	// Save instance
	if err := s.instanceRepo.Update(ctx, &instance); err != nil {
		response.AddFlashMessage(*flash.NewError("Error saving instance"))
		response.Error = fmt.Errorf("saving instance: %w", err)
		return response
	}

	user.CurrentInstance = instance
	return response
}

// DismissQuickstart marks the user as having dismissed the quickstart
func (s *GameManagerService) DismissQuickstart(ctx context.Context, instanceID string) error {
	return s.instanceRepo.DismissQuickstart(ctx, instanceID)
}

func (s *GameManagerService) GetInstanceIDsForUser(ctx context.Context, userID string) ([]string, error) {
	instances, err := s.instanceRepo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("finding instances for user: %w", err)
	}

	ids := make([]string, len(instances))
	for i, instance := range instances {
		ids[i] = instance.ID
	}
	return ids, nil
}
