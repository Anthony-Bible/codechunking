package microservice

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

var (
	ErrUserNotFound     = errors.New("user not found")
	ErrInvalidUserData  = errors.New("invalid user data")
	ErrDuplicateEmail   = errors.New("email already exists")
)

// UserRepository defines the interface for user data access
type UserRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	Create(ctx context.Context, user *User) error
	Update(ctx context.Context, user *User) error
	Delete(ctx context.Context, id uuid.UUID) error
	ListUsers(ctx context.Context, limit, offset int) ([]*User, error)
}

// User represents a user entity in the system
type User struct {
	ID        uuid.UUID  `json:"id" db:"id"`
	Email     string     `json:"email" db:"email"`
	FirstName string     `json:"first_name" db:"first_name"`
	LastName  string     `json:"last_name" db:"last_name"`
	IsActive  bool       `json:"is_active" db:"is_active"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt time.Time  `json:"updated_at" db:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty" db:"deleted_at"`
}

// FullName returns the user's full name
func (u *User) FullName() string {
	return fmt.Sprintf("%s %s", u.FirstName, u.LastName)
}

// IsDeleted returns true if the user has been soft deleted
func (u *User) IsDeleted() bool {
	return u.DeletedAt != nil
}

// Validate validates the user data
func (u *User) Validate() error {
	if u.Email == "" {
		return fmt.Errorf("%w: email is required", ErrInvalidUserData)
	}
	
	if u.FirstName == "" || u.LastName == "" {
		return fmt.Errorf("%w: first name and last name are required", ErrInvalidUserData)
	}
	
	return nil
}

// UserService handles user business logic
type UserService struct {
	repo   UserRepository
	logger *zap.Logger
	db     *sql.DB
}

// NewUserService creates a new user service instance
func NewUserService(repo UserRepository, logger *zap.Logger, db *sql.DB) *UserService {
	return &UserService{
		repo:   repo,
		logger: logger,
		db:     db,
	}
}

// CreateUser creates a new user in the system
func (s *UserService) CreateUser(ctx context.Context, req *CreateUserRequest) (*User, error) {
	s.logger.Info("creating new user", zap.String("email", req.Email))

	// Validate request
	if err := req.Validate(); err != nil {
		s.logger.Error("invalid create user request", zap.Error(err))
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Check if user already exists
	existingUser, err := s.repo.GetByEmail(ctx, req.Email)
	if err != nil && !errors.Is(err, ErrUserNotFound) {
		s.logger.Error("failed to check existing user", zap.Error(err))
		return nil, fmt.Errorf("failed to check existing user: %w", err)
	}
	
	if existingUser != nil {
		return nil, ErrDuplicateEmail
	}

	// Create new user
	user := &User{
		ID:        uuid.New(),
		Email:     req.Email,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.repo.Create(ctx, user); err != nil {
		s.logger.Error("failed to create user", zap.Error(err))
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	s.logger.Info("user created successfully", zap.String("user_id", user.ID.String()))
	return user, nil
}

// GetUserByID retrieves a user by their ID
func (s *UserService) GetUserByID(ctx context.Context, id uuid.UUID) (*User, error) {
	s.logger.Debug("getting user by ID", zap.String("user_id", id.String()))

	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		s.logger.Error("failed to get user", zap.Error(err))
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	if user.IsDeleted() {
		return nil, ErrUserNotFound
	}

	return user, nil
}

// UpdateUser updates an existing user
func (s *UserService) UpdateUser(ctx context.Context, id uuid.UUID, req *UpdateUserRequest) (*User, error) {
	s.logger.Info("updating user", zap.String("user_id", id.String()))

	// Get existing user
	user, err := s.GetUserByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Update fields
	if req.FirstName != "" {
		user.FirstName = req.FirstName
	}
	if req.LastName != "" {
		user.LastName = req.LastName
	}
	if req.IsActive != nil {
		user.IsActive = *req.IsActive
	}
	user.UpdatedAt = time.Now()

	// Validate updated user
	if err := user.Validate(); err != nil {
		return nil, err
	}

	// Save updated user
	if err := s.repo.Update(ctx, user); err != nil {
		s.logger.Error("failed to update user", zap.Error(err))
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	s.logger.Info("user updated successfully", zap.String("user_id", user.ID.String()))
	return user, nil
}

// DeleteUser soft deletes a user
func (s *UserService) DeleteUser(ctx context.Context, id uuid.UUID) error {
	s.logger.Info("deleting user", zap.String("user_id", id.String()))

	user, err := s.GetUserByID(ctx, id)
	if err != nil {
		return err
	}

	now := time.Now()
	user.DeletedAt = &now
	user.IsActive = false
	user.UpdatedAt = now

	if err := s.repo.Update(ctx, user); err != nil {
		s.logger.Error("failed to delete user", zap.Error(err))
		return fmt.Errorf("failed to delete user: %w", err)
	}

	s.logger.Info("user deleted successfully", zap.String("user_id", user.ID.String()))
	return nil
}

// ListUsers retrieves a paginated list of users
func (s *UserService) ListUsers(ctx context.Context, req *ListUsersRequest) (*ListUsersResponse, error) {
	s.logger.Debug("listing users", zap.Int("limit", req.Limit), zap.Int("offset", req.Offset))

	users, err := s.repo.ListUsers(ctx, req.Limit, req.Offset)
	if err != nil {
		s.logger.Error("failed to list users", zap.Error(err))
		return nil, fmt.Errorf("failed to list users: %w", err)
	}

	// Filter out deleted users
	activeUsers := make([]*User, 0, len(users))
	for _, user := range users {
		if !user.IsDeleted() {
			activeUsers = append(activeUsers, user)
		}
	}

	return &ListUsersResponse{
		Users: activeUsers,
		Total: len(activeUsers),
	}, nil
}

// Request and Response types

// CreateUserRequest represents a request to create a new user
type CreateUserRequest struct {
	Email     string `json:"email" validate:"required,email"`
	FirstName string `json:"first_name" validate:"required,min=1,max=50"`
	LastName  string `json:"last_name" validate:"required,min=1,max=50"`
}

// Validate validates the create user request
func (r *CreateUserRequest) Validate() error {
	if r.Email == "" {
		return fmt.Errorf("%w: email is required", ErrInvalidUserData)
	}
	if r.FirstName == "" {
		return fmt.Errorf("%w: first name is required", ErrInvalidUserData)
	}
	if r.LastName == "" {
		return fmt.Errorf("%w: last name is required", ErrInvalidUserData)
	}
	return nil
}

// UpdateUserRequest represents a request to update a user
type UpdateUserRequest struct {
	FirstName *string `json:"first_name,omitempty" validate:"omitempty,min=1,max=50"`
	LastName  *string `json:"last_name,omitempty" validate:"omitempty,min=1,max=50"`
	IsActive  *bool   `json:"is_active,omitempty"`
}

// ListUsersRequest represents a request to list users
type ListUsersRequest struct {
	Limit  int `json:"limit" validate:"min=1,max=100"`
	Offset int `json:"offset" validate:"min=0"`
}

// ListUsersResponse represents a response containing a list of users
type ListUsersResponse struct {
	Users []*User `json:"users"`
	Total int     `json:"total"`
}