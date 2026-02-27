package project

// User represents a system user.
type User struct {
	ID    int
	Name  string
	Email string
}

// Repository is the interface for user storage.
type Repository interface {
	FindByID(id int) (*User, error)
	Save(user *User) error
}

func newUser(name, email string) *User {
	return &User{Name: name, Email: email}
}
