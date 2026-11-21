package config

import "time"

type GrpcServer struct {
	Port              int           `yaml:"port" env:"GRPC_PORT"`
	MaxConnectionIdle time.Duration `yaml:"max_connection_idle"`
	MaxConnectionAge  time.Duration `yaml:"max_connection_age"`
	Timeout           time.Duration `yaml:"timeout"`
	Time              time.Duration `yaml:"time"`
}

type HttpServer struct {
	Port      int `yaml:"port" env:"HTTP_PORT"`
	AdminPort int `yaml:"admin_port" env:"ADMIN_HTTP_PORT"`
}

type Graceful struct {
	Timeout time.Duration `yaml:"timeout" env:"GRACEFUL_TIMEOUT"`
}

type Retry struct {
	Default  RetryPolicy   `yaml:"__default__"`
	Handlers []RetryPolicy `yaml:"handlers"`
}


type RetryPolicy struct {
	Method                         string        `yaml:"method"`
	Enabled                        *bool         `yaml:"enabled"` 
	SeparateRetryPerHandlerEnabled bool          `yaml:"separate_retry_per_handler_enabled"`
	Timeout                        time.Duration `yaml:"timeout"`
	MaxAttempts                    int           `yaml:"max_attempts"`
	InitialBackoff                 time.Duration `yaml:"initial_backoff"`
	MaxBackoff                     time.Duration `yaml:"max_backoff"`
	BackoffMultiplier              float64       `yaml:"backoff_multiplier"`
	RetryableCodes                 []string      `yaml:"retryable_codes"` 
	Throttler                      *Throttler    `yaml:"throttler"`
}

type Throttler struct {
	ThrottlingEnabled  bool    `yaml:"throttling_enabled"`
	ThrottleMaxTokens  float64 `yaml:"throttle_max_tokens"`
	ThrottleTokenRatio float64 `yaml:"throttle_token_ratio"`
}
