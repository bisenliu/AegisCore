// Package httpserver 提供业务中立的 net/http server 生命周期管理。
//
// Managed 同步绑定监听地址、异步执行 Serve，并统一处理异常退出、优雅关闭、
// 强制关闭和已进入 handler 的请求 drain。启用策略、配置默认值、日志和进程
// shutdown 策略由调用方负责。
package httpserver
