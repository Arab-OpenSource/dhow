package models

import (
	"time"

	"github.com/google/uuid"
)

type Filter struct {
	Category    string
	Action      string
	ActorUserID *uuid.UUID
	From        *time.Time
	To          *time.Time
}
