package entity

// DomainError represents a domain-specific error.
type DomainError struct {
	message string
	code    string
}

// NewDomainError creates a new domain error.
func NewDomainError(message, code string) *DomainError {
	return &DomainError{
		message: message,
		code:    code,
	}
}

// Error implements the error interface.
func (e *DomainError) Error() string {
	return e.message
}

// Code returns the error code.
func (e *DomainError) Code() string {
	return e.code
}

// Message returns the error message.
func (e *DomainError) Message() string {
	return e.message
}
