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

	// Auth error messages
	MsgPasswordIsNotValid = "Password is not valid"
	MsgInviteIsInvalid    = "Invite is invalid"
	MsgInviteIsActivated  = "Invite is activated"
	MsgUserNotFound       = "User not found"

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
)
