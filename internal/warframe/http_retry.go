// HTTP 请求重试辅助，供市场 API、仲裁、CDN、导出下载等远程数据源复用
// 策略：指数退避重试（1s → 2s → 4s），仅对可恢复错误重试：
//   - 网络/传输错误（TLS 握手超时、连接拒绝等）
//   - HTTP 429（限速）与 5xx（服务端临时错误）
//
// 4xx 业务错误不重试（重试无意义）
package warframe

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// retryConfig HTTP 重试参数。
type retryConfig struct {
	attempts int           // 总尝试次数（含首次）
	baseWait time.Duration // 首次重试等待
	maxWait  time.Duration // 最大等待（指数退避上限）
	timeout  time.Duration // 单次请求超时（0 表示不设）
}

// SetRetryConfig 设置全局 HTTP 重试参数（从 config.yaml 的 warframe 块读取）。
// 任一参数 <= 0 时保持当前值不变（对齐配置合并语义）。
func SetRetryConfig(attempts int, baseWaitSeconds, timeoutSeconds int) {
	if attempts > 0 {
		defaultRetry.attempts = attempts
	}
	if baseWaitSeconds > 0 {
		defaultRetry.baseWait = time.Duration(baseWaitSeconds) * time.Second
		defaultRetry.maxWait = defaultRetry.baseWait * 4 // 指数退避上限 = 基准的 4 倍
	}
	if timeoutSeconds > 0 {
		defaultRetry.timeout = time.Duration(timeoutSeconds) * time.Second
	}
}

// defaultRetry 默认重试参数：3 次尝试，1s 起始指数退避，15s 单次超时。
// 与 config.yaml 的 warframe.http_retry_* 默认值保持一致。
var defaultRetry = retryConfig{
	attempts: 3,
	baseWait: time.Second,
	maxWait:  4 * time.Second,
	timeout:  15 * time.Second,
}

// retryable 判断错误是否值得重试（网络错误或超时）。
func retryable(err error) bool {
	if err == nil {
		return false
	}
	// context 超时/取消不重试（调用方主动放弃）
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	// net 包错误（TLS 握手超时、连接拒绝、DNS 等）
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	// 常见网络错误文案兜底
	text := err.Error()
	for _, keyword := range []string{"TLS handshake timeout", "connection refused", "connection reset", "no such host", "i/o timeout", "EOF"} {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

// retryableStatus 判断 HTTP 状态码是否值得重试（429 限速、5xx 服务端错误）。
func retryableStatus(status int) bool {
	return status == http.StatusTooManyRequests || status >= 500
}

// doRequestWithRetry 执行带重试的 HTTP 请求，返回响应体字节。
// 返回的 error 已包含最终错误信息；响应为不可重试状态（4xx）时直接返回。
func doRequestWithRetry(ctx context.Context, client *http.Client, method, url string, headers map[string]string) ([]byte, error) {
	cfg := defaultRetry
	var lastErr error
	for attempt := 1; attempt <= cfg.attempts; attempt++ {
		body, retry, err := singleRequest(ctx, client, method, url, headers, cfg.timeout)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry || attempt == cfg.attempts {
			return nil, err
		}
		// 指数退避：1s → 2s → 4s
		wait := cfg.baseWait << (attempt - 1)
		if wait > cfg.maxWait {
			wait = cfg.maxWait
		}
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return nil, lastErr
}

// singleRequest 单次请求：返回 (响应体, 是否可重试, 错误)。
func singleRequest(ctx context.Context, client *http.Client, method, url string, headers map[string]string, timeout time.Duration) ([]byte, bool, error) {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	request, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, false, err
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}

	response, err := client.Do(request)
	if err != nil {
		return nil, retryable(err), fmt.Errorf("request %s failed: %w", url, err)
	}
	defer response.Body.Close()

	switch {
	case response.StatusCode == http.StatusTooManyRequests:
		return nil, true, fmt.Errorf("HTTP 429 rate limited")
	case response.StatusCode >= 500:
		return nil, true, fmt.Errorf("HTTP %d", response.StatusCode)
	case response.StatusCode < 200 || response.StatusCode >= 400:
		return nil, false, fmt.Errorf("HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, retryable(err), fmt.Errorf("read response: %w", err)
	}
	return body, false, nil
}
