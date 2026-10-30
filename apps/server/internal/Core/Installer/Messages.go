package installer

import (
	engine "github.com/Khaymah/dhow/internal/installer"
)

type (
	Key      = engine.Key
	Messages = engine.Messages
)

const (
	KeyApplyFix          = engine.KeyApplyFix
	KeyBlockers          = engine.KeyBlockers
	KeyChoose            = engine.KeyChoose
	KeyChooseInvalid     = engine.KeyChooseInvalid
	KeyNo                = engine.KeyNo
	KeyNone              = engine.KeyNone
	KeyPlan              = engine.KeyPlan
	KeyPlanCommand       = engine.KeyPlanCommand
	KeyPlanFile          = engine.KeyPlanFile
	KeyPlanPackage       = engine.KeyPlanPackage
	KeyPlanPackagesTotal = engine.KeyPlanPackagesTotal
	KeyPlanRemove        = engine.KeyPlanRemove
	KeyProposedFix       = engine.KeyProposedFix
	KeyRollbackFailed    = engine.KeyRollbackFailed
	KeySevFail           = engine.KeySevFail
	KeySevInfo           = engine.KeySevInfo
	KeySevOK             = engine.KeySevOK
	KeySevWarn           = engine.KeySevWarn
	KeyStepAlreadyDone   = engine.KeyStepAlreadyDone
	KeyStepRolledBack    = engine.KeyStepRolledBack
	KeyWant              = engine.KeyWant
	KeyYes               = engine.KeyYes
	KeyYesNoDefaultNo    = engine.KeyYesNoDefaultNo
	KeyYesNoDefaultYes   = engine.KeyYesNoDefaultYes
	KeyYesNoInvalid      = engine.KeyYesNoInvalid
)

func NewMessages(lang string) Messages { return engine.NewMessages(lang, enMessages, arMessages) }

func ChooseLang(flagLang, answersLang, locale string) string {
	return engine.ChooseLang(flagLang, answersLang, locale)
}

func MessageKeys(lang string) []Key {
	if lang == LangAr {
		return engine.Keys(arMessages)
	}
	return engine.Keys(enMessages)
}
