// Package services provides business logic services for the application.
// This package contains various service implementations that handle
// the core business operations and coordinate between different layers.
package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ServiceError represents a service-level error with additional context.
// This error type provides structured information about what went wrong
// and can be used for logging, monitoring, and client responses.
type ServiceError struct {
	// Code represents the error code for categorization
	Code string `json:"code"`
	// Message is the human-readable error description
	Message string `json:"message"`
	// Context provides additional information about the error
	Context map[string]interface{} `json:"context,omitempty"`
	// Timestamp indicates when the error occurred
	Timestamp time.Time `json:"timestamp"`
	// Cause is the underlying error that caused this service error
	Cause error `json:"-"`
}

// Error implements the error interface for ServiceError.
// It returns a formatted string representation of the error.
func (e *ServiceError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap returns the underlying cause of the error.
// This method enables error unwrapping introduced in Go 1.13.
func (e *ServiceError) Unwrap() error {
	return e.Cause
}

// NewServiceError creates a new ServiceError with the given parameters.
// This constructor ensures that all required fields are set properly.
//
// Parameters:
//   - code: A string identifier for the error type
//   - message: A human-readable description of the error
//   - cause: The underlying error that caused this service error (can be nil)
//
// Returns:
//   - *ServiceError: A pointer to the newly created ServiceError
func NewServiceError(code, message string, cause error) *ServiceError {
	return &ServiceError{
		Code:      code,
		Message:   message,
		Cause:     cause,
		Timestamp: time.Now(),
		Context:   make(map[string]interface{}),
	}
}

// UserRepository defines the contract for user data persistence operations.
// This interface abstracts the data layer and enables dependency injection
// for testing and different storage implementations.
type UserRepository interface {
	// GetByID retrieves a user by their unique identifier.
	// Returns ErrUserNotFound if the user does not exist.
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)

	// GetByEmail retrieves a user by their email address.
	// Email lookup is case-insensitive and returns ErrUserNotFound if not found.
	GetByEmail(ctx context.Context, email string) (*User, error)

	// Create persists a new user to the data store.
	// The user ID will be generated if not provided.
	// Returns ErrDuplicateEmail if the email already exists.
	Create(ctx context.Context, user *User) error

	// Update modifies an existing user in the data store.
	// All fields except ID and timestamps are updateable.
	Update(ctx context.Context, user *User) error

	// Delete removes a user from the data store.
	// This performs a hard delete - for soft delete, use the service layer.
	Delete(ctx context.Context, id uuid.UUID) error

	// List retrieves users with optional filtering and pagination.
	// The options parameter controls sorting, filtering, and pagination behavior.
	List(ctx context.Context, opts ListOptions) ([]*User, int, error)
}

// User represents a user entity in the system with full metadata.
// This struct contains all user-related information including
// profile data, authentication details, and audit timestamps.
type User struct {
	// ID is the unique identifier for the user
	ID uuid.UUID `json:"id" db:"id"`

	// Email is the user's email address (unique)
	Email string `json:"email" db:"email" validate:"required,email"`

	// FirstName is the user's given name
	FirstName string `json:"first_name" db:"first_name" validate:"required,min=1,max=50"`

	// LastName is the user's family name
	LastName string `json:"last_name" db:"last_name" validate:"required,min=1,max=50"`

	// PasswordHash stores the hashed password (never expose in JSON)
	PasswordHash string `json:"-" db:"password_hash"`

	// IsActive indicates if the user account is active
	IsActive bool `json:"is_active" db:"is_active"`

	// IsAdmin indicates if the user has administrative privileges
	IsAdmin bool `json:"is_admin" db:"is_admin"`

	// EmailVerified indicates if the user's email has been verified
	EmailVerified bool `json:"email_verified" db:"email_verified"`

	// Profile contains optional user profile information
	Profile *UserProfile `json:"profile,omitempty"`

	// CreatedAt is the timestamp when the user was created
	CreatedAt time.Time `json:"created_at" db:"created_at"`

	// UpdatedAt is the timestamp when the user was last updated
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`

	// DeletedAt is the soft deletion timestamp (nil if not deleted)
	DeletedAt *time.Time `json:"deleted_at,omitempty" db:"deleted_at"`

	// LastLoginAt is the timestamp of the user's last login
	LastLoginAt *time.Time `json:"last_login_at,omitempty" db:"last_login_at"`

	// LoginAttempts tracks failed login attempts for security
	LoginAttempts int `json:"login_attempts" db:"login_attempts"`

	// LockedUntil contains the timestamp until when the account is locked
	LockedUntil *time.Time `json:"locked_until,omitempty" db:"locked_until"`
}

// UserProfile contains extended user profile information.
// This struct holds optional profile data that enhances the user experience.
type UserProfile struct {
	// Bio is a short biographical description
	Bio string `json:"bio,omitempty" db:"bio"`

	// AvatarURL is the URL to the user's profile picture
	AvatarURL string `json:"avatar_url,omitempty" db:"avatar_url" validate:"omitempty,url"`

	// WebsiteURL is the user's personal or professional website
	WebsiteURL string `json:"website_url,omitempty" db:"website_url" validate:"omitempty,url"`

	// Location is the user's geographical location
	Location string `json:"location,omitempty" db:"location"`

	// Phone is the user's phone number
	Phone string `json:"phone,omitempty" db:"phone" validate:"omitempty,phone"`

	// DateOfBirth is the user's birth date (optional)
	DateOfBirth *time.Time `json:"date_of_birth,omitempty" db:"date_of_birth"`

	// Preferences stores user preferences as JSON
	Preferences map[string]interface{} `json:"preferences,omitempty" db:"preferences"`
}

// FullName returns the user's complete name by combining first and last name.
// This method handles cases where either name component might be empty.
func (u *User) FullName() string {
	name := fmt.Sprintf("%s %s", u.FirstName, u.LastName)
	return strings.TrimSpace(name)
}

// IsDeleted returns true if the user has been soft deleted.
// A user is considered deleted if the DeletedAt timestamp is set.
func (u *User) IsDeleted() bool {
	return u.DeletedAt != nil
}

// IsLocked returns true if the user account is currently locked.
// An account is locked if LockedUntil is set and is in the future.
func (u *User) IsLocked() bool {
	return u.LockedUntil != nil && u.LockedUntil.After(time.Now())
}

// CanLogin returns true if the user is eligible to log in.
// This checks various conditions including active status, deletion, and locking.
func (u *User) CanLogin() bool {
	return u.IsActive && !u.IsDeleted() && !u.IsLocked()
}

// UpdateLoginAttempts increments the failed login attempts counter.
// If the attempts exceed the maximum allowed, the account will be locked.
//
// Parameters:
//   - failed: whether the login attempt failed (true) or succeeded (false)
//   - maxAttempts: maximum allowed failed attempts before locking
//   - lockDuration: duration to lock the account after exceeding max attempts
func (u *User) UpdateLoginAttempts(failed bool, maxAttempts int, lockDuration time.Duration) {
	if failed {
		u.LoginAttempts++
		if u.LoginAttempts >= maxAttempts {
			lockUntil := time.Now().Add(lockDuration)
			u.LockedUntil = &lockUntil
		}
	} else {
		// Reset attempts on successful login
		u.LoginAttempts = 0
		u.LockedUntil = nil
		now := time.Now()
		u.LastLoginAt = &now
	}
	u.UpdatedAt = time.Now()
}

// Validate performs comprehensive validation on the user data.
// This method checks all required fields and validates formats.
//
// Returns:
//   - error: validation error if any field is invalid, nil otherwise
func (u *User) Validate() error {
	if u.Email == "" {
		return NewServiceError("INVALID_EMAIL", "Email is required", nil)
	}

	if u.FirstName == "" {
		return NewServiceError("INVALID_NAME", "First name is required", nil)
	}

	if u.LastName == "" {
		return NewServiceError("INVALID_NAME", "Last name is required", nil)
	}

	if u.PasswordHash == "" {
		return NewServiceError("INVALID_PASSWORD", "Password hash is required", nil)
	}

	// Validate profile if present
	if u.Profile != nil {
		if err := u.Profile.Validate(); err != nil {
			return err
		}
	}

	return nil
}

// Validate performs validation on the user profile data.
// This method ensures that profile information meets the required criteria.
func (p *UserProfile) Validate() error {
	// Bio length validation
	if len(p.Bio) > 1000 {
		return NewServiceError("INVALID_BIO", "Bio must be 1000 characters or less", nil)
	}

	// URL validation for avatar and website
	if p.AvatarURL != "" && !isValidURL(p.AvatarURL) {
		return NewServiceError("INVALID_AVATAR_URL", "Avatar URL format is invalid", nil)
	}

	if p.WebsiteURL != "" && !isValidURL(p.WebsiteURL) {
		return NewServiceError("INVALID_WEBSITE_URL", "Website URL format is invalid", nil)
	}

	return nil
}

// UserService provides business logic operations for user management.
// This service coordinates between the repository layer and external systems,
// implementing business rules, validation, and security measures.
type UserService struct {
	// repo is the user repository for data persistence
	repo UserRepository

	// logger provides structured logging capabilities
	logger *zap.Logger

	// db is the database connection for transactions
	db *sql.DB

	// config contains service configuration parameters
	config *ServiceConfig
}

// ServiceConfig holds configuration parameters for the user service.
// This struct centralizes all configurable aspects of the service behavior.
type ServiceConfig struct {
	// MaxLoginAttempts is the maximum failed login attempts before locking
	MaxLoginAttempts int `json:"max_login_attempts" default:"5"`

	// LockDuration is how long to lock accounts after max attempts
	LockDuration time.Duration `json:"lock_duration" default:"30m"`

	// PasswordMinLength is the minimum required password length
	PasswordMinLength int `json:"password_min_length" default:"8"`

	// RequireEmailVerification indicates if email verification is mandatory
	RequireEmailVerification bool `json:"require_email_verification" default:"true"`

	// SessionTimeout is the duration before user sessions expire
	SessionTimeout time.Duration `json:"session_timeout" default:"24h"`
}

// NewUserService creates a new instance of the UserService.
// This constructor initializes the service with all required dependencies.
//
// Parameters:
//   - repo: UserRepository implementation for data persistence
//   - logger: Structured logger for audit and debugging
//   - db: Database connection for transaction management
//   - config: Service configuration parameters
//
// Returns:
//   - *UserService: Configured user service instance
func NewUserService(repo UserRepository, logger *zap.Logger, db *sql.DB, config *ServiceConfig) *UserService {
	if config == nil {
		config = &ServiceConfig{
			MaxLoginAttempts:         5,
			LockDuration:            30 * time.Minute,
			PasswordMinLength:       8,
			RequireEmailVerification: true,
			SessionTimeout:          24 * time.Hour,
		}
	}

	return &UserService{
		repo:   repo,
		logger: logger,
		db:     db,
		config: config,
	}
}

// CreateUser creates a new user with comprehensive validation and security measures.
// This method handles password hashing, duplicate detection, and audit logging.
//
// The creation process includes:
//   - Input validation and sanitization
//   - Duplicate email detection
//   - Password hashing with salt
//   - Initial user state setup
//   - Audit logging
//
// Parameters:
//   - ctx: Request context for cancellation and tracing
//   - req: User creation request with all required information
//
// Returns:
//   - *User: The created user with generated ID and timestamps
//   - error: Service error if creation fails for any reason
func (s *UserService) CreateUser(ctx context.Context, req *CreateUserRequest) (*User, error) {
	// Start timing for performance monitoring
	startTime := time.Now()
	defer func() {
		s.logger.Debug("CreateUser completed",
			zap.Duration("duration", time.Since(startTime)),
			zap.String("email", req.Email),
		)
	}()

	s.logger.Info("Starting user creation process",
		zap.String("email", req.Email),
		zap.String("first_name", req.FirstName),
		zap.String("last_name", req.LastName),
	)

	// Validate the creation request
	if err := req.Validate(); err != nil {
		s.logger.Error("User creation request validation failed",
			zap.Error(err),
			zap.String("email", req.Email),
		)
		return nil, fmt.Errorf("request validation failed: %w", err)
	}

	// Check if user already exists
	existingUser, err := s.repo.GetByEmail(ctx, req.Email)
	if err != nil && !errors.Is(err, ErrUserNotFound) {
		s.logger.Error("Failed to check for existing user",
			zap.Error(err),
			zap.String("email", req.Email),
		)
		return nil, NewServiceError("DATABASE_ERROR", "Failed to verify email uniqueness", err)
	}

	if existingUser != nil {
		s.logger.Warn("Attempted to create user with existing email",
			zap.String("email", req.Email),
			zap.String("existing_user_id", existingUser.ID.String()),
		)
		return nil, NewServiceError("DUPLICATE_EMAIL", "A user with this email already exists", nil)
	}

	// Hash the password securely
	passwordHash, err := hashPassword(req.Password)
	if err != nil {
		s.logger.Error("Failed to hash password",
			zap.Error(err),
			zap.String("email", req.Email),
		)
		return nil, NewServiceError("PASSWORD_HASH_ERROR", "Failed to secure password", err)
	}

	// Create the user entity
	now := time.Now()
	user := &User{
		ID:            uuid.New(),
		Email:         strings.ToLower(strings.TrimSpace(req.Email)),
		FirstName:     strings.TrimSpace(req.FirstName),
		LastName:      strings.TrimSpace(req.LastName),
		PasswordHash:  passwordHash,
		IsActive:      true,
		IsAdmin:       false,
		EmailVerified: !s.config.RequireEmailVerification,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	// Add profile information if provided
	if req.Profile != nil {
		user.Profile = &UserProfile{
			Bio:        req.Profile.Bio,
			AvatarURL:  req.Profile.AvatarURL,
			WebsiteURL: req.Profile.WebsiteURL,
			Location:   req.Profile.Location,
			Phone:      req.Profile.Phone,
		}
	}

	// Final validation of the complete user object
	if err := user.Validate(); err != nil {
		s.logger.Error("User validation failed before creation",
			zap.Error(err),
			zap.String("email", req.Email),
		)
		return nil, err
	}

	// Persist the user to the database
	if err := s.repo.Create(ctx, user); err != nil {
		s.logger.Error("Failed to create user in database",
			zap.Error(err),
			zap.String("email", req.Email),
			zap.String("user_id", user.ID.String()),
		)
		return nil, NewServiceError("DATABASE_ERROR", "Failed to create user", err)
	}

	s.logger.Info("User created successfully",
		zap.String("user_id", user.ID.String()),
		zap.String("email", user.Email),
		zap.Bool("email_verified", user.EmailVerified),
	)

	return user, nil
}

// ListOptions defines parameters for user listing operations.
// This struct provides comprehensive filtering, sorting, and pagination options.
type ListOptions struct {
	// Limit is the maximum number of users to return
	Limit int `json:"limit" validate:"min=1,max=1000"`

	// Offset is the number of users to skip (for pagination)
	Offset int `json:"offset" validate:"min=0"`

	// SortBy specifies the field to sort by
	SortBy string `json:"sort_by" validate:"oneof=name email created_at updated_at last_login_at"`

	// SortOrder specifies the sort direction
	SortOrder string `json:"sort_order" validate:"oneof=asc desc"`

	// IncludeDeleted indicates whether to include soft-deleted users
	IncludeDeleted bool `json:"include_deleted"`

	// ActiveOnly filters to only active users
	ActiveOnly bool `json:"active_only"`

	// AdminOnly filters to only admin users
	AdminOnly bool `json:"admin_only"`

	// SearchQuery filters users by name or email containing this text
	SearchQuery string `json:"search_query"`

	// CreatedAfter filters users created after this timestamp
	CreatedAfter *time.Time `json:"created_after,omitempty"`

	// CreatedBefore filters users created before this timestamp
	CreatedBefore *time.Time `json:"created_before,omitempty"`
}

// CreateUserRequest represents a request to create a new user.
// This struct contains all the information needed to create a user account.
type CreateUserRequest struct {
	// Email is the user's email address (required, must be unique)
	Email string `json:"email" validate:"required,email"`

	// Password is the plain text password (will be hashed)
	Password string `json:"password" validate:"required,min=8"`

	// FirstName is the user's given name
	FirstName string `json:"first_name" validate:"required,min=1,max=50"`

	// LastName is the user's family name
	LastName string `json:"last_name" validate:"required,min=1,max=50"`

	// Profile contains optional profile information
	Profile *CreateProfileRequest `json:"profile,omitempty"`

	// IsAdmin indicates if the user should have admin privileges (optional)
	IsAdmin bool `json:"is_admin,omitempty"`
}

// CreateProfileRequest represents profile information for user creation.
type CreateProfileRequest struct {
	Bio        string `json:"bio,omitempty" validate:"max=1000"`
	AvatarURL  string `json:"avatar_url,omitempty" validate:"omitempty,url"`
	WebsiteURL string `json:"website_url,omitempty" validate:"omitempty,url"`
	Location   string `json:"location,omitempty" validate:"max=200"`
	Phone      string `json:"phone,omitempty" validate:"omitempty,phone"`
}

// Validate performs validation on the create user request.
// This method ensures all required fields are present and valid.
func (r *CreateUserRequest) Validate() error {
	if r.Email == "" {
		return NewServiceError("INVALID_EMAIL", "Email is required", nil)
	}

	if r.Password == "" {
		return NewServiceError("INVALID_PASSWORD", "Password is required", nil)
	}

	if len(r.Password) < 8 {
		return NewServiceError("WEAK_PASSWORD", "Password must be at least 8 characters", nil)
	}

	if r.FirstName == "" {
		return NewServiceError("INVALID_NAME", "First name is required", nil)
	}

	if r.LastName == "" {
		return NewServiceError("INVALID_NAME", "Last name is required", nil)
	}

	return nil
}

// Helper functions

// isValidURL validates URL format
func isValidURL(urlStr string) bool {
	// Simplified URL validation - in production use proper URL parsing
	return strings.HasPrefix(urlStr, "http://") || strings.HasPrefix(urlStr, "https://")
}

// hashPassword securely hashes a password using bcrypt
func hashPassword(password string) (string, error) {
	// This would use bcrypt.GenerateFromPassword in real implementation
	// For this example, we'll simulate the behavior
	return fmt.Sprintf("hashed_%s_%d", password, time.Now().Unix()), nil
}

// Common service errors
var (
	ErrUserNotFound    = NewServiceError("USER_NOT_FOUND", "User not found", nil)
	ErrInvalidEmail    = NewServiceError("INVALID_EMAIL", "Invalid email format", nil)
	ErrDuplicateEmail  = NewServiceError("DUPLICATE_EMAIL", "Email already exists", nil)
	ErrWeakPassword    = NewServiceError("WEAK_PASSWORD", "Password does not meet security requirements", nil)
	ErrAccountLocked   = NewServiceError("ACCOUNT_LOCKED", "Account is temporarily locked", nil)
	ErrAccountInactive = NewServiceError("ACCOUNT_INACTIVE", "Account is inactive", nil)
)

import "strings"