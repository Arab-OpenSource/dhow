package models

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const HashSize = sha256.Size

type Event struct {
	ID             uuid.UUID
	CreatedAt      time.Time
	OrganizationID *uuid.UUID
	ActorUserID    *uuid.UUID
	Action         string
	TargetType     string
	TargetID       string
	IP             string
	Data           map[string]string
	PrevHash       []byte
	Hash           []byte
}

type VerifyResult struct {
	Valid         bool
	Checked       int
	FirstBrokenID *uuid.UUID
}

type canonical struct {
	ID             string            `json:"Id"`
	CreatedAt      string            `json:"CreatedAt"`
	OrganizationID string            `json:"OrganizationId"`
	ActorUserID    string            `json:"ActorUserId"`
	Action         string            `json:"Action"`
	TargetType     string            `json:"TargetType"`
	TargetID       string            `json:"TargetId"`
	IP             string            `json:"Ip"`
	Data           map[string]string `json:"Data"`
}

func GenesisHash() []byte { return make([]byte, HashSize) }

func Canonical(e Event) ([]byte, error) {
	c := canonical{
		ID:         e.ID.String(),
		CreatedAt:  e.CreatedAt.UTC().Format(time.RFC3339Nano),
		Action:     e.Action,
		TargetType: e.TargetType,
		TargetID:   e.TargetID,
		IP:         e.IP,
		Data:       e.Data,
	}
	if e.OrganizationID != nil {
		c.OrganizationID = e.OrganizationID.String()
	}
	if e.ActorUserID != nil {
		c.ActorUserID = e.ActorUserID.String()
	}
	if c.Data == nil {
		c.Data = map[string]string{}
	}
	raw, err := json.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("encode audit event: %w", err)
	}
	return raw, nil
}

func ComputeHash(prev []byte, e Event) ([]byte, error) {
	raw, err := Canonical(e)
	if err != nil {
		return nil, err
	}
	h := sha256.New()
	h.Write(prev)
	h.Write(raw)
	return h.Sum(nil), nil
}
