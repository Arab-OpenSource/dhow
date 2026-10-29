package installer

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDoctorWarnsWhenTheEnvironmentListStrandsLinkedAccounts(t *testing.T) {
	tests := []struct {
		name    string
		env     string
		db      fakeDB
		warn    bool
		wantIDs []string
	}{
		{"accounts on saved providers", "DHOW_OIDC_PROVIDERS= google , corp,\n", fakeDB{stranded: Stranded{Providers: 2, Identities: 5}}, true, []string{"google", "corp"}},
		{"saved providers without accounts", "DHOW_OIDC_PROVIDERS=google\n", fakeDB{stranded: Stranded{Providers: 0, Identities: 0}}, false, []string{"google"}},
		{"the database is unreachable", "DHOW_OIDC_PROVIDERS=google\n", fakeDB{err: errors.New("connection refused")}, false, []string{"google"}},
		{"the list is in the dashboard", "", fakeDB{stranded: Stranded{Providers: 2, Identities: 5}}, false, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, _ := installedHost(t)
			f.put(EnvFilePath, f.content(EnvFilePath)+tt.env, 0o600)
			var active []string
			tt.db.active = &active
			app, out := newApp(f, "")
			app.DB = tt.db
			err := app.Doctor(context.Background(), DoctorOptions{})
			const title = "5 accounts linked to 2 sign-in providers saved in the database cannot sign in with them, because DHOW_OIDC_PROVIDERS is set in env"
			if tt.warn {
				assert.Contains(t, out.String(), title)
				assert.Contains(t, out.String(), "Remove DHOW_OIDC_PROVIDERS from the env file")
				assert.ErrorIs(t, err, ErrUnhealthy)
			} else {
				assert.NotContains(t, out.String(), "cannot sign in with them")
			}
			assert.Equal(t, tt.wantIDs, active)
		})
	}
}

func TestDoctorCannotCountStrandedAccountsInCompose(t *testing.T) {
	d := Doctor{A: Answers{Mode: ModeCompose}, Settings: fakeDB{stranded: Stranded{Providers: 1, Identities: 1}}}
	assert.Empty(t, d.strandedIdentities(context.Background(), []string{"corp"}))
}

func TestTheStrandedWarningReadsInArabicToo(t *testing.T) {
	ar := NewMessages(LangAr).T(KeyCheckStranded, 5, 2)
	assert.Contains(t, ar, "عدد الحسابات: 5")
	assert.Contains(t, ar, "عدد المزوّدين: 2")
}
