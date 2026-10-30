package sources

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	config "github.com/Khaymah/dhow/apps/server/internal/Config"
	models "github.com/Khaymah/dhow/apps/server/internal/Modules/Sources/Models"
)

const (
	HeaderDhowSignature    = "X-Dhow-Signature"
	HeaderDhowDelivery     = "X-Dhow-Delivery"
	HeaderForgejoSignature = "X-Forgejo-Signature"
	HeaderForgejoDelivery  = "X-Forgejo-Delivery"
	HeaderForgejoEvent     = "X-Forgejo-Event"
	HeaderGiteaSignature   = "X-Gitea-Signature"
	HeaderGiteaDelivery    = "X-Gitea-Delivery"
	HeaderGiteaEvent       = "X-Gitea-Event"
	HeaderHubSignature     = "X-Hub-Signature-256"
	HeaderGitHubDelivery   = "X-GitHub-Delivery"
	HeaderGitHubEvent      = "X-GitHub-Event"

	FormatDhow    = "dhow"
	FormatForgejo = "forgejo"
	FormatGitea   = "gitea"
	FormatHub     = "hub"

	hubPrefix         = "sha256="
	maxSignatureParts = 8
	maxSignatures     = 4
	maxTimestampLen   = 12
)

var deliveryIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)

func GenericSignatureHeaders() []string {
	return []string{HeaderDhowSignature, HeaderForgejoSignature, HeaderGiteaSignature, HeaderHubSignature}
}

func mac(secret []byte, parts ...[]byte) []byte {
	h := hmac.New(sha256.New, secret)
	for _, p := range parts {
		h.Write(p)
	}
	return h.Sum(nil)
}

func single(h http.Header, name string) (string, bool) {
	values := h.Values(name)
	if len(values) != 1 {
		return "", len(values) == 0
	}
	return values[0], true
}

func VerifyGenericWebhook(r WebhookRequest) (Proof, error) {
	for _, name := range GenericSignatureHeaders() {
		value, ok := single(r.Header, name)
		if !ok {
			return Proof{}, ErrSignature
		}
		if value == "" {
			continue
		}
		var format string
		var err error
		switch name {
		case HeaderDhowSignature:
			format, err = FormatDhow, verifyDhow(value, r)
		case HeaderHubSignature:
			format, err = FormatHub, verifyHex(strings.TrimPrefix(value, hubPrefix), strings.HasPrefix(value, hubPrefix), r)
		case HeaderForgejoSignature:
			format, err = FormatForgejo, verifyHex(value, true, r)
		default:
			format, err = FormatGitea, verifyHex(value, true, r)
		}
		if err != nil {
			return Proof{}, err
		}
		id, err := deliveryID(r.Header, format)
		if err != nil {
			return Proof{}, err
		}
		return Proof{Format: format, DeliveryID: id}, nil
	}
	return Proof{}, ErrSignature
}

func verifyHex(value string, wellFormed bool, r WebhookRequest) error {
	want := mac(r.Secret, r.Body)
	got, err := hex.DecodeString(value)
	if !hmac.Equal(want, got) || err != nil || !wellFormed {
		return ErrSignature
	}
	return nil
}

func verifyDhow(value string, r WebhookRequest) error {
	parts := strings.Split(value, ",")
	if len(parts) > maxSignatureParts {
		return ErrSignature
	}
	var stamp string
	var sigs []string
	for _, part := range parts {
		key, v, ok := strings.Cut(strings.TrimSpace(part), "=")
		switch {
		case !ok:
			return ErrSignature
		case key == "t" && stamp == "":
			stamp = v
		case key == "t":
			return ErrSignature
		case key == "v1":
			sigs = append(sigs, v)
		}
	}
	if stamp == "" || len(stamp) > maxTimestampLen || len(sigs) == 0 || len(sigs) > maxSignatures {
		return ErrSignature
	}
	unix, err := strconv.ParseInt(stamp, 10, 64)
	if err != nil || strconv.FormatInt(unix, 10) != stamp {
		return ErrSignature
	}
	want := mac(r.Secret, []byte(stamp), []byte("."), r.Body)
	matched := false
	for _, s := range sigs {
		got, err := hex.DecodeString(s)
		if err == nil && hmac.Equal(want, got) {
			matched = true
		}
	}
	skew := r.Now.Sub(time.Unix(unix, 0))
	if !matched || skew > config.WebhookReplayWindow || skew < -config.WebhookReplayWindow {
		return ErrSignature
	}
	return nil
}

func deliveryID(h http.Header, format string) (string, error) {
	names := []string{HeaderForgejoDelivery, HeaderGiteaDelivery, HeaderGitHubDelivery}
	if format == FormatDhow {
		names = []string{HeaderDhowDelivery}
	}
	for _, name := range names {
		v, ok := single(h, name)
		if !ok {
			return "", ErrDeliveryID
		}
		if v == "" {
			continue
		}
		if !deliveryIDPattern.MatchString(v) {
			return "", ErrDeliveryID
		}
		return v, nil
	}
	return "", ErrDeliveryID
}

func eventHeader(h http.Header) (string, bool) {
	for _, name := range []string{HeaderForgejoEvent, HeaderGiteaEvent, HeaderGitHubEvent} {
		if v := h.Get(name); v != "" {
			return v, true
		}
	}
	return "", false
}

func ParseGenericEvent(r WebhookRequest, p Proof) (models.Event, error) {
	if p.Format == FormatDhow {
		return parseDhowPayload(r.Body)
	}
	if name, ok := eventHeader(r.Header); ok && name != "push" {
		return models.Event{Name: eventName(name), Ignore: models.ReasonEventNotSupported}, nil
	}
	return parseGiteaPush(r.Body)
}

func eventName(v string) string {
	v = strings.ToLower(v)
	if len(v) == 0 || len(v) > 32 || strings.TrimFunc(v, func(r rune) bool { return (r >= 'a' && r <= 'z') || r == '_' }) != "" {
		return "other"
	}
	return v
}
