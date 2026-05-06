package domain

type User struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func NewUser(id, email, password string) User {
	return User{
		ID:       id,
		Email:    email,
		Password: password,
	}
}

type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func NewLoginInput(email, password string) LoginInput {
	return LoginInput{
		Email:    email,
		Password: password,
	}
}
