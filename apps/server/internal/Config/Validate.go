package config

import (
	"errors"
	"fmt"
	"net"
	"net/netip"

	"github.com/go-playground/validator/v10"
)

const ipv6PortTag = "ipv6_port"

var envByField = map[string]string{
	"DatabaseURL": EnvDatabaseURL,
	"SecretKey":   EnvSecretKey,
	"PublicURL":   EnvPublicURL,
	"ListenAddr":  EnvListenAddr,
}

func (c Config) Validate() error {
	v := validator.New(validator.WithRequiredStructEnabled())
	if err := v.RegisterValidation(ipv6PortTag, isIPv6Port); err != nil {
		return fmt.Errorf("register config validation: %w", err)
	}
	err := v.Struct(c)
	if err == nil {
		return nil
	}
	var verrs validator.ValidationErrors
	if !errors.As(err, &verrs) {
		return fmt.Errorf("validate configuration: %w", err)
	}
	errs := make([]error, 0, len(verrs))
	for _, fe := range verrs {
		errs = append(errs, fmt.Errorf("%s: %s", envName(fe.Field()), ruleMessage(fe)))
	}
	return errors.Join(errs...)
}

func envName(field string) string {
	if name, ok := envByField[field]; ok {
		return name
	}
	return field
}

func ruleMessage(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "is required"
	case "min":
		return fmt.Sprintf("must be at least %s bytes", fe.Param())
	case "hostname_port|" + ipv6PortTag:
		return "must be host:port"
	default:
		return "is invalid"
	}
}

func isIPv6Port(fl validator.FieldLevel) bool {
	host, port, err := net.SplitHostPort(fl.Field().String())
	if err != nil || port == "" {
		return false
	}
	addr, err := netip.ParseAddr(host)
	return err == nil && addr.Is6()
}
