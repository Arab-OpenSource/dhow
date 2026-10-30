package sources

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"

	models "github.com/Khaymah/dhow/apps/server/internal/Modules/Sources/Models"
	"github.com/Khaymah/dhow/internal/gitref"
)

const maxActorRunes = 255

type dhowPayload struct {
	Ref string `json:"Ref"`
	Sha string `json:"Sha"`
}

type giteaUser struct {
	Login    string `json:"Login"`
	Username string `json:"Username"`
}

type giteaPush struct {
	Ref    string    `json:"Ref"`
	After  string    `json:"After"`
	Pusher giteaUser `json:"Pusher"`
}

func decodeStrict(body []byte, into any) error {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(into); err != nil {
		return ErrPayload
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return ErrPayload
	}
	return nil
}

func decodeLoose(body []byte, into any) error {
	dec := json.NewDecoder(bytes.NewReader(body))
	if err := dec.Decode(into); err != nil {
		return ErrPayload
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return ErrPayload
	}
	return nil
}

func parseDhowPayload(body []byte) (models.Event, error) {
	var p dhowPayload
	if err := decodeStrict(body, &p); err != nil {
		return models.Event{}, err
	}
	return eventOf(p.Ref, p.Sha, "")
}

func parseGiteaPush(body []byte) (models.Event, error) {
	var p giteaPush
	if err := decodeLoose(body, &p); err != nil {
		return models.Event{}, err
	}
	actor := p.Pusher.Login
	if actor == "" {
		actor = p.Pusher.Username
	}
	return eventOf(p.Ref, p.After, actor)
}

func eventOf(ref, sha, actor string) (models.Event, error) {
	parsed, err := gitref.Parse(ref)
	if err != nil || (parsed.Kind != gitref.KindBranch && parsed.Kind != gitref.KindTag) {
		return models.Event{}, ErrPayload
	}
	if !gitref.ValidSha(sha) {
		return models.Event{}, ErrPayload
	}
	name := "push"
	if parsed.Kind == gitref.KindTag {
		name = "tag"
	}
	ev := models.Event{Name: name, Ref: ref, Sha: sha, Actor: CleanActor(actor)}
	if gitref.IsZeroSha(sha) {
		ev.Sha = ""
		ev.Ignore = models.ReasonRefDeleted
	}
	return ev, nil
}

func CleanActor(v string) string {
	if !utf8.ValidString(v) {
		return ""
	}
	var b strings.Builder
	n := 0
	for _, r := range v {
		if n >= maxActorRunes {
			break
		}
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			continue
		}
		b.WriteRune(r)
		n++
	}
	return strings.TrimSpace(b.String())
}
