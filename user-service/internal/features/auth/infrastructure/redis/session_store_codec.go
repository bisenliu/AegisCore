package redis

import (
	"fmt"
	"strconv"
	"time"
)

// redisScore 将时间戳转换为浮点数值，用于在缓存排行榜中按时间先后顺序排列。
func redisScore(t time.Time) string {
	return strconv.FormatFloat(redisScoreFloat(t), 'f', 9, 64)
}

// redisScoreFloat 将指定时间转换为分数值，便于缓存按过期时间检索和清理。
func redisScoreFloat(t time.Time) float64 {
	return float64(t.UnixNano()) / float64(time.Second)
}

// seconds 将时长转换为秒数文本，供设置缓存有效时长使用。
func seconds(ttl time.Duration) string {
	return strconv.FormatInt(int64(ttl/time.Second), 10)
}

// milliseconds 将时长转换为毫秒数文本。
func milliseconds(ttl time.Duration) string {
	return strconv.FormatInt(ttl.Milliseconds(), 10)
}

// parseTokenVersion 从缓存文本中解析出数字形式的凭证版本号；格式损坏时返回错误。
func parseTokenVersion(value string) (int64, error) {
	return strconv.ParseInt(value, 10, 64)
}

// formatTokenVersion 将数字凭证版本号转为标准文本，便于存入缓存中。
func formatTokenVersion(version int64) string {
	return strconv.FormatInt(version, 10)
}

// redisMemberString 将缓存集合中的元素安全提取为文本；如果类型异常则返回空文本。
func redisMemberString(member interface{}) string {
	switch value := member.(type) {
	case string:
		return value
	case []byte:
		return string(value)
	default:
		return fmt.Sprint(value)
	}
}
