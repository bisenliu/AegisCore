package redis

import (
	"fmt"
	"strconv"
	"time"
)

func redisScore(t time.Time) string {
	return strconv.FormatFloat(redisScoreFloat(t), 'f', 9, 64)
}

func redisScoreFloat(t time.Time) float64 {
	return float64(t.UnixNano()) / float64(time.Second)
}

func seconds(ttl time.Duration) string {
	return strconv.FormatInt(int64(ttl/time.Second), 10)
}

func milliseconds(ttl time.Duration) string {
	return strconv.FormatInt(ttl.Milliseconds(), 10)
}

func parseTokenVersion(value string) (int64, error) {
	return strconv.ParseInt(value, 10, 64)
}

func formatTokenVersion(version int64) string {
	return strconv.FormatInt(version, 10)
}

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
