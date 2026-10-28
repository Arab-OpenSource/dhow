package jobspec

import (
	"errors"
	"net/netip"
	"strconv"
	"strings"
)

const (
	DefaultTLSPort  = 443
	DefaultHTTPPort = 80
	maxHostLength   = 253
	wildcardPrefix  = "*."
)

var (
	ErrEgressEmpty    = errors.New("an egress entry is empty")
	ErrEgressTooLong  = errors.New("an egress entry is too long")
	ErrEgressPort     = errors.New("an egress port must be a number from 1 to 65535 without leading zeros")
	ErrEgressIP       = errors.New("an egress entry must name a host, not an IP address")
	ErrEgressHostname = errors.New("an egress host must be lowercase ASCII labels (punycode for other scripts)")
)

type EgressRule struct {
	Host     string
	Wildcard bool
	Port     int
}

func ParseEgress(entry string) (EgressRule, error) {
	if entry == "" {
		return EgressRule{}, ErrEgressEmpty
	}
	if len(entry) > MaxEgressEntryLength {
		return EgressRule{}, ErrEgressTooLong
	}
	host, portText, hasPort := strings.Cut(entry, ":")
	rule := EgressRule{}
	if hasPort {
		port, ok := parsePort(portText)
		if !ok {
			return EgressRule{}, ErrEgressPort
		}
		rule.Port = port
	}
	if strings.HasPrefix(host, wildcardPrefix) {
		rule.Wildcard = true
		host = strings.TrimPrefix(host, wildcardPrefix)
	}
	if err := checkHostname(host); err != nil {
		return EgressRule{}, err
	}
	rule.Host = host
	return rule, nil
}

func (r EgressRule) Allows(host string, port int, tunnel bool) bool {
	want := r.Port
	if want == 0 {
		want = DefaultHTTPPort
		if tunnel {
			want = DefaultTLSPort
		}
	}
	if port != want {
		return false
	}
	if r.Wildcard {
		return strings.HasSuffix(host, "."+r.Host)
	}
	return host == r.Host
}

func NormalizeHost(host string) (string, error) {
	h := strings.TrimSuffix(strings.ToLower(host), ".")
	if err := checkHostname(h); err != nil {
		return "", err
	}
	return h, nil
}

func checkHostname(host string) error {
	if host == "" || len(host) > maxHostLength {
		return ErrEgressHostname
	}
	if _, err := netip.ParseAddr(strings.Trim(host, "[]")); err == nil {
		return ErrEgressIP
	}
	labels := strings.Split(host, ".")
	for _, label := range labels {
		if !validLabel(label) {
			return ErrEgressHostname
		}
	}
	last := labels[len(labels)-1]
	if allDigits(last) || strings.HasPrefix(last, "0x") {
		return ErrEgressIP
	}
	return nil
}

func validLabel(label string) bool {
	if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
		return false
	}
	for i := 0; i < len(label); i++ {
		c := label[i]
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
			return false
		}
	}
	return true
}

func allDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return s != ""
}

func parsePort(text string) (int, bool) {
	n, err := strconv.Atoi(text)
	if err != nil || n < 1 || n > maxPort || strconv.Itoa(n) != text {
		return 0, false
	}
	return n, true
}
