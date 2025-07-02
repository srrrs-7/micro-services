package entity

import "github.com/google/uuid"

type SessionID string

func NewSessionID() SessionID {
	return SessionID(uuid.New().String())
}

func (s SessionID) String() string {
	return string(s)
}
