package models

import "github.com/nathanhollows/Rapua/v8/game"

type QuestSettings struct {
	baseModel

	QuestID         string `bun:"quest_id,pk,type:varchar(36)"`
	EnablePoints    bool   `bun:"enable_points,type:bool"`
	ShowLeaderboard bool   `bun:"show_leaderboard,type:bool"`
	// Read through Normalised: a row from before the column carries nothing.
	ProgressFeedback game.ProgressFeedback `bun:"progress_feedback,type:varchar(16)"`
}
