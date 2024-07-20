package services

import (
	"context"
	"fmt"
	"hash/fnv"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/nathanhollows/Rapua/internal/flash"
	"github.com/nathanhollows/Rapua/internal/models"
	"github.com/nathanhollows/Rapua/pkg/db"
	"github.com/uptrace/bun"
	"golang.org/x/exp/rand"
)

type GameplayService struct{}

func (s *GameplayService) GetTeamByCode(ctx context.Context, teamCode string) (*models.Team, error) {
	teamCode = strings.TrimSpace(strings.ToUpper(teamCode))
	team, err := models.FindTeamByCode(ctx, teamCode)
	if err != nil {
		return nil, fmt.Errorf("GetTeamStatus: %w", err)
	}
	return team, nil
}

func (s *GameplayService) GetLocationByCode(ctx context.Context, team *models.Team, locationCode string) (*models.Location, error) {
	locationCode = strings.TrimSpace(strings.ToUpper(locationCode))
	location, err := models.FindLocationByInstanceAndCode(ctx, team.InstanceID, locationCode)
	if err != nil {
		return nil, fmt.Errorf("GetLocationByCode: %w", err)
	}
	return location, nil
}

func (s *GameplayService) StartPlaying(ctx context.Context, teamCode, customTeamName string) (response *ServiceResponse) {
	response = &ServiceResponse{}

	team, err := models.FindTeamByCode(ctx, teamCode)
	if err != nil {
		response.Error = fmt.Errorf("StartPlaying find team: %w", err)
		response.AddFlashMessage(*flash.NewError("Team not found. Please double check the code and try again."))
		return response
	}

	// Update team with custom name if provided
	if !team.HasStarted || customTeamName != "" {
		team.Name = customTeamName
		team.HasStarted = true
		if err := team.Update(ctx); err != nil {
			response.Error = fmt.Errorf("StartPlaying update team: %w", err)
			response.AddFlashMessage(*flash.NewError("Something went wrong. Please try again."))
			return response
		}
	}

	response.AddFlashMessage(*flash.NewSuccess("You have started the game!"))
	return response
}

func (s *GameplayService) SuggestNextLocations(ctx context.Context, team *models.Team, limit int) ([]*models.Location, error) {
	var locations []*models.Location

	visited := make([]string, len(team.Scans))
	for i, s := range team.Scans {
		visited[i] = s.LocationID
	}

	var err error
	if len(visited) != 0 {
		err = db.DB.NewSelect().Model(&locations).
			Where("location.instance_id = ?", team.InstanceID).
			Where("location.code NOT IN (?)", bun.In(visited)).
			Relation("Marker").
			Scan(ctx)
	} else {
		err = db.DB.NewSelect().Model(&locations).
			Where("location.instance_id = ?", team.InstanceID).
			Relation("Marker").
			Scan(ctx)
	}
	if err != nil {
		log.Error(err)
		return nil, err
	}

	seed := team.Code + fmt.Sprintf("%s", visited)
	h := fnv.New64a()
	_, err = h.Write([]byte(seed))
	if err != nil {
		log.Error(err)
		return nil, err
	}

	rand.New(rand.NewSource(uint64(h.Sum64()))).Shuffle(len(locations), func(i, j int) {
		locations[i], locations[j] = locations[j], locations[i]
	})

	if len(locations) > limit {
		locations = locations[:limit]
	}

	for i := 0; i < len(locations); i++ {
		for j := i + 1; j < len(locations); j++ {
			if locations[i].CurrentCount > locations[j].CurrentCount {
				locations[i], locations[j] = locations[j], locations[i]
			}
		}
	}

	return locations, nil
}

func (s *GameplayService) CheckIn(ctx context.Context, team *models.Team, locationCode string) (response *ServiceResponse) {
	response = &ServiceResponse{}

	location, err := models.FindLocationByInstanceAndCode(ctx, team.InstanceID, locationCode)
	if err != nil {
		msg := flash.NewWarning("Please double check the code and try again.").SetTitle("Location code not found")
		response.AddFlashMessage(msg)
		response.Error = fmt.Errorf("finding location: %w", err)
		return response
	}

	if team.MustScanOut != "" {
		if locationCode != team.MustScanOut {
			msg := flash.NewWarning(fmt.Sprintf("You need to scan out at %s.", team.BlockingLocation.Name)).SetTitle("You are already scanned in.")
			response.AddFlashMessage(msg)
			response.Error = fmt.Errorf("player must scan out first")
			return response
		}
	}

	// Check if the team has already scanned in
	scanned := false
	for _, s := range team.Scans {
		if s.LocationID == location.Code {
			scanned = true
			break
		}
	}
	if scanned {
		msg := flash.NewWarning("If you want to revisit this site, please click \"See my scanned locations\" below").SetTitle("You have already visited here.")
		response.AddFlashMessage(msg)
		response.Error = fmt.Errorf("player has already scanned in")
		return response
	}

	// Check if the location is valid for the team to check in
	valid := s.CheckValidLocation(ctx, team, locationCode)
	if valid.Error != nil {
		response.Error = fmt.Errorf("CheckIn: %w", valid.Error)
		return response
	}

	// Log the CheckIn
	_, err = location.LogScan(ctx, team.Code)
	if err != nil {
		msg := flash.NewWarning("Please double check the code and try again.").SetTitle("Couldn't scan in.")
		response.AddFlashMessage(msg)
		err := fmt.Errorf("logging scan: %w", err)
		response.Error = fmt.Errorf("logging scan: %w", err)
		return response
	}

	response.Data = make(map[string]interface{})
	response.Data["locationID"] = location.Code

	msg := flash.NewSuccess("You have scanned in.")
	response.AddFlashMessage(*msg)
	return response
}

func (s *GameplayService) CheckOut(ctx context.Context, teamCode, locationCode string) error {
	team, err := models.FindTeamByCode(ctx, teamCode)
	if err != nil {
		return fmt.Errorf("LogScanOut: %v", err)
	}

	location, err := models.FindLocationByInstanceAndCode(ctx, team.InstanceID, locationCode)
	if err != nil {
		return fmt.Errorf("LogScanOut: %v", err)
	}

	// Check if the team must scan out
	if team.MustScanOut == "" {
		return fmt.Errorf("You don't need to scan out.")
	} else if team.MustScanOut != locationCode {
		return fmt.Errorf("You need to scan out at %s", team.BlockingLocation.Name)
	}

	// Log the scan out
	err = location.LogScanOut(ctx, teamCode)
	if err != nil {
		return fmt.Errorf("LogScanOut: %v", err)
	}

	// Clear the mustScanOut field
	team.MustScanOut = ""
	err = team.Update(ctx)
	if err != nil {
		return fmt.Errorf("LogScanOut: %v", err)
	}

	return nil
}

// CheckLocation checks if the location is valid for the team to check in
func (s *GameplayService) CheckValidLocation(ctx context.Context, team *models.Team, locationCode string) (response *ServiceResponse) {
	response = &ServiceResponse{}

	switch team.Instance.NavigationMode {
	// All locations are valid in FreeRoamShowAllNavigation mode
	case models.FreeRoamShowAllNavigation:
		return response
	default:
		response.Error = fmt.Errorf("CheckValidLocation: unknown navigation mode")
	}

	return response
}
