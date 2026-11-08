package utils

import (
	"crypto/rand"
	"errors"
	"math/big"

	"golang.org/x/crypto/bcrypt"
)

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func VerifyPassword(password, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func GenerateStrongPassword(length int) (string, error) {
	if length < 8 {
		return "", errors.New("password length must be at least 8")
	}

	upper := "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	lower := "abcdefghijklmnopqrstuvwxyz"
	digits := "0123456789"
	all := upper + lower + digits

	pick := func(pool string) (byte, error) {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(pool))))
		if err != nil {
			return 0, err
		}
		return pool[n.Int64()], nil
	}

	password := make([]byte, 0, length)
	if ch, err := pick(upper); err == nil {
		password = append(password, ch)
	} else {
		return "", err
	}
	if ch, err := pick(lower); err == nil {
		password = append(password, ch)
	} else {
		return "", err
	}
	if ch, err := pick(digits); err == nil {
		password = append(password, ch)
	} else {
		return "", err
	}

	for len(password) < length {
		ch, err := pick(all)
		if err != nil {
			return "", err
		}
		password = append(password, ch)
	}

	for i := len(password) - 1; i > 0; i-- {
		j, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return "", err
		}
		password[i], password[j.Int64()] = password[j.Int64()], password[i]
	}

	return string(password), nil
}
