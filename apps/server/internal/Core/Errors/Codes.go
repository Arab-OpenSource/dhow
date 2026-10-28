package apperrors

const (
	CodeInternal           = "INTERNAL"
	CodeNotFound           = "NOT_FOUND"
	CodeMethodNotAllowed   = "METHOD_NOT_ALLOWED"
	CodeBadRequest         = "BAD_REQUEST"
	CodeValidationFailed   = "VALIDATION_FAILED"
	CodePayloadTooLarge    = "PAYLOAD_TOO_LARGE"
	CodeServiceUnavailable = "SERVICE_UNAVAILABLE"
)

const (
	CodeUnauthenticated        = "UNAUTHENTICATED"
	CodeForbidden              = "FORBIDDEN"
	CodeCSRFFailed             = "CSRF_FAILED"
	CodeInvalidCredentials     = "INVALID_CREDENTIALS"      //nolint:gosec // A public error code, not a credential.
	CodeInvalidCurrentPassword = "INVALID_CURRENT_PASSWORD" //nolint:gosec // A public error code, not a credential.
	CodePasswordBreached       = "PASSWORD_BREACHED"
	CodeSetupCompleted         = "SETUP_COMPLETED"
	CodeSetupTokenInvalid      = "SETUP_TOKEN_INVALID" //nolint:gosec // A public error code, not a credential.
	CodeConflict               = "CONFLICT"
	CodeMfaChallengeExpired    = "MFA_CHALLENGE_EXPIRED"
	CodeInvalidMfaCode         = "INVALID_MFA_CODE"
	CodeInvalidCurrentFactor   = "INVALID_CURRENT_FACTOR"
	CodeEnrollmentExpired      = "ENROLLMENT_EXPIRED"
	CodePasskeyRejected        = "PASSKEY_REJECTED"
	CodeRunnerRevoked          = "RUNNER_REVOKED"
	CodeSecretsUnavailable     = "SECRETS_UNAVAILABLE"
	CodeJobFinished            = "JOB_FINISHED"
	CodeJobQueueFull           = "JOB_QUEUE_FULL"
	CodeAttemptNotFound        = "ATTEMPT_NOT_FOUND"
	CodeTooManyStreams         = "TOO_MANY_STREAMS"
)

const (
	CodeLastOwner               = "LAST_OWNER"
	CodeRoleNotAllowed          = "ROLE_NOT_ALLOWED"
	CodeAlreadyMember           = "ALREADY_MEMBER"
	CodeInvitationPending       = "INVITATION_PENDING"
	CodeInvitationInvalid       = "INVITATION_INVALID"
	CodeInvitationEmailMismatch = "INVITATION_EMAIL_MISMATCH"
	CodeInvitationAccountExists = "INVITATION_ACCOUNT_EXISTS"
)

const (
	KeyInternal           = "errors.internal"
	KeyNotFound           = "errors.notFound"
	KeyMethodNotAllowed   = "errors.methodNotAllowed"
	KeyBadRequest         = "errors.badRequest"
	KeyValidationFailed   = "errors.validationFailed"
	KeyPayloadTooLarge    = "errors.payloadTooLarge"
	KeyServiceUnavailable = "errors.serviceUnavailable"
)

const (
	KeyUnauthenticated        = "errors.unauthenticated"
	KeyForbidden              = "errors.forbidden"
	KeyCSRFFailed             = "errors.csrfFailed"
	KeyInvalidCredentials     = "errors.invalidCredentials"
	KeyInvalidCurrentPassword = "errors.invalidCurrentPassword"
	KeyPasswordBreached       = "errors.passwordBreached"
	KeySetupCompleted         = "errors.setupCompleted"
	KeySetupTokenInvalid      = "errors.setupTokenInvalid"
	KeyConflict               = "errors.conflict"
	KeyMfaChallengeExpired    = "errors.mfaChallengeExpired"
	KeyInvalidMfaCode         = "errors.invalidMfaCode"
	KeyInvalidCurrentFactor   = "errors.invalidCurrentFactor"
	KeyEnrollmentExpired      = "errors.enrollmentExpired"
	KeyPasskeyRejected        = "errors.passkeyRejected"
	KeyRunnerRevoked          = "errors.runnerRevoked"
	KeySecretsUnavailable     = "errors.secretsUnavailable"
	KeyJobFinished            = "errors.jobFinished"
	KeyJobQueueFull           = "errors.jobQueueFull"
	KeyAttemptNotFound        = "errors.attemptNotFound"
	KeyTooManyStreams         = "errors.tooManyStreams"
)

const (
	KeyLastOwner               = "errors.lastOwner"
	KeyRoleNotAllowed          = "errors.roleNotAllowed"
	KeyAlreadyMember           = "errors.alreadyMember"
	KeyInvitationPending       = "errors.invitationPending"
	KeyInvitationInvalid       = "errors.invitationInvalid"
	KeyInvitationEmailMismatch = "errors.invitationEmailMismatch"
	KeyInvitationAccountExists = "errors.invitationAccountExists"
)
