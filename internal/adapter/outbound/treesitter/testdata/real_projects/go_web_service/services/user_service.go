package services

import (
	"errors"
	"sync"

	"../models"
	"../utils"
)

type UserService struct {
	users     map[int]*models.User
	nextID    int
	mu        sync.RWMutex
	validator *utils.Validator
}

func NewUserService() *UserService {
	return &UserService{
		users:     make(map[int]*models.User),
		nextID:    1,
		validator: utils.NewValidator(),
	}
}

func (s *UserService) GetAllUsers() []*models.User {
	s.mu.RLock()
	defer s.mu.RUnlock()

	users := make([]*models.User, 0, len(s.users))
	for _, user := range s.users {
		users = append(users, user)
	}
	return users
}

func (s *UserService) GetUserByID(id int) (*models.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, exists := s.users[id]
	if !exists {
		return nil, errors.New("user not found")
	}
	return user, nil
}

func (s *UserService) CreateUser(name, email string) (*models.User, error) {
	user := models.NewUser(name, email)
	
	if err := user.Validate(); err != nil {
		return nil, err
	}

	if err := s.validator.ValidateUser(user); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check for duplicate email
	for _, existingUser := range s.users {
		if existingUser.Email == email {
			return nil, errors.New("user with this email already exists")
		}
	}

	user.ID = s.nextID
	s.nextID++
	s.users[user.ID] = user

	return user, nil
}

func (s *UserService) UpdateUser(id int, name, email string) (*models.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, exists := s.users[id]
	if !exists {
		return nil, errors.New("user not found")
	}

	// Check for duplicate email (if changing email)
	if email != "" && email != user.Email {
		for _, existingUser := range s.users {
			if existingUser.Email == email && existingUser.ID != id {
				return nil, errors.New("user with this email already exists")
			}
		}
	}

	user.Update(name, email)

	if err := user.Validate(); err != nil {
		return nil, err
	}

	if err := s.validator.ValidateUser(user); err != nil {
		return nil, err
	}

	return user, nil
}

func (s *UserService) DeleteUser(id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, exists := s.users[id]
	if !exists {
		return errors.New("user not found")
	}

	delete(s.users, id)
	return nil
}

func (s *UserService) GetUserCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.users)
}

func (s *UserService) FindUsersByEmail(email string) []*models.User {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var matchingUsers []*models.User
	for _, user := range s.users {
		if user.Email == email {
			matchingUsers = append(matchingUsers, user)
		}
	}
	return matchingUsers
}