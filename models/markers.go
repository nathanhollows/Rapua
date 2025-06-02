package models

type Marker struct {
	baseModel

	Code         string  `bun:"code,unique,pk"`
	Lat          float64 `bun:"lat,type:float"`
	Lng          float64 `bun:"lng,type:float"`
	Name         string  `bun:"name,type:varchar(255)"`
	TotalVisits  int     `bun:"total_visits,type:int"`
	CurrentCount int     `bun:"current_count,type:int"`
	AvgDuration  float64 `bun:"avg_duration,type:float"`

	Locations []Location `bun:"rel:has-many,join:code=marker_id"`
}

func (m Marker) IsMapped() bool {
	return m.Lat != 0 && m.Lng != 0
}
