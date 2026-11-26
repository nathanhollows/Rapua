package models

type Notification struct {
	baseModel

	ID        string `bun:"id,pk,notnull"`
	Content   string `bun:"content,type:varchar(255)"`
	Type      string `bun:"type,type:varchar(255)"`
	RunCode   string `bun:"run_code,type:varchar(36)"`
	Dismissed bool   `bun:"dismissed,type:bool"`
}
