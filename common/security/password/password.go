package password

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"math"
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
	// maxPasswordLength 限制明文密码长度，避免超大输入放大请求内存和解析成本。
	maxPasswordLength = 256

	// defaultArgon2Concurrency 是单进程默认 Argon2id 执行并发上限。
	defaultArgon2Concurrency = 2
	// defaultArgon2QueueSize 是单进程默认 Argon2id KDF 区域请求总数上限，包含执行中和等待中的请求。
	defaultArgon2QueueSize = 16
)

var (
	// ErrEmptyPassword 表示哈希或校验收到空明文密码。
	ErrEmptyPassword = errors.New("password is empty")
	// ErrPasswordTooLong 表示明文密码超过包允许的最大长度。
	ErrPasswordTooLong = errors.New("password is too long")
	// ErrInvalidHash 表示编码后的密码哈希格式错误或不受支持。
	ErrInvalidHash = errors.New("password hash is invalid")
	// ErrPasswordKDFBusy 表示密码 KDF 执行中和等待执行的请求总数已达上限。
	ErrPasswordKDFBusy = errors.New("password kdf is busy")
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

// argon2Gate 限制同一进程内同时执行 Argon2id 的数量。
// 默认 2 个并发约等于 128MiB Argon2 工作内存，不包含 Go runtime、HTTP、DB、Redis 等额外内存。
var argon2Gate = make(chan struct{}, defaultArgon2Concurrency)

// argon2Queue 限制同一进程内执行中和等待执行 Argon2id 的请求总数，避免 handler goroutine 无限堆积。
var argon2Queue = make(chan struct{}, defaultArgon2QueueSize)

// HashContext 使用包默认安全参数创建编码后的 Argon2id 密码哈希，并支持等待 KDF 槽位时被 ctx 取消。
func HashContext(ctx context.Context, plain string) (string, error) {
	if err := validatePlainPassword(plain); err != nil {
		return "", err
	}

	salt := make([]byte, defaultPasswordParams.saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}

	key, err := deriveKeyContext(ctx, []byte(plain), salt, defaultPasswordParams)
	if err != nil {
		return "", err
	}

	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Key := base64.RawStdEncoding.EncodeToString(key)

	return fmt.Sprintf(
		"$%s$v=%d$m=%d,t=%d,p=%d$%s$%s",
		passwordAlgorithm,
		passwordVersion,
		defaultPasswordParams.memory,
		defaultPasswordParams.iterations,
		defaultPasswordParams.parallelism,
		b64Salt,
		b64Key,
	), nil
}

// VerifyContext 使用常量时间比较校验明文密码和编码后的 Argon2id 哈希，并支持等待 KDF 槽位时被 ctx 取消。
func VerifyContext(ctx context.Context, plain, encodedHash string) (bool, error) {
	if err := validatePlainPassword(plain); err != nil {
		return false, err
	}

	parsed, salt, expected, err := parsePasswordHash(encodedHash)
	if err != nil {
		// 格式错误、超长哈希或不符合当前策略的参数会在运行 Argon2 前被拒绝。
		return false, err
	}

	actual, err := deriveKeyContext(ctx, []byte(plain), salt, parsed)
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

func deriveKeyContext(ctx context.Context, plain, salt []byte, params passwordParams) ([]byte, error) {
	if err := enterArgon2Queue(ctx); err != nil {
		return nil, err
	}
	defer leaveArgon2Queue()

	if err := acquireArgon2Slot(ctx); err != nil {
		return nil, err
	}
	defer releaseArgon2Slot()

	return argon2.IDKey(
		plain,
		salt,
		params.iterations,
		params.memory,
		params.parallelism,
		params.keyLength,
	), nil
}

func enterArgon2Queue(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	select {
	case argon2Queue <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	default:
		return ErrPasswordKDFBusy
	}

	if err := ctx.Err(); err != nil {
		leaveArgon2Queue()
		return err
	}

	return nil
}

func leaveArgon2Queue() {
	<-argon2Queue
}

func acquireArgon2Slot(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	select {
	case argon2Gate <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}

	if err := ctx.Err(); err != nil {
		releaseArgon2Slot()
		return err
	}

	return nil
}

func releaseArgon2Slot() {
	<-argon2Gate
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
	if len(salt) > math.MaxUint32 || len(key) > math.MaxUint32 {
		return passwordParams{}, nil, nil, ErrInvalidHash
	}

	parsedParams.saltLength = uint32(len(salt)) // #nosec G115 -- encodedHash 最大 512 字节，长度已受控。
	parsedParams.keyLength = uint32(len(key))   // #nosec G115 -- encodedHash 最大 512 字节，长度已受控。

	if !isPasswordParamsAllowed(parsedParams) {
		return passwordParams{}, nil, nil, ErrInvalidHash
	}

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
	if !okM || !okT || !okP || m > math.MaxUint32 || t > math.MaxUint32 || p > math.MaxUint8 {
		return passwordParams{}, ErrInvalidHash
	}

	return passwordParams{
		memory:      uint32(m),
		iterations:  uint32(t),
		parallelism: uint8(p),
	}, nil
}

func isPasswordParamsAllowed(params passwordParams) bool {
	return params.memory == passwordMemory &&
		params.iterations == passwordIterations &&
		params.parallelism == passwordParallelism &&
		params.saltLength == passwordSaltLength &&
		params.keyLength == passwordKeyLength
}
