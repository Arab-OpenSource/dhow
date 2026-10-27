package models

import "time"

type Instance struct {
	ID               int16
	CreatedAt        time.Time
	UpdatedAt        time.Time
	SetupCompletedAt *time.Time
}

func (i Instance) SetupRequired() bool { return i.SetupCompletedAt == nil }
