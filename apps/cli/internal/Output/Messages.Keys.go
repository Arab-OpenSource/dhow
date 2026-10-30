package output

type Key string

const (
	KeyUsage          Key = "usage"
	KeyHint           Key = "hint"
	KeyUnknownCommand Key = "unknown.command"
	KeyUnknownOption  Key = "unknown.option"
	KeyOptionValue    Key = "option.value"
	KeyFormatInvalid  Key = "format.invalid"
	KeyLangInvalid    Key = "lang.invalid"
	KeyTooManyPaths   Key = "too.many.paths"
	KeyFileNotFound   Key = "file.not.found"
	KeyFileNotRegular Key = "file.not.regular"
	KeyFileSymlink    Key = "file.symlink"
	KeyFileNoAccess   Key = "file.no.access"
	KeyFileUnreadable Key = "file.unreadable"
	KeyValid          Key = "valid"
	KeyValidWarnings  Key = "valid.warnings"
	KeyInvalid        Key = "invalid"
	KeySeverityError  Key = "severity.error"
	KeySeverityWarn   Key = "severity.warning"
	KeyAt             Key = "at"
)
