package unit_test

import (
	"crypto/sha256"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	models "github.com/Khaymah/dhow/apps/server/internal/Modules/Audit/Models"
)

func sample() models.Event {
	org := uuid.MustParse("0198a000-0000-7000-8000-000000000001")
	actor := uuid.MustParse("0198a000-0000-7000-8000-000000000002")
	return models.Event{
		ID:             uuid.MustParse("0198a000-0000-7000-8000-000000000003"),
		CreatedAt:      time.Date(2026, 10, 27, 12, 30, 45, 123456000, time.UTC),
		OrganizationID: &org,
		ActorUserID:    &actor,
		Action:         "auth.login",
		TargetType:     "session",
		TargetID:       "0198a000-0000-7000-8000-000000000004",
		IP:             "203.0.113.7",
		Data:           map[string]string{"b": "2", "a": "1"},
	}
}

func TestCanonicalIsStable(t *testing.T) {
	raw, err := models.Canonical(sample())
	require.NoError(t, err)
	assert.JSONEq(t, `{"Id":"0198a000-0000-7000-8000-000000000003","CreatedAt":"2026-10-27T12:30:45.123456Z",`+
		`"OrganizationId":"0198a000-0000-7000-8000-000000000001","ActorUserId":"0198a000-0000-7000-8000-000000000002",`+
		`"Action":"auth.login","TargetType":"session","TargetId":"0198a000-0000-7000-8000-000000000004","Ip":"203.0.113.7",`+
		`"Data":{"a":"1","b":"2"}}`, string(raw))
	assert.Equal(t, `{"Id":"0198a000-0000-7000-8000-000000000003","CreatedAt":"2026-10-27T12:30:45.123456Z",`+
		`"OrganizationId":"0198a000-0000-7000-8000-000000000001","ActorUserId":"0198a000-0000-7000-8000-000000000002",`+
		`"Action":"auth.login","TargetType":"session","TargetId":"0198a000-0000-7000-8000-000000000004","Ip":"203.0.113.7",`+
		`"Data":{"a":"1","b":"2"}}`, string(raw), "field order and key order are part of the format")
}

func TestHashIsSHA256OfPrevAndCanonical(t *testing.T) {
	e := sample()
	prev := models.GenesisHash()
	got, err := models.ComputeHash(prev, e)
	require.NoError(t, err)
	raw, err := models.Canonical(e)
	require.NoError(t, err)
	want := sha256.Sum256(append(append([]byte{}, prev...), raw...))
	assert.Equal(t, want[:], got)
	assert.Len(t, got, models.HashSize)
	assert.Equal(t, make([]byte, 32), prev)
}

func TestHashChangesWithEveryField(t *testing.T) {
	base, err := models.ComputeHash(models.GenesisHash(), sample())
	require.NoError(t, err)
	other := uuid.MustParse("0198a000-0000-7000-8000-0000000000ff")
	tests := []struct {
		name string
		edit func(*models.Event)
	}{
		{"id", func(e *models.Event) { e.ID = other }},
		{"time", func(e *models.Event) { e.CreatedAt = e.CreatedAt.Add(time.Microsecond) }},
		{"organization", func(e *models.Event) { e.OrganizationID = &other }},
		{"no organization", func(e *models.Event) { e.OrganizationID = nil }},
		{"actor", func(e *models.Event) { e.ActorUserID = &other }},
		{"no actor", func(e *models.Event) { e.ActorUserID = nil }},
		{"action", func(e *models.Event) { e.Action = "auth.logout" }},
		{"target type", func(e *models.Event) { e.TargetType = "user" }},
		{"target id", func(e *models.Event) { e.TargetID = "x" }},
		{"ip", func(e *models.Event) { e.IP = "203.0.113.8" }},
		{"data value", func(e *models.Event) { e.Data = map[string]string{"a": "1", "b": "3"} }},
		{"data key", func(e *models.Event) { e.Data = map[string]string{"a": "1", "c": "2"} }},
		{"extra data", func(e *models.Event) { e.Data = map[string]string{"a": "1", "b": "2", "c": "3"} }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := sample()
			tt.edit(&e)
			got, err := models.ComputeHash(models.GenesisHash(), e)
			require.NoError(t, err)
			assert.NotEqual(t, base, got)
		})
	}
	t.Run("previous hash", func(t *testing.T) {
		prev := models.GenesisHash()
		prev[0] = 1
		got, err := models.ComputeHash(prev, sample())
		require.NoError(t, err)
		assert.NotEqual(t, base, got)
	})
}

func TestHashIgnoresWhatIsNotPartOfTheEvent(t *testing.T) {
	base, err := models.ComputeHash(models.GenesisHash(), sample())
	require.NoError(t, err)
	e := sample()
	e.PrevHash, e.Hash = []byte("whatever"), []byte("whatever")
	e.CreatedAt = e.CreatedAt.In(time.FixedZone("EET", 2*3600))
	got, err := models.ComputeHash(models.GenesisHash(), e)
	require.NoError(t, err)
	assert.Equal(t, base, got, "the stored hashes and the time zone do not change the digest")

	e = sample()
	e.Data = nil
	nilData, err := models.ComputeHash(models.GenesisHash(), e)
	require.NoError(t, err)
	e.Data = map[string]string{}
	emptyData, err := models.ComputeHash(models.GenesisHash(), e)
	require.NoError(t, err)
	assert.Equal(t, nilData, emptyData, "nil and empty data are the same event")
}

func TestChainLinks(t *testing.T) {
	prev := models.GenesisHash()
	seen := map[string]bool{}
	for i := range 20 {
		e := sample()
		e.TargetID = string(rune('a' + i))
		h, err := models.ComputeHash(prev, e)
		require.NoError(t, err)
		require.False(t, seen[string(h)], "hash %d repeats", i)
		seen[string(h)] = true
		prev = h
	}
}
