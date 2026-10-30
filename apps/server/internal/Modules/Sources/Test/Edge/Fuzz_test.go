package edge_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	sources "github.com/Khaymah/dhow/apps/server/internal/Modules/Sources"
	"github.com/Khaymah/dhow/internal/gitref"
)

var fuzzNow = time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)

func macHex(key []byte, parts ...string) string {
	h := hmac.New(sha256.New, key)
	for _, p := range parts {
		h.Write([]byte(p))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func signedWith(h http.Header, secret, body []byte, format string) bool {
	switch format {
	case sources.FormatHub:
		return strings.EqualFold(h.Get(sources.HeaderHubSignature), "sha256="+macHex(secret, string(body)))
	case sources.FormatForgejo:
		return strings.EqualFold(h.Get(sources.HeaderForgejoSignature), macHex(secret, string(body)))
	case sources.FormatGitea:
		return strings.EqualFold(h.Get(sources.HeaderGiteaSignature), macHex(secret, string(body)))
	case sources.FormatDhow:
		for _, part := range strings.Split(h.Get(sources.HeaderDhowSignature), ",") {
			if t, ok := strings.CutPrefix(strings.TrimSpace(part), "t="); ok {
				unix, err := strconv.ParseInt(t, 10, 64)
				skew := fuzzNow.Sub(time.Unix(unix, 0))
				return err == nil && skew <= 5*time.Minute && skew >= -5*time.Minute &&
					strings.Contains(strings.ToLower(h.Get(sources.HeaderDhowSignature)), "v1="+macHex(secret, t, ".", string(body)))
			}
		}
	}
	return false
}

func FuzzWebhook(f *testing.F) {
	sha := "0123456789abcdef0123456789abcdef01234567"
	push := `{"ref":"refs/heads/main","after":"` + sha + `","pusher":{"login":"sara"}}`
	ours := `{"Ref":"refs/tags/v1","Sha":"` + sha + `"}`
	secret := "s3cret"
	stamp := strconv.FormatInt(fuzzNow.Unix(), 10)
	f.Add(sources.HeaderGiteaSignature, macHex([]byte(secret), push), "id-1", "push", secret, []byte(push))
	f.Add(sources.HeaderHubSignature, "sha256="+macHex([]byte(secret), push), "id-2", "push", secret, []byte(push))
	f.Add(sources.HeaderForgejoSignature, macHex([]byte(secret), push), "id-3", "issues", secret, []byte(push))
	f.Add(sources.HeaderDhowSignature, "t="+stamp+",v1="+macHex([]byte(secret), stamp, ".", ours), "id-4", "", secret, []byte(ours))
	f.Add(sources.HeaderDhowSignature, "t=1,v1=00", "", "", "", []byte("{}"))
	f.Add(sources.HeaderGiteaSignature, "", "id-5", "push", "x", []byte(`{"ref":"refs/heads/-x","after":"`+sha+`"}`))
	f.Fuzz(func(t *testing.T, header, signature, delivery, event, secretText string, body []byte) {
		key := []byte(secretText)
		h := http.Header{}
		h.Set(header, signature)
		h.Set(sources.HeaderGiteaDelivery, delivery)
		h.Set(sources.HeaderDhowDelivery, delivery)
		h.Set(sources.HeaderGiteaEvent, event)
		req := sources.WebhookRequest{Header: h, Body: body, Secret: key, Now: fuzzNow}
		proof, err := sources.VerifyGenericWebhook(req)
		if err != nil {
			return
		}
		if !signedWith(h, key, body, proof.Format) {
			t.Fatalf("accepted a signature that does not match: %q=%q format %s", header, signature, proof.Format)
		}
		if proof.DeliveryID == "" || len(proof.DeliveryID) > 128 {
			t.Fatalf("accepted a delivery id %q", proof.DeliveryID)
		}
		ev, err := sources.ParseGenericEvent(req, proof)
		if err != nil || ev.Ignore != "" {
			return
		}
		ref, err := gitref.Parse(ev.Ref)
		if err != nil || (ref.Kind != gitref.KindBranch && ref.Kind != gitref.KindTag) {
			t.Fatalf("accepted a ref %q", ev.Ref)
		}
		if !gitref.ValidSha(ev.Sha) || gitref.IsZeroSha(ev.Sha) {
			t.Fatalf("accepted a sha %q", ev.Sha)
		}
	})
}
