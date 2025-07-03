package entity

import "time"

type UserID string
type Scope string
type Role string
type Expired time.Time

type AccessToken Token
type RefreshToken Token

type Token struct {
	UserID  UserID  `json:"user_id,omitempty"`
	Scope   Scope   `json:"scope,omitempty"`
	Role    Role    `json:"role,omitempty"`
	Expired Expired `json:"expired,omitempty"`
}

func NewAccessToken(uid UserID, scope Scope, role Role) AccessToken {
	return AccessToken(createToken(uid, scope, role, 15*time.Minute))
}

func NewRefreshToken(uid UserID, scope Scope, role Role) RefreshToken {
	return RefreshToken(createToken(uid, scope, role, 12*time.Hour))
}

func createToken(uid UserID, scope Scope, role Role, expired time.Duration) Token {
	return Token{
		UserID:  uid,
		Scope:   scope,
		Role:    role,
		Expired: Expired(time.Now().Add(time.Duration(expired))),
	}
}
