package sources

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"

	config "github.com/Khaymah/dhow/apps/server/internal/Config"
	apperrors "github.com/Khaymah/dhow/apps/server/internal/Core/Errors"
	ratelimit "github.com/Khaymah/dhow/apps/server/internal/Core/RateLimit"
	"github.com/Khaymah/dhow/internal/gitsource"
	"github.com/Khaymah/dhow/internal/netguard"
)

const (
	hostKeyUser   = "git"
	limitDialUser = "git-dial-user"
	limitDialOrg  = "git-dial-org"
)

var errHostKeyCaptured = errors.New("the host key was captured")

func hostKeyAlgorithms() []string {
	return []string{
		ssh.KeyAlgoED25519, ssh.KeyAlgoECDSA256, ssh.KeyAlgoECDSA384, ssh.KeyAlgoECDSA521, ssh.KeyAlgoRSASHA512, ssh.KeyAlgoRSASHA256,
	}
}

func (s *Service) chargeDial(ctx context.Context, org, user uuid.UUID) error {
	if s.limiter == nil {
		return nil
	}
	for _, c := range []struct {
		limit   ratelimit.Limit
		subject string
	}{
		{ratelimit.Limit{Name: limitDialUser, Max: config.GitDialPerUser, Window: config.GitDialWindow}, user.String()},
		{ratelimit.Limit{Name: limitDialOrg, Max: config.GitDialPerOrg, Window: config.GitDialWindow}, org.String()},
	} {
		d, err := s.limiter.Take(ctx, c.limit, c.subject)
		if err != nil {
			return err
		}
		if err := d.Err(); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) fetchHostKey(ctx context.Context, u gitsource.URL) (ssh.PublicKey, error) {
	ctx, cancel := context.WithTimeout(ctx, config.HostKeyFetchTimeout)
	defer cancel()
	conn, err := s.dialer.DialContext(ctx, "tcp", u.HostPort())
	if errors.Is(err, netguard.ErrBlocked) {
		return nil, apperrors.GitHostBlocked()
	}
	if err != nil {
		return nil, apperrors.GitHostUnreachable(err)
	}
	defer func() { _ = conn.Close() }()
	deadline, _ := ctx.Deadline()
	if err := conn.SetDeadline(deadline); err != nil {
		return nil, apperrors.GitHostUnreachable(err)
	}
	stop := context.AfterFunc(ctx, func() { _ = conn.SetDeadline(time.Unix(1, 0)) })
	defer stop()
	user := u.User
	if user == "" {
		user = hostKeyUser
	}
	var captured ssh.PublicKey
	cfg := &ssh.ClientConfig{
		User:              user,
		HostKeyAlgorithms: hostKeyAlgorithms(),
		Timeout:           config.HostKeyFetchTimeout,
		HostKeyCallback: func(_ string, _ net.Addr, key ssh.PublicKey) error {
			captured = key
			return errHostKeyCaptured
		},
	}
	_, _, _, err = ssh.NewClientConn(conn, u.HostPort(), cfg)
	if captured != nil {
		return captured, nil
	}
	return nil, apperrors.GitHostUnreachable(err)
}
