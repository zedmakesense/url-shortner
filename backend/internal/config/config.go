package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"strconv"
	"time"

	// autoloading env config.
	_ "github.com/joho/godotenv/autoload"
	"github.com/zedmakesense/url-shortner/internal/domain"
)

type Config struct {
	App    AppConfig
	Log    LogConfig
	DB     DBConfig
	Redis  RedisConfig
	Resend ResendConfig
}

type AppConfig struct {
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

type LogConfig struct {
	Level     slog.Level
	Format    string
	AddSource bool
}

type ResendConfig struct {
	APIKey string // #nosec G117
}

type RedisConfig struct {
	DB           int
	Host         string
	Port         string
	Password     string // #nosec G117
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

type DBConfig struct {
	Host            string
	Port            int
	User            string
	Password        string // #nosec G117
	Name            string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

func Load() (*Config, error) {
	cfg := &Config{}
	if err := cfg.LoadResendConfig(); err != nil {
		return nil, fmt.Errorf("failed to load Resend config: %w", err)
	}
	if err := cfg.LoadAppConfig(); err != nil {
		return nil, fmt.Errorf("failed to load App config: %w", err)
	}
	if err := cfg.LoadRedisConfig(); err != nil {
		return nil, fmt.Errorf("failed to load Redis config: %w", err)
	}
	if err := cfg.LoadDBConfig(); err != nil {
		return nil, fmt.Errorf("failed to load DB config: %w", err)
	}
	cfg.LoadLogConfig()
	return cfg, nil
}

func NewLogConfig() *Config {
	cfg := &Config{}
	cfg.LoadLogConfig()
	return cfg
}

func (c *Config) LoadLogConfig() {
	c.Log.Level = parseLevel(getEnv("LOG_LEVEL", "debug"))
	c.Log.Format = getEnv("LOG_FORMAT", "text")
	c.Log.AddSource = parseBool(getEnv("LOG_ADDSOURCE", "true"))
}

func (c *Config) LoadResendConfig() error {
	c.Resend.APIKey = getEnv("RESEND_API", "")
	if c.Resend.APIKey == "" {
		return domain.ErrResendAPIKeyNotFound
	}
	return nil
}

func (c *Config) LoadAppConfig() error {
	var err error
	c.App.Port = getEnv("APP_PORT", "8080")
	if c.App.ReadTimeout, err = parseDuration(getEnv("APP_READ_TIMEOUT", "10s")); err != nil {
		return err
	}
	if c.App.WriteTimeout, err = parseDuration(getEnv("APP_WRITE_TIMEOUT", "10s")); err != nil {
		return err
	}
	if c.App.IdleTimeout, err = parseDuration(getEnv("APP_IDLE_TIMEOUT", "60s")); err != nil {
		return err
	}
	return nil
}

func (c *Config) LoadRedisConfig() error {
	var err error
	if c.Redis.DB, err = parseInt(getEnv("REDIS_DB", "0")); err != nil {
		return err
	}
	c.Redis.Host = getEnv("REDIS_HOST_GO", "redis")
	if _, err = parseInt(getEnv("REDIS_PORT", "6379")); err != nil {
		return err
	}
	c.Redis.Port = getEnv("REDIS_PORT", "6379")
	c.Redis.Password = getEnv("REDIS_PASSWORD", "")
	if c.Redis.Password == "" {
		return errors.New("no Redis password provided")
	}
	if c.Redis.DialTimeout, err = parseDuration(getEnv("REDIS_DIAL_TIMEOUT", "5")); err != nil {
		return err
	}
	if c.Redis.ReadTimeout, err = parseDuration(getEnv("REDIS_READ_TIMEOUT", "3")); err != nil {
		return err
	}
	if c.Redis.WriteTimeout, err = parseDuration(getEnv("REDIS_WRITE_TIMEOUT", "3")); err != nil {
		return err
	}
	return nil
}

func (c *Config) LoadDBConfig() error {
	var err error
	c.DB.Host = getEnv("DB_HOST_GO", "postgres")
	if c.DB.Port, err = parseInt(getEnv("DB_PORT", "5433")); err != nil {
		return err
	}
	c.DB.User = getEnv("DB_USER", "")
	if c.DB.User == "" {
		return errors.New("user not provided for DB")
	}
	c.DB.Password = getEnv("DB_PASSWORD", "")
	if c.DB.Password == "" {
		return errors.New("password not provided for DB")
	}
	c.DB.Name = getEnv("DB_NAME", "")
	if c.DB.Password == "" {
		return errors.New("db name not provided for DB")
	}
	if c.DB.MaxOpenConns, err = parseInt(getEnv("DB_MAX_OPEN_CONNS", "25")); err != nil {
		return err
	}
	if c.DB.MaxIdleConns, err = parseInt(getEnv("DB_MAX_IDLE_CONNS", "25")); err != nil {
		return err
	}
	if c.DB.ConnMaxLifetime, err = parseDuration(getEnv("DB_CONN_MAX_LIFETIME", "5m")); err != nil {
		return err
	}
	if c.DB.ConnMaxIdleTime, err = parseDuration(getEnv("DB_CONN_MAX_IDLE_TIME", "5m")); err != nil {
		return err
	}
	return nil
}

func (c *Config) DatabaseURL() string {
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(c.DB.User, c.DB.Password),
		Host:     net.JoinHostPort(c.DB.Host, strconv.Itoa(c.DB.Port)),
		Path:     "/" + c.DB.Name,
		RawQuery: "sslmode=disable",
	}
	return u.String()
}

func parseInt(s string) (int, error) {
	result, err := strconv.Atoi(s)
	if err != nil {
		return -1, fmt.Errorf("invalid %s %w", s, err)
	}
	return result, nil
}

func parseDuration(s string) (time.Duration, error) {
	result, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", s, err)
	}
	return result, nil
}

func parseBool(s string) bool {
	return s != "false"
}

func parseLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func getEnv(key, defaultValue string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultValue
}
