package httpserver_test

import (
	"context"
	"net/http"
	"time"

	"github.com/aegiscore/common/runtime/httpserver"
)

func ExampleManaged() {
	server, err := httpserver.New(httpserver.Options{
		Name:            "example",
		Addr:            "127.0.0.1:0",
		Handler:         http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(http.StatusNoContent) }),
		ReadTimeout:     5 * time.Second,
		WriteTimeout:    5 * time.Second,
		IdleTimeout:     30 * time.Second,
		ShutdownTimeout: 10 * time.Second,
	})
	if err != nil {
		return
	}
	if err := server.Start(context.Background()); err != nil {
		return
	}
	_ = server.Stop(context.Background())
}
