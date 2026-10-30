package installer

const (
	KeyAdviceMPTCP         Key = "advice.mptcp"
	KeyApplyFix            Key = "apply.fix"
	KeyBlockers            Key = "blockers"
	KeyChoose              Key = "choose"
	KeyCheckMPTCP          Key = "check.mptcp"
	KeyChooseInvalid       Key = "choose.invalid"
	KeyDoctorAdvisories    Key = "doctor.advisories"
	KeyKernelAdvisoriesURL Key = "kernel.advisories.url"
	KeyKernelUnknown       Key = "kernel.unknown"
	KeyMPTCPAbsent         Key = "mptcp.absent"
	KeyMPTCPOff            Key = "mptcp.off"
	KeyMPTCPOn             Key = "mptcp.on"
	KeyMPTCPUnreadable     Key = "mptcp.unreadable"
	KeyNo                  Key = "no"
	KeyNone                Key = "none"
	KeyPlan                Key = "plan"
	KeyPlanCommand         Key = "plan.command"
	KeyPlanFile            Key = "plan.file"
	KeyPlanPackage         Key = "plan.package"
	KeyPlanPackagesTotal   Key = "plan.packages.total"
	KeyPlanRemove          Key = "plan.remove"
	KeyProposedFix         Key = "proposed.fix"
	KeyRollbackFailed      Key = "rollback.failed"
	KeySevFail             Key = "sev.fail"
	KeySevInfo             Key = "sev.info"
	KeySevOK               Key = "sev.o.k"
	KeySevWarn             Key = "sev.warn"
	KeyStepAlreadyDone     Key = "step.already.done"
	KeyStepRolledBack      Key = "step.rolled.back"
	KeyWant                Key = "want"
	KeyYes                 Key = "yes"
	KeyYesNoDefaultNo      Key = "yes.no.default.no"
	KeyYesNoDefaultYes     Key = "yes.no.default.yes"
	KeyYesNoInvalid        Key = "yes.no.invalid"
)

var engineEn = map[Key]string{
	KeyAdviceMPTCP:         "Dhow turns MPTCP off in its own listeners, but other services on this host may still use it. Install your distribution's kernel update that fixes CVE-2026-98432, or turn MPTCP off with: sysctl -w net.mptcp.enabled=0. The kernel version alone does not show whether the fix is in, because distributions backport it. Dhow does not change this setting for you. See %s",
	KeyApplyFix:            "Apply the fix \"%s\"?",
	KeyBlockers:            "The install cannot continue:",
	KeyChoose:              "Number",
	KeyCheckMPTCP:          "Kernel multipath TCP (MPTCP, CVE-2026-98432)",
	KeyChooseInvalid:       "Type a number from 1 to %d.",
	KeyDoctorAdvisories:    "Nothing blocks Dhow, but read the warnings above.",
	KeyKernelAdvisoriesURL: "https://github.com/Khaymah/dhow/blob/main/docs/en/security/kernel-advisories.md",
	KeyKernelUnknown:       "unknown",
	KeyMPTCPAbsent:         "This kernel has no MPTCP. Kernel %s.",
	KeyMPTCPOff:            "MPTCP is off on this host (net.mptcp.enabled=0). Kernel %s.",
	KeyMPTCPOn:             "MPTCP is on on this host (net.mptcp.enabled=%s). Kernel %s.",
	KeyMPTCPUnreadable:     "Could not read %s, so MPTCP was not checked. Kernel %s.",
	KeyNo:                  "no",
	KeyNone:                "none",
	KeyPlan:                "Plan:",
	KeyPlanCommand:         "run",
	KeyPlanFile:            "file",
	KeyPlanPackage:         "package",
	KeyPlanPackagesTotal:   "Packages to install:",
	KeyPlanRemove:          "remove",
	KeyProposedFix:         "fix:",
	KeyRollbackFailed:      "rolling back also failed",
	KeySevFail:             "FAIL",
	KeySevInfo:             "info",
	KeySevOK:               "ok",
	KeySevWarn:             "WARN",
	KeyStepAlreadyDone:     "already done",
	KeyStepRolledBack:      "rolled back",
	KeyWant:                "want",
	KeyYes:                 "yes",
	KeyYesNoDefaultNo:      "[y/N]",
	KeyYesNoDefaultYes:     "[Y/n]",
	KeyYesNoInvalid:        "Answer yes or no.",
}
