package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Server ServerConfig `mapstructure:"server"`
	DB     DBConfig     `mapstructure:"db"`
	Redis  RedisConfig  `mapstructure:"redis"`
	NATS   NATSConfig   `mapstructure:"nats"`
	SMTP   SMTPConfig   `mapstructure:"smtp"`
	Worker WorkerConfig `mapstructure:"worker"`
}

type ServerConfig struct {
	Port                int    `mapstructure:"port"`
	JWTSecret           string `mapstructure:"jwt_secret"`
	JWTExpireHours      int    `mapstructure:"jwt_expire_hours"`
	SeedDefaultAccounts bool   `mapstructure:"seed_default_accounts"`
	SeedDefaultChannels bool   `mapstructure:"seed_default_channels"`
}

type DBConfig struct {
	Host           string `mapstructure:"host"`
	Port           int    `mapstructure:"port"`
	User           string `mapstructure:"user"`
	Password       string `mapstructure:"password"`
	DBName         string `mapstructure:"dbname"`
	SSLMode        string `mapstructure:"sslmode"`
	MaxOpenConns   int    `mapstructure:"max_open_conns"`    // 0 = unlimited
	MaxIdleConns   int    `mapstructure:"max_idle_conns"`    // 0 = Go default (2)
	ConnMaxIdleSec int    `mapstructure:"conn_max_idle_sec"` // 0 = no limit
}

type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type NATSConfig struct {
	URL           string `mapstructure:"url"`
	MemoryStorage bool   `mapstructure:"memory_storage"` // true = 内存存储，吞吐更高但重启丢消息
	Replicas      int    `mapstructure:"replicas"`       // JetStream 副本数，单节点填 1（默认）
}

// WorkerConfig 控制此 Worker 进程订阅的 NATS 主题列表。
// 默认订阅 ["task.>"]（全类型）。
// 如需运行专用 Worker（如 GPU 节点只处理视频），配置示例：
//
//	worker:
//	  subjects:
//	    - "task.video.*"
//	  max_concurrent: 10  # 最大同时执行的任务数，0 表示不限制
type WorkerConfig struct {
	Subjects      []string `mapstructure:"subjects"`
	MaxConcurrent int      `mapstructure:"max_concurrent"`
}

type SMTPConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	From     string `mapstructure:"from"`
}

func Load() (*Config, error) {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("/app")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	v.SetDefault("server.port", 8080)
	v.SetDefault("server.jwt_secret", "")
	v.SetDefault("server.jwt_expire_hours", 24)
	v.SetDefault("server.seed_default_accounts", false)
	v.SetDefault("server.seed_default_channels", false)
	v.SetDefault("db.sslmode", "disable")

	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (cfg *Config) Validate() error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	secret := strings.TrimSpace(cfg.Server.JWTSecret)
	if isWeakJWTSecret(secret) || len(secret) < 32 {
		return fmt.Errorf("server.jwt_secret must be a strong random string of at least 32 characters")
	}
	if cfg.Server.JWTExpireHours <= 0 {
		return fmt.Errorf("server.jwt_expire_hours must be greater than 0")
	}
	return nil
}

func isWeakJWTSecret(secret string) bool {
	switch strings.ToLower(strings.TrimSpace(secret)) {
	case "", "change-me", "change-me-in-production", "your-secret", "your-jwt-secret", "replace-me", "替换为强随机字符串":
		return true
	default:
		return false
	}
}
