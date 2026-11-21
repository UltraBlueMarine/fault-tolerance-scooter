package retry

import (
	"slices"
	"time"

	"github.com/samber/lo"
	"google.golang.org/grpc/codes"
)

// Config — секция retry из config.yaml (формат hotel).
type Config struct {
	Default  DefaultConfig   `yaml:"__default__"`
	Handlers []HandlerConfig `yaml:"handlers"`
}

type DefaultConfig struct {
	MainConfig                `yaml:",inline"`
	SeparatePerHandlerEnabled *bool `yaml:"separate_retry_per_handler_enabled"`
}

type HandlerConfig struct {
	// Полное имя исходящего метода: /package.Service/Method
	Method     string `yaml:"method"`
	MainConfig `yaml:",inline"`
}

type MainConfig struct {
	Enabled *bool `yaml:"enabled"`

	Timeout time.Duration `yaml:"timeout"`

	MaxAttempts int `yaml:"max_attempts"`


	InitialBackoff    time.Duration `yaml:"initial_backoff"`
	MaxBackoff        time.Duration `yaml:"max_backoff"`
	BackoffMultiplier float64       `yaml:"backoff_multiplier"`

	Throttler ThrottlerConfig `yaml:"throttler"`

	RetryableStatusCodes []string `yaml:"retryable_codes"`
}

type ThrottlerConfig struct {
	Enabled *bool `yaml:"throttling_enabled"`
	MaxTokens float64 `yaml:"throttle_max_tokens"`
	TokenRatio float64 `yaml:"throttle_token_ratio"`
}

var retryableCodeMap = func() map[string]codes.Code {
	m := make(map[string]codes.Code)
	for c := codes.OK; c <= codes.Unauthenticated; c++ {
		m[c.String()] = c
	}
	return m
}()

func withDefaults(cfg Config) Config {
	cfg.Handlers = slices.Clone(cfg.Handlers)
	cfg.Default.SeparatePerHandlerEnabled = lo.ToPtr(*overrideIfZero(cfg.Default.SeparatePerHandlerEnabled, lo.ToPtr(false)))
	cfg.Default.MainConfig = defaultConfig(cfg.Default.MainConfig)

	for idx, h := range cfg.Handlers {
		cfg.Handlers[idx].MainConfig = enrich(h.MainConfig, cfg.Default.MainConfig)
	}

	return cfg
}

func enrich(cfg, enrichment MainConfig) MainConfig {
	cfg.Enabled = lo.ToPtr(*overrideIfZero(cfg.Enabled, enrichment.Enabled))
	cfg.Timeout = overrideIfZero(cfg.Timeout, enrichment.Timeout)
	cfg.MaxAttempts = overrideIfZero(cfg.MaxAttempts, enrichment.MaxAttempts)
	cfg.InitialBackoff = overrideIfZero(cfg.InitialBackoff, enrichment.InitialBackoff)
	cfg.MaxBackoff = overrideIfZero(cfg.MaxBackoff, enrichment.MaxBackoff)
	cfg.BackoffMultiplier = overrideIfZero(cfg.BackoffMultiplier, enrichment.BackoffMultiplier)

	cfg.Throttler.Enabled = lo.ToPtr(*overrideIfZero(cfg.Throttler.Enabled, enrichment.Throttler.Enabled))
	cfg.Throttler.MaxTokens = overrideIfZero(cfg.Throttler.MaxTokens, enrichment.Throttler.MaxTokens)
	cfg.Throttler.TokenRatio = overrideIfZero(cfg.Throttler.TokenRatio, enrichment.Throttler.TokenRatio)

	cfg.RetryableStatusCodes = slices.Clone(overrideIfNilSlice(cfg.RetryableStatusCodes, enrichment.RetryableStatusCodes))
	return cfg
}

func defaultConfig(cfg MainConfig) MainConfig {
	defaultCfg := MainConfig{
		Enabled:           lo.ToPtr(false),
		MaxAttempts:       3,
		InitialBackoff:    50 * time.Millisecond,
		MaxBackoff:        1 * time.Second,
		BackoffMultiplier: 1.5,

		Throttler: ThrottlerConfig{
			Enabled:    lo.ToPtr(false),
			MaxTokens:  10,
			TokenRatio: 0.1,
		},
		RetryableStatusCodes: []string{"Unavailable"},
	}

	return enrich(cfg, defaultCfg)
}

func overrideIfZero[T comparable](value, fallback T) T {
	var zero T
	if value == zero {
		return fallback
	}
	return value
}

// nil — не задано, наследуем; [] — явный запрет повторов.
func overrideIfNilSlice[T any](value, fallback []T) []T {
	if value == nil {
		return fallback
	}
	return value
}