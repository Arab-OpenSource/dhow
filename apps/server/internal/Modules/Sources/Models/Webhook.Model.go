package models

import "github.com/google/uuid"

const (
	OutcomeTriggered = "triggered"
	OutcomeIgnored   = "ignored"

	ReasonEventNotSupported = "event_not_supported"
	ReasonRefDeleted        = "ref_deleted"
	ReasonProjectArchived   = "project_archived"
	ReasonHostKeyPending    = "host_key_unconfirmed"
)

type Event struct {
	Name   string
	Ref    string
	Sha    string
	Actor  string
	Ignore string
}

type Received struct {
	Duplicate  bool
	DeliveryID uuid.UUID
	Outcome    string
	Reason     string
}
