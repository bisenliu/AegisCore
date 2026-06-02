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
	algorithm = "argon2id"
	version   = argon2.Version

	memory      uint32 = 64 * 1024
	iterations  uint32 = 3
	parallelism uint8  = 4
	saltLength         = 16
	keyLength   uint32 = 32
)

var (
	ErrEmptyPassword = errors.New("password is empty")
	ErrInvalidHash   = errors.New("password hash is invalid")
)

type params struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
	saltLength  uint32
	keyLength   uint32
}

var defaultParams = params{
	memory:      memory,
	iterations:  iterations,
	parallelism: parallelism,
	saltLength:  saltLength,
	keyLength:   keyLength,
}

func Hash(plain string) (string, error) {
	if plain == "" {
		return "", ErrEmptyPassword
	}

	salt := make([]byte, defaultParams.saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}

	key := argon2.IDKey([]byte(plain), salt, defaultParams.iterations, defaultParams.memory, defaultParams.parallelism, defaultParams.keyLength)
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Key := base64.RawStdEncoding.EncodeToString(key)
	return fmt.Sprintf("$%s$v=%d$m=%d,t=%d,p=%d$%s$%s", algorithm, version, defaultParams.memory, defaultParams.iterations, defaultParams.parallelism, b64Salt, b64Key), nil
}

func Verify(plain, encodedHash string) (bool, error) {
	if plain == "" {
		return false, ErrEmptyPassword
	}
	parsed, salt, expected, err := parse(encodedHash)
	if err != nil {
		return false, err
	}

	actual := argon2.IDKey([]byte(plain), salt, parsed.iterations, parsed.memory, parsed.parallelism, parsed.keyLength)
	if subtle.ConstantTimeCompare(actual, expected) == 1 {
		return true, nil
	}
	return false, nil
}

func parse(encodedHash string) (params, []byte, []byte, error) {
	if len(encodedHash) > 512 {
		return params{}, nil, nil, ErrInvalidHash
	}

	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != algorithm {
		return params{}, nil, nil, ErrInvalidHash
	}

	parsedVersion, err := parseVersion(parts[2])
	if err != nil || parsedVersion != version {
		return params{}, nil, nil, ErrInvalidHash
	}

	parsedParams, err := parseParams(parts[3])
	if err != nil {
		return params{}, nil, nil, ErrInvalidHash
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) == 0 {
		return params{}, nil, nil, ErrInvalidHash
	}
	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(key) == 0 {
		return params{}, nil, nil, ErrInvalidHash
	}
	parsedParams.saltLength = uint32(len(salt))
	parsedParams.keyLength = uint32(len(key))
	return parsedParams, salt, key, nil
}

func parseVersion(value string) (int, error) {
	if !strings.HasPrefix(value, "v=") {
		return 0, ErrInvalidHash
	}
	return strconv.Atoi(strings.TrimPrefix(value, "v="))
}

func parseParams(value string) (params, error) {
	fields := strings.Split(value, ",")
	if len(fields) != 3 {
		return params{}, ErrInvalidHash
	}

	values := make(map[string]uint64, 3)
	for _, field := range fields {
		key, valStr, found := strings.Cut(field, "=")
		if !found || key == "" || valStr == "" {
			return params{}, ErrInvalidHash
		}

		parsed, err := strconv.ParseUint(valStr, 10, 32)
		if err != nil || parsed == 0 {
			return params{}, ErrInvalidHash
		}
		values[key] = parsed
	}

	m, okM := values["m"]
	t, okT := values["t"]
	p, okP := values["p"]
	if !okM || !okT || !okP || p > 255 {
		return params{}, ErrInvalidHash
	}
	return params{memory: uint32(m), iterations: uint32(t), parallelism: uint8(p)}, nil
}
