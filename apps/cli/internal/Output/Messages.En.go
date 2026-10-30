package output

var enMessages = map[Key]string{
	KeyUsage: `Usage:
  dhow lint [--format text|json] [--lang en|ar] [path]
  dhow version
  dhow help

dhow lint checks a .dhow.yml file with the same rules as the Dhow server, without a network connection.
The path is .dhow.yml in the current folder by default. If the path is a folder, dhow lint checks the .dhow.yml in it.

Exit codes:
  0  the file is valid (it may have warnings)
  1  the file has errors, and the server refuses it
  2  the file cannot be read, or the command is wrong
`,
	KeyHint:           "Run dhow help to see the commands.",
	KeyUnknownCommand: "dhow: %s is not a command.",
	KeyUnknownOption:  "dhow lint: %s is not an option.",
	KeyOptionValue:    "dhow lint: %s needs a value.",
	KeyFormatInvalid:  "dhow lint: --format is text or json.",
	KeyLangInvalid:    "dhow lint: --lang is en or ar.",
	KeyTooManyPaths:   "dhow lint: give one path only.",
	KeyFileNotFound:   "dhow lint: %s: the file does not exist.",
	KeyFileNotRegular: "dhow lint: %s: this is not a regular file.",
	KeyFileSymlink:    "dhow lint: %s: this is a symbolic link. dhow lint does not follow links, so a pull request cannot point it at another file on this machine. Lint the real file instead.",
	KeyFileNoAccess:   "dhow lint: %s: you do not have permission to read this file.",
	KeyFileUnreadable: "dhow lint: %s: the file cannot be read.",
	KeyValid:          "%s: the pipeline is valid.",
	KeyValidWarnings:  "%s: the pipeline is valid, with warnings: %d.",
	KeyInvalid:        "%s: errors: %d, warnings: %d. The server refuses this pipeline.",
	KeySeverityError:  "error",
	KeySeverityWarn:   "warning",
	KeyAt:             "at",
}
