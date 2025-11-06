package models

type Team struct {
	baseModel

	ID              string   `bun:"id,pk"`
	Code            string   `bun:"code,unique"`
	Name            string   `bun:"name,"`
	InstanceID      string   `bun:"instance_id,notnull"`
	HasStarted      bool     `bun:"has_started,default:false"`
	MustCheckOut    string   `bun:"must_scan_out"`
	Points          int      `bun:"points,"`
	SkippedGroupIDs []string `bun:"skipped_group_ids,type:text[],array"`

	Instance         Instance         `bun:"rel:has-one,join:instance_id=id"`
	CheckIns         []CheckIn        `bun:"rel:has-many,join:code=team_code"`
	BlockingLocation Location         `bun:"rel:has-one,join:must_scan_out=marker_id,join:instance_id=instance_id"`
	Messages         []Notification   `bun:"rel:has-many,join:code=team_code"`
	Blocks           []TeamBlockState `bun:"rel:has-many,join:code=team_code"`
}
