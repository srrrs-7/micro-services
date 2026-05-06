package request

import (
	validation "github.com/go-ozzo/ozzo-validation/v4"
)

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (r LoginRequest) Validate() error {
	return validation.ValidateStruct(&r,
		validation.Field(&r.Email, validation.Required, validation.Length(5, 100)),
		validation.Field(&r.Password, validation.Required, validation.Length(6, 100)),
	)
}
