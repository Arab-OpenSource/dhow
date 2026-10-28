package client

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"

	config "github.com/Khaymah/dhow/apps/runner/internal/Config"
	runnerv1 "github.com/Khaymah/dhow/packages/proto/gen/go/dhow/runner/v1"
)

var ErrNoIdentity = errors.New("this client has no runner identity")

type unary[Req, Res any] func(context.Context, *connect.Request[Req]) (*connect.Response[Res], error)

func call[Req, Res any](ctx context.Context, c *Client, timeout time.Duration, fn unary[Req, Res], msg *Req) (*Res, error) {
	if c.tokens == nil {
		return nil, ErrNoIdentity
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	for attempt := 0; ; attempt++ {
		token, err := c.tokens.get(ctx)
		if err != nil {
			return nil, err
		}
		req := connect.NewRequest(msg)
		req.Header().Set("Authorization", "Bearer "+token)
		res, err := fn(ctx, req)
		if err == nil {
			return res.Msg, nil
		}
		err = mapAuthError(err)
		if Code(err) == connect.CodeUnauthenticated && attempt == 0 {
			c.tokens.invalidate(token)
			continue
		}
		return nil, err
	}
}

func (c *Client) Register(ctx context.Context, req *runnerv1.RegisterRequest) (*runnerv1.RegisterResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, config.RPCTimeout)
	defer cancel()
	res, err := c.registration.Register(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, mapAuthError(err)
	}
	return res.Msg, nil
}

func (c *Client) Authenticate(ctx context.Context) error {
	if c.tokens == nil {
		return ErrNoIdentity
	}
	_, err := c.tokens.get(ctx)
	return err
}

func (c *Client) Lease(ctx context.Context, freeSlots int32, wait time.Duration) (*runnerv1.SignedLeaseGrant, error) {
	res, err := call(ctx, c, wait+config.LeasePollSlack, c.lease.Lease, &runnerv1.LeaseRequest{
		FreeSlots: freeSlots, WaitSeconds: int32(wait / time.Second), //nolint:gosec // The wait is at most 25 seconds.
	})
	if err != nil {
		return nil, err
	}
	return res.GetGrant(), nil
}

func (c *Client) Start(ctx context.Context, ref *runnerv1.LeaseRef, proof []byte) (*runnerv1.StartResponse, error) {
	return call(ctx, c, config.RPCTimeout, c.lease.Start, &runnerv1.StartRequest{Lease: ref, PopSignature: proof})
}

func (c *Client) Heartbeat(ctx context.Context, ref *runnerv1.LeaseRef) (*runnerv1.LeaseControl, error) {
	res, err := call(ctx, c, config.RPCTimeout, c.lease.Heartbeat, &runnerv1.HeartbeatRequest{Lease: ref})
	if err != nil {
		return nil, err
	}
	return res.GetControl(), nil
}

func (c *Client) BeginStep(ctx context.Context, ref *runnerv1.LeaseRef, index int32, mutating bool) (*runnerv1.LeaseControl, error) {
	res, err := call(ctx, c, config.RPCTimeout, c.lease.BeginStep, &runnerv1.BeginStepRequest{Lease: ref, Index: index, Mutating: mutating})
	if err != nil {
		return nil, err
	}
	return res.GetControl(), nil
}

func (c *Client) EndStep(ctx context.Context, ref *runnerv1.LeaseRef, index, exitCode int32, reason runnerv1.FailureReason) (*runnerv1.LeaseControl, error) {
	res, err := call(ctx, c, config.RPCTimeout, c.lease.EndStep, &runnerv1.EndStepRequest{
		Lease: ref, Index: index, ExitCode: exitCode, FailureReason: reason,
	})
	if err != nil {
		return nil, err
	}
	return res.GetControl(), nil
}

func (c *Client) Complete(ctx context.Context, req *runnerv1.CompleteRequest) error {
	_, err := call(ctx, c, config.RPCTimeout, c.lease.Complete, req)
	return err
}

func (c *Client) Append(ctx context.Context, ref *runnerv1.LeaseRef, chunks []*runnerv1.LogChunk) (*runnerv1.AppendResponse, error) {
	return call(ctx, c, config.AppendTimeout, c.logs.Append, &runnerv1.AppendRequest{Lease: ref, Chunks: chunks})
}
