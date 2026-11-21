package retry

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
)

const grpcMaxAttempts = 5

type serviceConfig struct {
	MethodConfig    []methodConfig   `json:"methodConfig,omitempty"`
	RetryThrottling *retryThrottling `json:"retryThrottling,omitempty"`
}

type methodConfig struct {
	Name        []methodName `json:"name"`
	Timeout     string       `json:"timeout,omitempty"`
	RetryPolicy retryPolicy  `json:"retryPolicy"`
}

type methodName struct {
	Service string `json:"service"`
	Method  string `json:"method,omitempty"`
}

type retryPolicy struct {
	MaxAttempts          int          `json:"maxAttempts"`
	InitialBackoff       string       `json:"initialBackoff"`
	MaxBackoff           string       `json:"maxBackoff"`
	BackoffMultiplier    float64      `json:"backoffMultiplier"`
	RetryableStatusCodes []codes.Code `json:"retryableStatusCodes"`
}

type retryThrottling struct {
	MaxTokens  float64 `json:"maxTokens"`
	TokenRatio float64 `json:"tokenRatio"`
}

// ServiceConfig переводит секцию retry в JSON service config grpc-go
func ServiceConfig(cfg Config, service string) (string, error) {
	cfg = withDefaults(cfg)
	if *cfg.Default.SeparatePerHandlerEnabled {
		return "", errors.New("separate_retry_per_handler_enabled: в grpc-go один throttler на соединение")
	}

	var sc serviceConfig
	if *cfg.Default.Enabled {
		mc, ok, err := toMethodConfig(cfg.Default.MainConfig, methodName{Service: service})
		if err != nil {
			return "", fmt.Errorf("__default__: %w", err)
		}
		if ok {
			sc.MethodConfig = append(sc.MethodConfig, mc)
		}
	}
	for _, h := range cfg.Handlers {
		svc, method, ok := strings.Cut(strings.TrimPrefix(h.Method, "/"), "/")
		if !ok || svc == "" || method == "" {
			return "", fmt.Errorf("method %q: ожидается /package.Service/Method", h.Method)
		}
		if svc != service || !*h.Enabled {
			continue
		}
		mc, ok, err := toMethodConfig(h.MainConfig, methodName{Service: svc, Method: method})
		if err != nil {
			return "", fmt.Errorf("%s: %w", h.Method, err)
		}
		if ok {
			sc.MethodConfig = append(sc.MethodConfig, mc)
		}
	}

	// Общий throttler соединения берётся из __default__.
	if t := cfg.Default.Throttler; *t.Enabled && len(sc.MethodConfig) > 0 {
		if t.MaxTokens <= 0 || t.MaxTokens > 1000 || t.TokenRatio <= 0 {
			return "", errors.New("throttler: throttle_max_tokens в (0, 1000], throttle_token_ratio > 0")
		}
		sc.RetryThrottling = &retryThrottling{MaxTokens: t.MaxTokens, TokenRatio: t.TokenRatio}
	}

	b, err := json.Marshal(sc)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// ok=false — повторы фактически выключены: max_attempts ≤ 1 или retryable_codes: [].
func toMethodConfig(c MainConfig, name methodName) (methodConfig, bool, error) {
	if c.MaxAttempts <= 1 || len(c.RetryableStatusCodes) == 0 {
		return methodConfig{}, false, nil
	}
	if c.MaxAttempts > grpcMaxAttempts {
		return methodConfig{}, false, fmt.Errorf("max_attempts %d > %d: нужен grpc.WithMaxCallAttempts", c.MaxAttempts, grpcMaxAttempts)
	}
	if c.InitialBackoff <= 0 || c.MaxBackoff <= 0 || c.BackoffMultiplier <= 0 {
		return methodConfig{}, false, errors.New("initial_backoff, max_backoff и backoff_multiplier должны быть > 0")
	}
	codeList := make([]codes.Code, 0, len(c.RetryableStatusCodes))
	for _, s := range c.RetryableStatusCodes {
		code, ok := retryableCodeMap[s]
		if !ok {
			return methodConfig{}, false, fmt.Errorf("неизвестный код %q", s)
		}
		codeList = append(codeList, code)
	}
	mc := methodConfig{
		Name: []methodName{name},
		RetryPolicy: retryPolicy{
			MaxAttempts:          c.MaxAttempts,
			InitialBackoff:       seconds(c.InitialBackoff),
			MaxBackoff:           seconds(c.MaxBackoff),
			BackoffMultiplier:    c.BackoffMultiplier,
			RetryableStatusCodes: codeList,
		},
	}
	if c.Timeout > 0 {
		mc.Timeout = seconds(c.Timeout)
	}
	return mc, true, nil
}

func seconds(d time.Duration) string {
	return strconv.FormatFloat(d.Seconds(), 'f', -1, 64) + "s"
}
