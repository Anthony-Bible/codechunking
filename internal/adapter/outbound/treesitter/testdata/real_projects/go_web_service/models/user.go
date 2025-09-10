package models

import "time"

type User struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func NewUser(name, email string) *User {
	now := time.Now()
	return &User{
		Name:      name,
		Email:     email,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func (u *User) Update(name, email string) {
	if name != "" {
		u.Name = name
	}
	if email != "" {
		u.Email = email
	}
	u.UpdatedAt = time.Now()
}

func (u *User) Validate() error {
	if u.Name == "" {
		return NewValidationError("name cannot be empty")
	}
	if u.Email == "" {
		return NewValidationError("email cannot be empty")
	}
	if !isValidEmail(u.Email) {
		return NewValidationError("invalid email format")
	}
	return nil
}

type ValidationError struct {
	Message string
}

func NewValidationError(message string) *ValidationError {
	return &ValidationError{Message: message}
}

func (e *ValidationError) Error() string {
	return e.Message
}

func isValidEmail(email string) bool {
	// Simple email validation for GREEN phase
	return len(email) > 0 && 
		   len(email) <= 254 && 
		   containsAt(email) && 
		   containsDot(email)
}

func containsAt(s string) bool {
	for _, c := range s {
		if c == '@' {
			return true
		}
	}
	return false
}

func containsDot(s string) bool {
	for _, c := range s {
		if c == '.' {
			return true
		}
	}
	return false
}