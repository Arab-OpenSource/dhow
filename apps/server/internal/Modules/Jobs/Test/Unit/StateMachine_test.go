package unit_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	models "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Jobs/Models"
)

type move struct{ from, to string }

func expectJob() map[move]bool {
	return map[move]bool{
		{"queued", "leased"}: true, {"queued", "canceled"}: true, {"queued", "failed"}: true,
		{"leased", "running"}: true, {"leased", "queued"}: true, {"leased", "canceled"}: true,
		{"leased", "failed"}: true, {"leased", "lost"}: true,
		{"running", "queued"}: true, {"running", "succeeded"}: true, {"running", "failed"}: true,
		{"running", "canceled"}: true, {"running", "timed_out"}: true, {"running", "lost"}: true,
	}
}

func expectAttempt() map[move]bool {
	return map[move]bool{
		{"leased", "running"}: true, {"leased", "failed"}: true, {"leased", "canceled"}: true, {"leased", "lost"}: true,
		{"running", "succeeded"}: true, {"running", "failed"}: true, {"running", "canceled"}: true,
		{"running", "timed_out"}: true, {"running", "lost"}: true,
	}
}

func expectLease() map[move]bool {
	return map[move]bool{{"active", "released"}: true, {"active", "expired"}: true, {"active", "revoked"}: true}
}

func expectStep() map[move]bool {
	return map[move]bool{{"running", "succeeded"}: true, {"running", "failed"}: true, {"running", "interrupted"}: true}
}

func TestEveryTransitionAllowedOrRefused(t *testing.T) {
	machines := []struct {
		name   string
		states []string
		can    func(string, string) bool
		want   map[move]bool
	}{
		{"job", models.JobStates(), models.JobCanMove, expectJob()},
		{"attempt", models.AttemptStates(), models.AttemptCanMove, expectAttempt()},
		{"lease", models.LeaseStates(), models.LeaseCanMove, expectLease()},
		{"step", models.StepStates(), models.StepCanMove, expectStep()},
	}
	for _, m := range machines {
		for _, from := range m.states {
			for _, to := range m.states {
				want := from == to || m.want[move{from, to}]
				t.Run(m.name+"/"+from+"->"+to, func(t *testing.T) {
					assert.Equal(t, want, m.can(from, to))
				})
			}
		}
	}
}

func TestTerminalStatesNeverMove(t *testing.T) {
	for _, from := range models.JobStates() {
		if !models.JobFinished(from) {
			continue
		}
		for _, to := range models.JobStates() {
			if to != from {
				assert.False(t, models.JobCanMove(from, to), "job %s -> %s", from, to)
			}
		}
	}
	for _, from := range models.AttemptStates() {
		if !models.AttemptFinished(from) {
			continue
		}
		for _, to := range models.AttemptStates() {
			if to != from {
				assert.False(t, models.AttemptCanMove(from, to), "attempt %s -> %s", from, to)
			}
		}
	}
}

func TestFinishedStates(t *testing.T) {
	assert.Equal(t, []string{"succeeded", "failed", "canceled", "timed_out", "lost"}, filter(models.JobStates(), models.JobFinished))
	assert.Equal(t, []string{"succeeded", "failed", "canceled", "timed_out", "lost"}, filter(models.AttemptStates(), models.AttemptFinished))
}

func filter(in []string, keep func(string) bool) []string {
	var out []string
	for _, s := range in {
		if keep(s) {
			out = append(out, s)
		}
	}
	return out
}
