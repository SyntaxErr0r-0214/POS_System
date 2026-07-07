package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestLogging 验证可观测性日志中间件是否正常转发 HTTP 请求并正确记录响应状态码
func TestLogging(t *testing.T) {
	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("ok"))
	})

	wrapped := Logging(dummyHandler)

	req := httptest.NewRequest(http.MethodPost, "/api/checkout", nil)
	w := httptest.NewRecorder()

	wrapped.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("日志中间件未能正确传递 HTTP 状态码：期望 %d，实际 %d", http.StatusCreated, resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "ok" {
		t.Fatalf("日志中间件未能正确传递响应体内容：期望 ok，实际 %s", string(body))
	}
}

// TestRecovery 验证异常捕获与恢复中间件是否能有效拦截未捕获的 Panic 并在不挂掉进程的前提下返回 500
func TestRecovery(t *testing.T) {
	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("模拟业务代码中发生的致命内存越界或空指针异常")
	})

	wrapped := Recovery(panicHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	w := httptest.NewRecorder()

	// 必须能够安全执行完毕且不触发底层测试框架崩溃
	wrapped.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("Panic Recovery 中间件期望返回 HTTP 500，实际返回: %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "服务器内部发生未捕获的系统异常") {
		t.Fatalf("Panic Recovery 未能输出正确的用户安全提示信息，实际内容: %s", string(body))
	}
}
