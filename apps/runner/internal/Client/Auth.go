package client

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	config "github.com/Arab-OpenSource/dhow/apps/runner/internal/Config"
	identity "github.com/Arab-OpenSource/dhow/apps/runner/internal/Identity"
	"github.com/Arab-OpenSource/dhow/internal/runnersig"
	runnerv1 "github.com/Arab-OpenSource/dhow/packages/proto/gen/go/dhow/runner/v1"
	"github.com/Arab-OpenSource/dhow/packages/proto/gen/go/dhow/runner/v1/runnerv1connect"
)

const maxTokenLength = 128

type tokens struct {
	mu      sync.Mutex
	auth    runnerv1connect.AuthServiceClient
	id      *identity.Identity
	now     func() time.Time
	token   string
	renewAt time.Time
}

func (t *tokens) get(ctx context.Context) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.token != "" && t.now().Before(t.renewAt) {
		return t.token, nil
	}
	token, renewAt, err := t.authenticate(ctx)
	if err != nil {
		return "", err
	}
	t.token, t.renewAt = token, renewAt
	return token, nil
}

func (t *tokens) invalidate(token string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.token == token {
		t.token = ""
	}
}

func (t *tokens) authenticate(ctx context.Context) (string, time.Time, error) {
	ctx, cancel := context.WithTimeout(ctx, config.RPCTimeout)
	defer cancel()
	started := t.now()
	ref := &runnerv1.RunnerRef{RunnerId: t.id.RunnerID}
	ch, err := t.auth.Challenge(ctx, connect.NewRequest(&runnerv1.ChallengeRequest{Runner: ref}))
	if err != nil {
		return "", time.Time{}, mapAuthError(err)
	}
	issued := ch.Msg.GetChallenge()
	var parsed runnerv1.AuthChallenge
	if err := proto.Unmarshal(issued.GetChallengeBytes(), &parsed); err != nil || parsed.GetRunnerId() != t.id.RunnerID {
		return "", time.Time{}, ErrBadChallenge
	}
	signature := t.id.Sign(runnersig.AuthMessage(t.id.ServerOrigin, t.id.ServerKeyVersion, issued.GetChallengeBytes()))
	res, err := t.auth.Authenticate(ctx, connect.NewRequest(&runnerv1.AuthenticateRequest{
		Runner: ref, Challenge: issued, Signature: signature,
	}))
	if err != nil {
		err = mapAuthError(err)
		if Code(err) == connect.CodeUnauthenticated {
			err = errors.Join(ErrCredentials, err)
		}
		return "", time.Time{}, err
	}
	token := res.Msg.GetAccessToken()
	lifetime := time.Duration(res.Msg.GetExpiresInSeconds()) * time.Second
	if !strings.HasPrefix(token, config.AccessTokenPrefix) || len(token) > maxTokenLength || lifetime <= 0 {
		return "", time.Time{}, ErrBadToken
	}
	return token, started.Add(lifetime * config.TokenRenewNumerator / config.TokenRenewDivisor), nil
}
