package password

import (
	"context"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// HashContext 使用固定 bcrypt 策略创建编码后的密码哈希。
func (s *Service) HashContext(ctx context.Context, plain string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := validatePlainPassword(plain); err != nil {
		return "", err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(plain), s.cost)
	if err != nil {
		return "", fmt.Errorf("hash password with bcrypt: %w", err)
	}
	return string(hash), nil
}

// VerifyContext 校验明文密码和编码后的 bcrypt 哈希。
func (s *Service) VerifyContext(ctx context.Context, plain, encodedHash string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if err := validatePlainPassword(plain); err != nil {
		return false, err
	}
	if len(encodedHash) == 0 || len(encodedHash) > maxEncodedHashLength {
		return false, ErrInvalidHash
	}

	if err := bcrypt.CompareHashAndPassword([]byte(encodedHash), []byte(plain)); err == nil {
		return true, nil
	} else if err == bcrypt.ErrMismatchedHashAndPassword {
		return false, nil
	} else {
		return false, ErrInvalidHash
	}
}

func validatePlainPassword(plain string) error {
	if plain == "" {
		return ErrEmptyPassword
	}
	if len(plain) > maxPasswordLength {
		return ErrPasswordTooLong
	}
	return nil
}
