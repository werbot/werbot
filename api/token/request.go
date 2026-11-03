package token

// registerRequest is the public registration body.
type registerRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

// acceptRequest is the public invite acceptance body. Password, name and
// surname are only required when the invite has no profile attached yet.
type acceptRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
	Name     string `json:"name"`
	Surname  string `json:"surname"`
}

// addProjectTokenRequest invites a new or existing profile to a project.
type addProjectTokenRequest struct {
	Email     string `json:"email"`
	Name      string `json:"name"`
	Surname   string `json:"surname"`
	ProfileID string `json:"profile_id"`
}

// addSchemeTokenRequest grants an existing or new profile access to a scheme.
// project_id is accepted for compatibility but is not needed: ownership is
// validated through the scheme's project.
type addSchemeTokenRequest struct {
	Email     string `json:"email"`
	ProfileID string `json:"profile_id"`
}

// addProfileTokenRequest creates an admin registration invite.
type addProfileTokenRequest struct {
	Name    string `json:"name"`
	Surname string `json:"surname"`
	Email   string `json:"email"`
}
