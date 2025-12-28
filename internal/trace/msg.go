package trace

const (
	// Default error messages
	MsgInvalidArgument     = "Invalid argument"
	MsgNotFound            = "Not found"
	MsgAlreadyExists       = "Already exists"
	MsgPermissionDenied    = "Permission denied"
	MsgAborted             = "Aborted due to internal error"
	MsgUnknownError        = "Unknown error"
	MsgBadRequest          = "Bad request"
	MsgUnauthorized        = "Unauthorized"
	MsgInternalServerError = "Internal server error"

	MsgSettingNotFound = "Setting not found"

	MsgSectiNotFound  = "Section not found"
	MsgActionNotFound = "Action not found"
	MsgStatusNotFound = "Status not found"

	// Auth error messages
	MsgPasswordIsNotValid      = "Password is not valid"
	MsgTokenIsInvalid          = "Token is invalid"
	MsgTokenIsActivated        = "Token is activated"
	MsgTokenNotFound           = "Token not found"
	MsgUserNotFound            = "User not found"
	MsgAuthMethodDoesNotMatch  = "The authentication method does not match"
	MsgPPasswordHashingProblem = "Password hashing problem"

	// Action error messages
	MsgFailedToAdd    = "Failed to add"
	MsgFailedToUpdate = "Failed to update"
	MsgFailedToDelete = "Failed to delete"

	// Validate error messages
	MsgFailedToValidateParams = "Failed to validate params"
	MsgFailedToValidateBody   = "Failed to validate body params"
	MsgFailedToValidateQuery  = "Failed to validate query params"

	// Transaction error messages
	MsgTransactionCreateError = "Transaction create error"
	MsgTransactionCommitError = "Transaction commit error"

	// Firewall error messages
	MsgAccessIsDeniedUser    = "Access is denied for this user"
	MsgAccessIsDeniedTime    = "Access is denied for this time"
	MsgAccessIsDeniedCountry = "Access is denied for this country"
	MsgAccessIsDeniedIP      = "Access is denied for this ip"

	// Key error messages
	MsgKeyNotFound          = "Key not found"
	MsgPublicKeyIsBroken    = "The public key has a broken"
	MsgPrivateKeyIsBroken   = "The private key has a broken"
	MsgFailedCreatingSSHKey = "Failed to creating SSH key"

	// License error messages
	MsgFailedToOpenLicenseFile  = "Failed to open license file"
	MsgLicenseKeyIsBroken       = "The license key has a broken"
	MsgLicenseStructureIsBroken = "The license has a broken"

	// Other error messages
	MsgFailedToOpenFile  = "Failed to open file"
	MsgStructureIsBroken = "Structure is broken"
	// MsgIncorrectParams   = "Incorrect parameters"

	// Member error messages
	MsgMemberNotFound = "Member not found"
	MsgOwnerNotFound  = "Owner not found"

	// Consumer error messages
	MsgConsumerNotFound = "Consumer not found"

	// Subscription error messages
	MsgSubscriptionPlanNotFound = "Subscription plan not found"

	// Account error messages
	MsgAccountNotFound = "Account not found"

	// Audit error messages
	MsgAuditNotFound = "Audit not found"

	// Firewall error messages
	MsgFirewallNotFound     = "Firewall not found"
	MsgCountryNotFound      = "Country not found"
	MsgNetworkNotFound      = "Network not found"
	MsgFirewallListNotFound = "Firewall list not found"

	// Project error messages
	MsgProfileNotFound = "Profile not found"

	// Project error messages
	MsgProjectNotFound = "Project not found"

	// Scheme error messages
	MsgSchemeNotFound        = "Scheme not found"
	MsgSchemeSettingNotFound = "Scheme setting not found"
	MsgSchemeDiffers         = "Schemes differs"
	MsgInvalidAuthType       = "Invalid authorization type"
)
