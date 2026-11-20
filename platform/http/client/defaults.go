package client

import "time"

// DefaultTimeout 是请求未显式设置 timeout 时使用的单次发送上限。
const DefaultTimeout = 60 * time.Second
