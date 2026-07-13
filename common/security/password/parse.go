package password

import (
	"encoding/base64"
	"math"
	"strconv"
	"strings"
)

func parsePasswordHash(encodedHash string) (passwordParams, []byte, []byte, error) {
	// 这是面向当前策略的 Argon2id hash parser，不是通用 PHC 兼容解析器；历史参数、未知算法或异常长度都会被拒绝。
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
