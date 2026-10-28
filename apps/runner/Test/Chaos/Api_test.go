//go:build chaos

package chaos_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	ownerEmail    = "owner@chaos.dhow.test"
	ownerPassword = "chaos-owner-password-7f3c9a"
	apiTimeout    = 15 * time.Second
)

type api struct {
	base   string
	cookie string
	org    string
}

type envelope struct {
	Success bool            `json:"Success"`
	Data    json.RawMessage `json:"Data"`
	Error   json.RawMessage `json:"Error"`
}

type jobStep struct {
	Name           string `json:"Name"`
	Run            string `json:"Run"`
	Mutating       bool   `json:"Mutating,omitempty"`
	TimeoutSeconds int    `json:"TimeoutSeconds,omitempty"`
}

type jobSpec struct {
	Image          string            `json:"Image"`
	Steps          []jobStep         `json:"Steps"`
	Env            map[string]string `json:"Env,omitempty"`
	TimeoutSeconds int               `json:"TimeoutSeconds,omitempty"`
	Network        *jobNetwork       `json:"Network,omitempty"`
}

type jobNetwork struct {
	Egress []string `json:"Egress"`
}

type jobAttempt struct {
	ID            string `json:"Id"`
	Number        int    `json:"Number"`
	State         string `json:"State"`
	FailureReason string `json:"FailureReason"`
}

type jobDetail struct {
	Job struct {
		ID            string `json:"Id"`
		State         string `json:"State"`
		FailureReason string `json:"FailureReason"`
		AttemptCount  int    `json:"AttemptCount"`
	} `json:"Job"`
	Attempts []jobAttempt `json:"Attempts"`
}

func (a *api) call(method, path string, body any, out any) (int, error) {
	var buf io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return 0, err
		}
		buf = bytes.NewReader(raw)
	}
	ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, a.base+"/api/v1"+path, buf)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", a.base)
	if a.cookie != "" {
		req.Header.Set("Cookie", a.cookie)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	for _, c := range resp.Cookies() {
		if strings.HasSuffix(c.Name, "dhow_session") && c.Value != "" {
			a.cookie = c.Name + "=" + c.Value
		}
	}
	var env envelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return resp.StatusCode, fmt.Errorf("decode %s %s: %w", method, path, err)
	}
	if !env.Success {
		return resp.StatusCode, fmt.Errorf("%s %s answered %d: %s", method, path, resp.StatusCode, env.Error)
	}
	if out != nil {
		return resp.StatusCode, json.Unmarshal(env.Data, out)
	}
	return resp.StatusCode, nil
}

func (a *api) setupE(setupToken string) error {
	var res struct {
		Organization struct {
			ID string `json:"Id"`
		} `json:"Organization"`
	}
	if _, err := a.call(http.MethodPost, "/setup", map[string]string{
		"SetupToken": setupToken, "OwnerName": "Chaos Owner", "Email": ownerEmail, "Password": ownerPassword,
		"OrganizationName": "Chaos", "OrganizationSlug": "chaos",
	}, &res); err != nil {
		return err
	}
	a.org = res.Organization.ID
	if a.cookie == "" {
		if _, err := a.call(http.MethodPost, "/sessions", map[string]string{"Email": ownerEmail, "Password": ownerPassword}, nil); err != nil {
			return err
		}
	}
	if a.cookie == "" {
		return fmt.Errorf("neither setup nor sign-in returned a session cookie")
	}
	return nil
}

func (a *api) registrationTokenE() (string, error) {
	var res struct {
		Token string `json:"Token"`
	}
	_, err := a.call(http.MethodPost, "/organizations/"+a.org+"/runner-registration-tokens",
		map[string]any{"Description": "chaos", "Labels": []string{"pool=chaos"}}, &res)
	return res.Token, err
}

func (a *api) createJob(t *testing.T, name string, spec jobSpec) string {
	t.Helper()
	var res jobDetail
	_, err := a.call(http.MethodPost, "/organizations/"+a.org+"/jobs",
		map[string]any{"Name": name, "Spec": spec, "MaxInfraRetries": 2}, &res)
	require.NoError(t, err)
	return res.Job.ID
}

func (a *api) job(id string) (jobDetail, error) {
	var res jobDetail
	_, err := a.call(http.MethodGet, "/organizations/"+a.org+"/jobs/"+id, nil, &res)
	return res, err
}

func (a *api) cancel(t *testing.T, id string) {
	t.Helper()
	_, err := a.call(http.MethodPost, "/organizations/"+a.org+"/jobs/"+id+"/cancel", nil, nil)
	require.NoError(t, err)
}

func (a *api) waitJob(t *testing.T, id string, within time.Duration, done func(jobDetail) bool) jobDetail {
	t.Helper()
	deadline := time.Now().Add(within)
	var last jobDetail
	for time.Now().Before(deadline) {
		d, err := a.job(id)
		if err == nil {
			last = d
			if done(d) {
				return d
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("job %s did not reach the expected state within %s, last state %q (%s), %d attempts",
		id, within, last.Job.State, last.Job.FailureReason, last.Job.AttemptCount)
	return last
}

func finished(d jobDetail) bool {
	switch d.Job.State {
	case "succeeded", "failed", "canceled", "lost", "timed_out":
		return true
	}
	return false
}
