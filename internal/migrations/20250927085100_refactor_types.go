package migrations

import (
	"time"

	"github.com/uptrace/bun"
)

type m20250927085100_RouteStrategy int
type m20250927085100_NavigationDisplayMode int

type InstanceSettings struct {
	bun.BaseModel `bun:"table:instance_settings"`

	CreatedAt time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`

	InstanceID            string                                `bun:"instance_id,pk,type:varchar(36)"`
	RouteStrategy         m20250927085100_RouteStrategy         `bun:"navigation_mode,type:int"`
	NavigationDisplayMode m20250927085100_NavigationDisplayMode `bun:"navigation_method,type:int"`
	MaxNextLocations      int                                   `bun:"max_next_locations,type:int,default:3"`
	MustCheckOut          bool                                  `bun:"must_check_out,type:bool"`
	ShowTeamCount         bool                                  `bun:"show_team_count,type:bool"`
	EnablePoints          bool                                  `bun:"enable_points,type:bool"`
	EnableBonusPoints     bool                                  `bun:"enable_bonus_points,type:bool"`
	ShowLeaderboard       bool                                  `bun:"show_leaderboard,type:bool"`
}
