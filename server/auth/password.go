package auth

import "golang.org/x/crypto/bcrypt"

const bcryptCost = 12

// HashPassword returns the bcrypt hash of the password.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckPassword compares a plaintext password with a bcrypt hash.
// Returns nil if they match.
func CheckPassword(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
