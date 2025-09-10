package utils

import (
	"errors"
	"regexp"
	"strings"

	"../models"
)

type Validator struct {
	emailRegex *regexp.Regexp
}

func NewValidator() *Validator {
	// Simple email regex for GREEN phase
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return &Validator{
		emailRegex: emailRegex,
	}
}

func (v *Validator) ValidateUser(user *models.User) error {
	if user == nil {
		return errors.New("user cannot be nil")
	}

	if err := v.ValidateName(user.Name); err != nil {
		return err
	}

	if err := v.ValidateEmail(user.Email); err != nil {
		return err
	}

	return nil
}

func (v *Validator) ValidateName(name string) error {
	name = strings.TrimSpace(name)
	
	if name == "" {
		return errors.New("name cannot be empty")
	}

	if len(name) < 2 {
		return errors.New("name must be at least 2 characters long")
	}

	if len(name) > 100 {
		return errors.New("name cannot exceed 100 characters")
	}

	// Check for valid characters (letters, spaces, hyphens, apostrophes)
	for _, r := range name {
		if !isValidNameCharacter(r) {
			return errors.New("name contains invalid characters")
		}
	}

	return nil
}

func (v *Validator) ValidateEmail(email string) error {
	email = strings.TrimSpace(email)

	if email == "" {
		return errors.New("email cannot be empty")
	}

	if len(email) > 254 {
		return errors.New("email address too long")
	}

	if !v.emailRegex.MatchString(email) {
		return errors.New("invalid email format")
	}

	return nil
}

func (v *Validator) ValidatePassword(password string) error {
	if len(password) < 8 {
		return errors.New("password must be at least 8 characters long")
	}

	if len(password) > 128 {
		return errors.New("password cannot exceed 128 characters")
	}

	hasUpper := false
	hasLower := false
	hasDigit := false
	hasSpecial := false

	for _, r := range password {
		switch {
		case r >= 'A' && r <= 'Z':
			hasUpper = true
		case r >= 'a' && r <= 'z':
			hasLower = true
		case r >= '0' && r <= '9':
			hasDigit = true
		case isSpecialCharacter(r):
			hasSpecial = true
		}
	}

	if !hasUpper {
		return errors.New("password must contain at least one uppercase letter")
	}
	if !hasLower {
		return errors.New("password must contain at least one lowercase letter")
	}
	if !hasDigit {
		return errors.New("password must contain at least one digit")
	}
	if !hasSpecial {
		return errors.New("password must contain at least one special character")
	}

	return nil
}

func (v *Validator) SanitizeInput(input string) string {
	// Remove leading and trailing whitespace
	input = strings.TrimSpace(input)
	
	// Replace multiple consecutive spaces with single space
	spaceRegex := regexp.MustCompile(`\s+`)
	input = spaceRegex.ReplaceAllString(input, " ")
	
	return input
}

func isValidNameCharacter(r rune) bool {
	return (r >= 'a' && r <= 'z') ||
		   (r >= 'A' && r <= 'Z') ||
		   r == ' ' ||
		   r == '-' ||
		   r == '\'' ||
		   r == '.'
}

func isSpecialCharacter(r rune) bool {
	specialChars := "!@#$%^&*()_+-=[]{}|;:,.<>?"
	for _, special := range specialChars {
		if r == special {
			return true
		}
	}
	return false
}