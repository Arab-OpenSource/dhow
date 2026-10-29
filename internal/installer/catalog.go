package installer

const (
	KeyApplyFix          Key = "apply.fix"
	KeyBlockers          Key = "blockers"
	KeyChoose            Key = "choose"
	KeyChooseInvalid     Key = "choose.invalid"
	KeyNo                Key = "no"
	KeyNone              Key = "none"
	KeyPlan              Key = "plan"
	KeyPlanCommand       Key = "plan.command"
	KeyPlanFile          Key = "plan.file"
	KeyPlanPackage       Key = "plan.package"
	KeyPlanPackagesTotal Key = "plan.packages.total"
	KeyPlanRemove        Key = "plan.remove"
	KeyProposedFix       Key = "proposed.fix"
	KeyRollbackFailed    Key = "rollback.failed"
	KeySevFail           Key = "sev.fail"
	KeySevInfo           Key = "sev.info"
	KeySevOK             Key = "sev.o.k"
	KeySevWarn           Key = "sev.warn"
	KeyStepAlreadyDone   Key = "step.already.done"
	KeyStepRolledBack    Key = "step.rolled.back"
	KeyWant              Key = "want"
	KeyYes               Key = "yes"
	KeyYesNoDefaultNo    Key = "yes.no.default.no"
	KeyYesNoDefaultYes   Key = "yes.no.default.yes"
	KeyYesNoInvalid      Key = "yes.no.invalid"
)

var engineEn = map[Key]string{
	KeyApplyFix:          "Apply the fix \"%s\"?",
	KeyBlockers:          "The install cannot continue:",
	KeyChoose:            "Number",
	KeyChooseInvalid:     "Type a number from 1 to %d.",
	KeyNo:                "no",
	KeyNone:              "none",
	KeyPlan:              "Plan:",
	KeyPlanCommand:       "run",
	KeyPlanFile:          "file",
	KeyPlanPackage:       "package",
	KeyPlanPackagesTotal: "Packages to install:",
	KeyPlanRemove:        "remove",
	KeyProposedFix:       "fix:",
	KeyRollbackFailed:    "rolling back also failed",
	KeySevFail:           "FAIL",
	KeySevInfo:           "info",
	KeySevOK:             "ok",
	KeySevWarn:           "WARN",
	KeyStepAlreadyDone:   "already done",
	KeyStepRolledBack:    "rolled back",
	KeyWant:              "want",
	KeyYes:               "yes",
	KeyYesNoDefaultNo:    "[y/N]",
	KeyYesNoDefaultYes:   "[Y/n]",
	KeyYesNoInvalid:      "Answer yes or no.",
}
