package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

var benchmarkRequestLogFieldsSink []zap.Field

func BenchmarkRequestLogFields(b *testing.B) {
	c := benchmarkRequestLogContext()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		fieldsRef := requestLogFields(c, 12*time.Millisecond)
		fields := *fieldsRef
		benchmarkRequestLogFieldsSink = fields
		releaseRequestLogFields(fieldsRef)
	}
}

func benchmarkRequestLogContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/users/123", nil)
	c.Writer.WriteHeader(http.StatusAccepted)
	return c
}
