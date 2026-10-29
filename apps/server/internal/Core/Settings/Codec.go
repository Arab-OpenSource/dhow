package instancesettings

import (
	"bytes"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	settingspec "github.com/Khaymah/dhow/apps/server/internal/Config/Settings"
)

var errWrongType = errors.New("has the wrong JSON type")

func textOf(k settingspec.Key, v json.RawMessage) (string, error) {
	switch k.Kind {
	case settingspec.KindString:
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			return "", errWrongType
		}
		return s, nil
	case settingspec.KindInt:
		dec := json.NewDecoder(bytes.NewReader(v))
		dec.UseNumber()
		var n json.Number
		if err := dec.Decode(&n); err != nil {
			return "", errWrongType
		}
		if _, err := strconv.ParseInt(n.String(), 10, 64); err != nil {
			return "", errWrongType
		}
		return n.String(), nil
	case settingspec.KindBool:
		var b bool
		if err := json.Unmarshal(v, &b); err != nil {
			return "", errWrongType
		}
		return strconv.FormatBool(b), nil
	default:
		return "", errWrongType
	}
}

func jsonOf(k settingspec.Key, text string) (any, error) {
	switch k.Kind {
	case settingspec.KindString:
		return text, nil
	case settingspec.KindInt:
		n, err := strconv.ParseInt(strings.TrimSpace(text), 10, 64)
		if err != nil {
			return nil, errWrongType
		}
		return n, nil
	case settingspec.KindBool:
		b, err := strconv.ParseBool(strings.TrimSpace(text))
		if err != nil {
			return nil, errWrongType
		}
		return b, nil
	default:
		return nil, errWrongType
	}
}
