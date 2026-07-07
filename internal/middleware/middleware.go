package middleware

import (
	"log"
	"net/http"
	"runtime"
	"strings"
	"time"
)

// responseWriter 包装 http.ResponseWriter 以便在中间件中捕获 HTTP 响应状态码
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Logging HTTP 请求与业务可观测性日志中间件。
// 记录请求耗时、状态码、客户端 IP，并针对关键业务操作（如结算、退款、库存变动、系统重置等）输出专项审计日志。
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(rw, r)

		duration := time.Since(start)

		// 1. 常规 HTTP 请求日志
		log.Printf("[HTTP] %s %s | 状态码: %d | 耗时: %v | 客户端: %s", r.Method, r.URL.RequestURI(), rw.statusCode, duration, r.RemoteAddr)

		// 2. 异常或错误状态记录
		if rw.statusCode >= http.StatusBadRequest {
			log.Printf("[HTTP警报] 请求发生错误或被拦截: %s %s -> 状态码 %d", r.Method, r.URL.RequestURI(), rw.statusCode)
		}

		// 3. 关键业务操作专项审计日志 (Critical Business Operation Auditing)
		path := r.URL.Path
		if strings.HasPrefix(path, "/api/checkout") ||
			strings.HasPrefix(path, "/api/refund") ||
			strings.HasPrefix(path, "/api/pickup") ||
			strings.HasPrefix(path, "/api/order/delete") ||
			strings.HasPrefix(path, "/api/order/clear_history") ||
			strings.HasPrefix(path, "/api/inventory/delete") ||
			strings.HasPrefix(path, "/api/inventory/batch-delete") ||
			strings.HasPrefix(path, "/api/inventory/procure") ||
			strings.HasPrefix(path, "/api/system/") {
			log.Printf("[业务审计] 关键业务操作记录: 操作=[%s] 请求方式=[%s] 执行结果状态=[%d] 处理耗时=[%v]", path, r.Method, rw.statusCode, duration)
		}
	})
}

// Recovery 全局异常捕获与恢复中间件 (Panic Recovery)。
// 拦截任何未处理的运行时异常 (Panic)，记录完整的调用堆栈信息，并向客户端返回整洁的 500 错误，杜绝进程崩溃打挂整个收银服务。
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				// 获取堆栈追踪信息
				buf := make([]byte, 4096)
				n := runtime.Stack(buf, false)
				stackTrace := string(buf[:n])

				log.Printf("[严重系统异常] 捕获未处理的运行时异常 (Panic) - 路径: %s | 错误信息: %v\n堆栈跟踪:\n%s", r.URL.Path, err, stackTrace)

				// 向客户端返回安全友好的 500 内部错误提示，杜绝进程直接退出
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("服务器内部发生未捕获的系统异常，异常已被捕获并记录至安全日志，请联系系统管理员处理。"))
			}
		}()

		next.ServeHTTP(w, r)
	})
}
