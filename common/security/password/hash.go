package password

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
)

// HashContext 使用实例安全参数创建编码后的 Argon2id 密码哈希，并支持等待 KDF 槽位时被 ctx 取消。
func (s *Service) HashContext(ctx context.Context, plain string) (string, error) {
	if err := validatePlainPassword(plain); err != nil {
		return "", err
	}

	salt := make([]byte, s.params.saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}

	key, err := s.deriveKeyContext(ctx, []byte(plain), salt, s.params)
	if err != nil {
		return "", err
	}

	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Key := base64.RawStdEncoding.EncodeToString(key)

	return fmt.Sprintf(
		"$%s$v=%d$m=%d,t=%d,p=%d$%s$%s",
		passwordAlgorithm,
		passwordVersion,
		s.params.memory,
		s.params.iterations,
		s.params.parallelism,
		b64Salt,
		b64Key,
	), nil
}

// VerifyContext 使用常量时间比较校验明文密码和编码后的 Argon2id 哈希，并支持等待 KDF 槽位时被 ctx 取消。
func (s *Service) VerifyContext(ctx context.Context, plain, encodedHash string) (bool, error) {
	if err := validatePlainPassword(plain); err != nil {
		return false, err
	}

	parsed, salt, expected, err := parsePasswordHash(encodedHash)
	if err != nil {
		// 格式错误、超长哈希或不符合当前策略的参数会在运行 Argon2 前被拒绝。
		return false, err
	}

	actual, err := s.deriveKeyContext(ctx, []byte(plain), salt, parsed)
	if err != nil {
		return false, err
	}

	if subtle.ConstantTimeCompare(actual, expected) == 1 {
		return true, nil
	}
	return false, nil
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
