package main

import (
	"context"
	"crypto/ed25519"
	"crypto/hpke"
	"crypto/rand"
	"errors"
	"flag"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"connectrpc.com/connect"

	"github.com/Khaymah/dhow/internal/runnersig"
	runnerv1 "github.com/Khaymah/dhow/packages/proto/gen/go/dhow/runner/v1"
	"github.com/Khaymah/dhow/packages/proto/gen/go/dhow/runner/v1/runnerv1connect"
)

const (
	usage       = "usage: runner register|authenticate --url URL --origin ORIGIN --dir DIR [--token-file FILE] [--runner-id ID]\n"
	seedFile    = "signing.seed"
	accessFile  = "access.token"
	runnerFile  = "runner.id"
	callTimeout = 10 * time.Second
)

type protocol struct{ next http.RoundTripper }

func (p protocol) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Dhow-Protocol", "1")
	r.Header.Set("Dhow-Runner-Version", "smoke")
	return p.next.RoundTrip(r)
}

type options struct {
	url, origin, dir, tokenFile string
}

func fail(msg string) int {
	_, _ = os.Stderr.WriteString(msg + "\n")
	return 1
}

func rpcFailure(err error) int {
	var ce *connect.Error
	if errors.As(err, &ce) {
		return fail(ce.Message())
	}
	return fail(err.Error())
}

func readToken(path string) (string, error) {
	var raw []byte
	var err error
	if path == "-" {
		raw, err = io.ReadAll(io.LimitReader(os.Stdin, 4096))
	} else {
		raw, err = os.ReadFile(path) //nolint:gosec // The smoke test passes a file it wrote itself.
	}
	return strings.TrimSpace(string(raw)), err
}

func register(ctx context.Context, o options, hc *http.Client) int {
	token, err := readToken(o.tokenFile)
	if err != nil {
		return fail("read token: " + err.Error())
	}
	seed := make([]byte, ed25519.SeedSize)
	if _, err := rand.Read(seed); err != nil {
		return fail(err.Error())
	}
	key := ed25519.NewKeyFromSeed(seed)
	enc, err := hpke.MLKEM768X25519().GenerateKey()
	if err != nil {
		return fail(err.Error())
	}
	pub, _ := key.Public().(ed25519.PublicKey)
	ek := enc.PublicKey().Bytes()
	client := runnerv1connect.NewRegistrationServiceClient(hc, o.url+"/runner-api")
	res, err := client.Register(ctx, connect.NewRequest(&runnerv1.RegisterRequest{
		Token: token, Name: "smoke-runner", Os: "linux", Arch: "amd64", Version: "smoke",
		IsolationLevels: []runnerv1.Isolation{runnerv1.Isolation_ISOLATION_CONTAINER}, Capacity: 1,
		SigningPublicKey: pub, EncryptionPublicKey: ek, Proof: ed25519.Sign(key, runnersig.RegisterMessage(o.origin, pub, ek)),
	}))
	if err != nil {
		return rpcFailure(err)
	}
	if err := os.WriteFile(filepath.Join(o.dir, seedFile), seed, 0o600); err != nil {
		return fail(err.Error())
	}
	if err := os.WriteFile(filepath.Join(o.dir, runnerFile), []byte(res.Msg.GetRunnerId()), 0o600); err != nil {
		return fail(err.Error())
	}
	_, _ = os.Stdout.WriteString(res.Msg.GetRunnerId() + " " + strings.Join(res.Msg.GetLabels(), ",") + "\n")
	return 0
}

func authenticate(ctx context.Context, o options, hc *http.Client) int {
	seed, err := os.ReadFile(filepath.Join(o.dir, seedFile))
	if err != nil {
		return fail(err.Error())
	}
	id, err := os.ReadFile(filepath.Join(o.dir, runnerFile))
	if err != nil {
		return fail(err.Error())
	}
	key := ed25519.NewKeyFromSeed(seed)
	ref := &runnerv1.RunnerRef{RunnerId: string(id)}
	client := runnerv1connect.NewAuthServiceClient(hc, o.url+"/runner-api")
	ch, err := client.Challenge(ctx, connect.NewRequest(&runnerv1.ChallengeRequest{Runner: ref}))
	if err != nil {
		return rpcFailure(err)
	}
	c := ch.Msg.GetChallenge()
	res, err := client.Authenticate(ctx, connect.NewRequest(&runnerv1.AuthenticateRequest{
		Runner: ref, Challenge: c, Signature: ed25519.Sign(key, runnersig.AuthMessage(o.origin, 1, c.GetChallengeBytes())),
	}))
	if err != nil {
		return rpcFailure(err)
	}
	if err := os.WriteFile(filepath.Join(o.dir, accessFile), []byte(res.Msg.GetAccessToken()), 0o600); err != nil {
		return fail(err.Error())
	}
	_, _ = os.Stdout.WriteString("authenticated\n")
	return 0
}

func run(args []string) int {
	if len(args) < 1 {
		_, _ = os.Stderr.WriteString(usage)
		return 2
	}
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	var o options
	fs.StringVar(&o.url, "url", "", "server URL")
	fs.StringVar(&o.origin, "origin", "", "the server's DHOW_PUBLIC_URL origin")
	fs.StringVar(&o.dir, "dir", "", "directory for the identity")
	fs.StringVar(&o.tokenFile, "token-file", "-", "registration token file, - for standard input")
	if err := fs.Parse(args[1:]); err != nil || o.url == "" || o.origin == "" || o.dir == "" {
		_, _ = os.Stderr.WriteString(usage)
		return 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
	defer cancel()
	hc := &http.Client{Transport: protocol{next: http.DefaultTransport}}
	switch args[0] {
	case "register":
		return register(ctx, o, hc)
	case "authenticate":
		return authenticate(ctx, o, hc)
	default:
		_, _ = os.Stderr.WriteString(usage)
		return 2
	}
}

func main() {
	os.Exit(run(os.Args[1:]))
}
