package models

type QuestSettings struct {
	baseModel

	QuestID         string `bun:"quest_id,pk,type:varchar(36)"`
	EnablePoints    bool   `bun:"enable_points,type:bool"`
	ShowLeaderboard bool   `bun:"show_leaderboard,type:bool"`
}
