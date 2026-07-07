package handler

import (
	"crypto/subtle"
	"net/http"
	"os"
	"strings"
)

// GetAdminToken 获取系统管理员校验凭证。
// 优先从环境变量 ADMIN_TOKEN 或 POS_ADMIN_TOKEN 读取，默认值为 "admin123"。
func GetAdminToken() string {
	token := os.Getenv("ADMIN_TOKEN")
	if token == "" {
		token = os.Getenv("POS_ADMIN_TOKEN")
	}
	if token == "" {
		token = "admin123"
	}
	return token
}

// RequireAdmin 针对高危系统管理与调试接口的管理员权限校验中间件。
// 支持通过 HTTP Header (X-Admin-Token / Authorization Bearer / Basic Auth) 或 URL Query 参数验证权限。
func RequireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		expectedToken := GetAdminToken()

		// 1. 优先从 HTTP Header 解析凭证
		token := r.Header.Get("X-Admin-Token")
		if token == "" {
			authHeader := r.Header.Get("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				token = strings.TrimPrefix(authHeader, "Bearer ")
			} else if strings.HasPrefix(authHeader, "Basic ") {
				user, pass, ok := r.BasicAuth()
				if ok && (pass == expectedToken || user == expectedToken) {
					next(w, r)
					return
				}
			}
		}

		// 2. 尝试从 URL Query 参数解析凭证
		if token == "" {
			token = r.URL.Query().Get("admin_token")
			if token == "" {
				token = r.URL.Query().Get("token")
			}
		}

		// 3. 尝试从 POST 表单参数解析凭证
		if token == "" && r.Method == http.MethodPost {
			_ = r.ParseMultipartForm(32 << 20)
			token = r.FormValue("admin_token")
			if token == "" {
				token = r.FormValue("token")
			}
		}

		// 使用常量时间比较函数，防止时序攻击 (Timing Attack)
		if subtle.ConstantTimeCompare([]byte(token), []byte(expectedToken)) != 1 {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("401 Unauthorized: 高危系统操作需要管理员权限验证，请提供有效的管理凭证。"))
			return
		}

		next(w, r)
	}
}
