package authhttp

import (
	"github.com/p30huiwei/alive/backend/internal/auth"
)

// loginRequest is the login body.
//
// Both fields are required. The binding tags reject a missing field before the
// service is reached, which is the difference between a 400 for a malformed
// request and a 401 for a refused one.
//
// No length limits here. A password shorter than the minimum is not a bad
// request, it is a wrong password, and reporting the two differently would tell
// an attacker how long the real password is not.
type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// userResponse is the owner account as the API reports it.
//
// The field list is deliberate. There is no password hash and no session token:
// the hash never leaves the repository, and the token exists only in the
// Set-Cookie header of the login response.
//
// Role is reported because it is a fact about the account and admin may want to
// show it. Nothing in this codebase branches on it; being visible in a response
// and being consulted for a decision are different things.
type userResponse struct {
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	Role        string `json:"role"`
	DisplayName string `json:"display_name"`
}

// newUserResponse converts a domain user into the response shape.
//
// A conversion rather than JSON tags on auth.User. The domain type is free to
// change without changing the API, and a field added to it is not published by
// accident.
func newUserResponse(user auth.User) userResponse {
	return userResponse{
		ID:          user.ID,
		Username:    user.Username,
		Role:        user.Role,
		DisplayName: user.DisplayName,
	}
}
