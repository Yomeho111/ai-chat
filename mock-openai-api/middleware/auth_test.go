package middleware

import (
	"mock-openai-api/pkg/config"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAuth(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("http:\n  access_token: test-token\n"), 0600); err != nil {
		t.Fatal(err)
	}
	config.InitConfig(path)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(Auth())
	router.POST("/v1/chat/completions", func(ctx *gin.Context) {
		ctx.Status(http.StatusOK)
	})

	tests := []struct {
		name          string
		authorization string
		want          int
	}{
		{name: "valid bearer token", authorization: "Bearer test-token", want: http.StatusOK},
		{name: "wrong token", authorization: "Bearer invalid", want: http.StatusUnauthorized},
		{name: "missing bearer prefix", authorization: "test-token", want: http.StatusUnauthorized},
		{name: "missing authorization", want: http.StatusUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
			req.Header.Set("Authorization", tt.authorization)
			res := httptest.NewRecorder()
			router.ServeHTTP(res, req)
			if res.Code != tt.want {
				t.Fatalf("status = %d, want %d", res.Code, tt.want)
			}
		})
	}
}
