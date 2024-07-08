package models

type InstanceLocation struct {
	baseModel

	ID         string             `bun:",pk" json:"id"`
	InstanceID string             `bun:",notnull" json:"instance_id"`
	LocationID string             `bun:",notnull" json:"location_id"`
	CriteriaID string             `bun:",notnull" json:"criteria_id"`
	Instance   Instance           `bun:"rel:has-one,join:instance_id=id" json:"instance"`
	Location   Location           `bun:"rel:has-one,join:location_id=code" json:"location"`
	Criteria   CompletionCriteria `bun:"rel:has-one,join:criteria_id=id" json:"criteria"`
}

type InstanceLocations []InstanceLocation