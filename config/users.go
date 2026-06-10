package config

import (
	"encoding/json"
	"fmt"
	"strings"
)

// APIUser is one credential in the API_USERS registry: an API key bound to a
// user_id and role. role "admin" sees every OLT; "user" (default) is scoped to
// OLTs whose user_id matches. When API_USERS is unset, per-user auth is off and
// the legacy single API_KEY applies unchanged.
type APIUser struct {
	UserID int
	APIKey string
	Role   string // "user" | "admin"
}

// IsAdmin reports whether this user has the admin role.
func (u APIUser) IsAdmin() bool { return u.Role == "admin" }

// apiUserJSON is the wire shape of one entry in the API_USERS JSON array.
type apiUserJSON struct {
	UserID int    `json:"user_id"`
	APIKey string `json:"api_key"`
	Role   string `json:"role"`
}

// buildUserRegistry parses the API_USERS JSON array into a map keyed by API key
// for O(1) lookup during authentication. Empty input returns (nil, nil): per-user
// auth is disabled and the caller falls back to the legacy single API_KEY.
//
// Validation is fail-fast: malformed JSON, an empty array, a missing/blank
// api_key, an unknown role, or duplicate api_keys all abort startup rather than
// silently weakening access control.
func buildUserRegistry(apiUsersJSON string) (map[string]APIUser, error) {
	if strings.TrimSpace(apiUsersJSON) == "" {
		return nil, nil
	}

	var entries []apiUserJSON
	if err := json.Unmarshal([]byte(apiUsersJSON), &entries); err != nil {
		return nil, fmt.Errorf("invalid API_USERS JSON: %w", err)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("API_USERS must contain at least one user")
	}

	reg := make(map[string]APIUser, len(entries))
	for i, e := range entries {
		if strings.TrimSpace(e.APIKey) == "" {
			return nil, fmt.Errorf("API_USERS[%d]: api_key is required", i)
		}
		role := strings.ToLower(strings.TrimSpace(e.Role))
		if role == "" {
			role = "user"
		}
		if role != "user" && role != "admin" {
			return nil, fmt.Errorf("API_USERS[%d]: invalid role %q (allowed: user, admin)", i, e.Role)
		}
		// A non-admin must own a real user_id (>=1). user_id 0 is "unowned"
		// (admin-only) on the OLT side, so allowing a user_id-0 user would let a
		// misconfigured entry see every unowned OLT — fail fast instead.
		if role != "admin" && e.UserID < 1 {
			return nil, fmt.Errorf("API_USERS[%d]: user_id must be >= 1 for role %q", i, role)
		}
		if _, dup := reg[e.APIKey]; dup {
			return nil, fmt.Errorf("API_USERS[%d]: duplicate api_key", i)
		}
		reg[e.APIKey] = APIUser{UserID: e.UserID, APIKey: e.APIKey, Role: role}
	}
	return reg, nil
}
