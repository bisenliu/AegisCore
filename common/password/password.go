package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	// passwordAlgorithm 和 passwordVersion 会写入编码后的哈希，用于兼容性校验。
	passwordAlgorithm = "argon2id"
	passwordVersion   = argon2.Version

	// passwordMemory 是 Argon2id 内存成本，单位为 KiB。
	passwordMemory uint32 = 64 * 1024
	// passwordIterations 是 Argon2id 时间成本。
	passwordIterations uint32 = 3
	// passwordParallelism 是 Argon2id 并行度成本。
	passwordParallelism uint8 = 4
	// passwordSaltLength 是随机盐长度，单位为字节。
	passwordSaltLength = 16
	// passwordKeyLength 是派生密钥长度，单位为字节。
	passwordKeyLength uint32 = 32
	// maxEncodedHashLength 限制不可信编码哈希输入的解析成本。
	maxEncodedHashLength = 512
)

var (
	ErrEmptyPassword = errors.New("password is empty")
	ErrInvalidHash   = errors.New("password hash is invalid")
)

type passwordParams struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
	saltLength  uint32
	keyLength   uint32
}

var defaultPasswordParams = passwordParams{
	memory:      passwordMemory,
	iterations:  passwordIterations,
	parallelism: passwordParallelism,
	saltLength:  passwordSaltLength,
	keyLength:   passwordKeyLength,
}

func Hash(plain string) (string, error) {
	if plain == "" {
		return "", ErrEmptyPassword
	}

	salt := make([]byte, defaultPasswordParams.saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}

	key := argon2.IDKey([]byte(plain), salt, defaultPasswordParams.iterations, defaultPasswordParams.memory, defaultPasswordParams.parallelism, defaultPasswordParams.keyLength)
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Key := base64.RawStdEncoding.EncodeToString(key)
	return fmt.Sprintf("$%s$v=%d$m=%d,t=%d,p=%d$%s$%s", passwordAlgorithm, passwordVersion, defaultPasswordParams.memory, defaultPasswordParams.iterations, defaultPasswordParams.parallelism, b64Salt, b64Key), nil
}

func Verify(plain, encodedHash string) (bool, error) {
	if plain == "" {
		return false, ErrEmptyPassword
	}
	parsed, salt, expected, err := parsePasswordHash(encodedHash)
	if err != nil {
		return false, err
	}

	actual := argon2.IDKey([]byte(plain), salt, parsed.iterations, parsed.memory, parsed.parallelism, parsed.keyLength)
	if subtle.ConstantTimeCompare(actual, expected) == 1 {
		return true, nil
	}
	return false, nil
}

func parsePasswordHash(encodedHash string) (passwordParams, []byte, []byte, error) {
	if len(encodedHash) > maxEncodedHashLength {
		return passwordParams{}, nil, nil, ErrInvalidHash
	}

	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != passwordAlgorithm {
		return passwordParams{}, nil, nil, ErrInvalidHash
	}

	parsedVersion, err := parsePasswordVersion(parts[2])
	if err != nil || parsedVersion != passwordVersion {
		return passwordParams{}, nil, nil, ErrInvalidHash
	}

	parsedParams, err := parsePasswordParams(parts[3])
	if err != nil {
		return passwordParams{}, nil, nil, ErrInvalidHash
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) == 0 {
		return passwordParams{}, nil, nil, ErrInvalidHash
	}
	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(key) == 0 {
		return passwordParams{}, nil, nil, ErrInvalidHash
	}
	parsedParams.saltLength = uint32(len(salt))
	parsedParams.keyLength = uint32(len(key))
	return parsedParams, salt, key, nil
}

func parsePasswordVersion(value string) (int, error) {
	if !strings.HasPrefix(value, "v=") {
		return 0, ErrInvalidHash
	}
	return strconv.Atoi(strings.TrimPrefix(value, "v="))
}

func parsePasswordParams(value string) (passwordParams, error) {
	fields := strings.Split(value, ",")
	if len(fields) != 3 {
		return passwordParams{}, ErrInvalidHash
	}

	values := make(map[string]uint64, 3)
	for _, field := range fields {
		key, valStr, found := strings.Cut(field, "=")
		if !found || key == "" || valStr == "" {
			return passwordParams{}, ErrInvalidHash
		}

		parsed, err := strconv.ParseUint(valStr, 10, 32)
		if err != nil || parsed == 0 {
			return passwordParams{}, ErrInvalidHash
		}
		values[key] = parsed
	}

	m, okM := values["m"]
	t, okT := values["t"]
	p, okP := values["p"]
	if !okM || !okT || !okP || p > 255 {
		return passwordParams{}, ErrInvalidHash
	}
	return passwordParams{memory: uint32(m), iterations: uint32(t), parallelism: uint8(p)}, nil
}
