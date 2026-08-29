package models

type QuestSettings struct {
	baseModel

	QuestID         string `bun:"quest_id,pk,type:varchar(36)"`
	ShowTeamCount   bool   `bun:"show_team_count,type:bool"`
	EnablePoints    bool   `bun:"enable_points,type:bool"`
	ShowLeaderboard bool   `bun:"show_leaderboard,type:bool"`
}
