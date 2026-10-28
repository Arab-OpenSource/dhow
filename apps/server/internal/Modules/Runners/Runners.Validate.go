package runners

import (
	"regexp"
	"slices"
	"strings"
	"time"

	config "github.com/Arab-OpenSource/dhow/apps/server/internal/Config"
	auth "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Auth"
	apperrors "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Errors"
	models "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Runners/Models"
)

const (
	reservedLabelPrefix = "dhow."
	messageInvalidValue = "errors.validationFailed"

	fieldName        = "Name"
	fieldLabels      = "Labels"
	fieldDescription = "Description"
	fieldMaxUses     = "MaxUses"
	fieldExpiresIn   = "ExpiresInSeconds"
	fieldBody        = "Body"
)

var (
	labelPattern   = regexp.MustCompile(`^[a-z0-9]([a-z0-9._-]{0,62}[a-z0-9])?(=[A-Za-z0-9]([A-Za-z0-9._-]{0,62}[A-Za-z0-9])?)?$`)
	versionPattern = regexp.MustCompile(`^[0-9A-Za-z][0-9A-Za-z.+_-]{0,63}$`)
	supportedOs    = []string{"linux"}
	supportedArch  = []string{"amd64", "arm64"}
	isolations     = []string{models.IsolationContainer, models.IsolationGvisor}
)

func invalid(field string) apperrors.IErrorDetail {
	return apperrors.IErrorDetail{Field: field, Message: messageInvalidValue}
}

func normalizeLabels(in []string) ([]string, bool) {
	if len(in) > config.RunnerMaxLabels {
		return nil, false
	}
	out := make([]string, 0, len(in))
	for _, l := range in {
		if len(l) > config.RunnerLabelMaxLength || !labelPattern.MatchString(l) || strings.HasPrefix(l, reservedLabelPrefix) {
			return nil, false
		}
		out = append(out, l)
	}
	slices.Sort(out)
	return slices.Compact(out), true
}

func capabilityLabels(os, arch string, levels []string) []string {
	out := []string{reservedLabelPrefix + "os=" + os, reservedLabelPrefix + "arch=" + arch}
	for _, l := range levels {
		out = append(out, reservedLabelPrefix+"isolation="+l)
	}
	return out
}

func validName(v string) bool { return auth.ValidDisplayName(v, config.RunnerNameMaxLength) }

func validRegistration(in *models.RegisterInput) string {
	switch {
	case !validName(in.Name):
		return "Name"
	case !slices.Contains(supportedOs, in.Os):
		return "Os"
	case !slices.Contains(supportedArch, in.Arch):
		return "Arch"
	case !versionPattern.MatchString(in.Version):
		return "Version"
	case in.Capacity < 1 || in.Capacity > config.RunnerMaxCapacity:
		return "Capacity"
	}
	levels := slices.Clone(in.IsolationLevels)
	slices.Sort(levels)
	levels = slices.Compact(levels)
	if len(levels) == 0 || len(levels) != len(in.IsolationLevels) {
		return "IsolationLevels"
	}
	for _, l := range levels {
		if !slices.Contains(isolations, l) {
			return "IsolationLevels"
		}
	}
	in.IsolationLevels = levels
	return ""
}

type tokenSettings struct {
	description string
	labels      []string
	maxUses     int
	ttl         time.Duration
}

func validateTokenInput(in models.TokenInput) (tokenSettings, *apperrors.AppError) {
	out := tokenSettings{labels: []string{}, maxUses: config.RunnerTokenDefaultUses, ttl: config.RunnerTokenDefaultTTL}
	var details []apperrors.IErrorDetail
	if in.Description != nil {
		out.description = strings.TrimSpace(*in.Description)
		if out.description != "" && !auth.ValidDisplayName(out.description, config.RunnerTokenDescriptionMax) {
			details = append(details, invalid(fieldDescription))
		}
	}
	if in.Labels != nil {
		labels, ok := normalizeLabels(*in.Labels)
		if !ok {
			details = append(details, invalid(fieldLabels))
		}
		out.labels = labels
	}
	if in.MaxUses != nil {
		out.maxUses = *in.MaxUses
		if out.maxUses < 1 || out.maxUses > config.RunnerTokenMaxUses {
			details = append(details, invalid(fieldMaxUses))
		}
	}
	if in.ExpiresIn != nil {
		out.ttl = time.Duration(*in.ExpiresIn) * time.Second
		if out.ttl < config.RunnerTokenMinTTL || out.ttl > config.RunnerTokenMaxTTL {
			details = append(details, invalid(fieldExpiresIn))
		}
	}
	if len(details) > 0 {
		return tokenSettings{}, apperrors.ValidationFailed(details...)
	}
	return out, nil
}

func validateUpdate(in *models.UpdateInput) *apperrors.AppError {
	if in.Name == nil && in.Labels == nil && in.Disabled == nil {
		return apperrors.ValidationFailed(invalid(fieldBody))
	}
	var details []apperrors.IErrorDetail
	if in.Name != nil && !validName(*in.Name) {
		details = append(details, invalid(fieldName))
	}
	if in.Labels != nil {
		labels, ok := normalizeLabels(*in.Labels)
		if !ok {
			details = append(details, invalid(fieldLabels))
		}
		in.Labels = &labels
	}
	if len(details) > 0 {
		return apperrors.ValidationFailed(details...)
	}
	return nil
}
