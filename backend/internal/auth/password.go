package auth

import "golang.org/x/crypto/bcrypt"

// Cost 12 as required by section 16.2.
const bcryptCost = 12

func HashPassword(p string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(p), bcryptCost)
	return string(b), err
}

func CheckPassword(hash, p string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(p)) == nil
}
