package project

import "fmt"

// UserService handles user business logic.
type UserService struct {
	repo Repository
}

// NewUserService creates a new UserService.
func NewUserService(repo Repository) *UserService {
	return &UserService{repo: repo}
}

// GetUser retrieves a user by ID.
func (s *UserService) GetUser(id int) (*User, error) {
	user, err := s.repo.FindByID(id)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	return user, nil
}

// CreateUser creates a new user.
func (s *UserService) CreateUser(name, email string) (*User, error) {
	user := newUser(name, email)
	if err := s.repo.Save(user); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return user, nil
}
