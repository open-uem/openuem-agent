package deploy

import "errors"

// ErrNoLoggedInUser is returned by deploy actions that must run in a user
// session (e.g., brew on macOS) when no user is logged in the console
var ErrNoLoggedInUser = errors.New("no logged in user")
