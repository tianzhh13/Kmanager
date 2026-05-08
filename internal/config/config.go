package config

import (
	"fmt"

	"github.com/spf13/viper"
)

// Config 应用配置
type Config struct {
	Server          ServerConfig          `mapstructure:"server"`
	Database        DatabaseConfig        `mapstructure:"database"`
	JWT             JWTConfig             `mapstructure:"jwt"`
	Encryption      EncryptionConfig      `mapstructure:"encryption"`
	Log             LogConfig             `mapstructure:"log"`
	VictoriaMetrics VictoriaMetricsConfig `mapstructure:"victoriametrics"`
	SyncWorker      SyncWorkerConfig      `mapstructure:"syncworker"`
}

// SyncWorkerConfig 数据同步 Worker 配置
type SyncWorkerConfig struct {
	Interval int `mapstructure:"interval"` // 同步间隔，单位秒，默认 30
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Port         int    `mapstructure:"port"`
	Mode         string `mapstructure:"mode"` // debug, release
	ReadTimeout  int    `mapstructure:"read_timeout"`
	WriteTimeout int    `mapstructure:"write_timeout"`
	IdleTimeout  int    `mapstructure:"idle_timeout"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Type            string `mapstructure:"type"` // mysql, postgres
	Host            string `mapstructure:"host"`
	Port            int    `mapstructure:"port"`
	Username        string `mapstructure:"username"`
	Password        string `mapstructure:"password"`
	Database        string `mapstructure:"database"`
	MaxOpenConns    int    `mapstructure:"max_open_conns"`
	MaxIdleConns    int    `mapstructure:"max_idle_conns"`
	ConnMaxLifetime int    `mapstructure:"conn_max_lifetime"` // 秒
}

// JWTConfig JWT 配置
type JWTConfig struct {
	Secret             string `mapstructure:"secret"`
	AccessTokenExpire  int    `mapstructure:"access_token_expire"`  // 秒
	RefreshTokenExpire int    `mapstructure:"refresh_token_expire"` // 秒
	Issuer             string `mapstructure:"issuer"`
}

// EncryptionConfig 加密配置
type EncryptionConfig struct {
	Key string `mapstructure:"key"` // 32 字节的 AES-256 密钥（Base64 编码）
}

// LogConfig 日志配置
type LogConfig struct {
	Level      string `mapstructure:"level"`       // debug, info, warn, error
	Format     string `mapstructure:"format"`      // json, console
	OutputPath string `mapstructure:"output_path"` // stdout, 文件路径
}

// VictoriaMetricsConfig VictoriaMetrics 配置
type VictoriaMetricsConfig struct {
	WriteURL string `mapstructure:"write_url"` // 写入地址，如 http://localhost:8428/insert/0/prometheus
	QueryURL string `mapstructure:"query_url"` // 查询地址，如 http://localhost:8428/select/0/prometheus
	Enabled  bool   `mapstructure:"enabled"`   // 是否启用
}

// Load 加载配置
func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./configs")
	viper.AddConfigPath(".")

	// 设置默认值
	setDefaults()

	// 读取环境变量
	viper.AutomaticEnv()

	// 读取配置文件
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// 配置文件不存在，使用默认值
			fmt.Println("Config file not found, using defaults")
		} else {
			return nil, fmt.Errorf("failed to read config: %w", err)
		}
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// 验证配置
	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &cfg, nil
}

// setDefaults 设置默认配置
func setDefaults() {
	// 服务器默认配置
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.mode", "debug")
	viper.SetDefault("server.read_timeout", 30)
	viper.SetDefault("server.write_timeout", 30)
	viper.SetDefault("server.idle_timeout", 60)

	// 数据库默认配置
	viper.SetDefault("database.type", "mysql")
	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.port", 3306)
	viper.SetDefault("database.username", "root")
	viper.SetDefault("database.password", "")
	viper.SetDefault("database.database", "kafka_management")
	viper.SetDefault("database.max_open_conns", 50)
	viper.SetDefault("database.max_idle_conns", 10)
	viper.SetDefault("database.conn_max_lifetime", 3600)

	// JWT 默认配置
	viper.SetDefault("jwt.secret", "change-this-secret-key")
	viper.SetDefault("jwt.access_token_expire", 3600)    // 1 小时
	viper.SetDefault("jwt.refresh_token_expire", 604800) // 7 天
	viper.SetDefault("jwt.issuer", "kafka-management-platform")

	// 加密默认配置（需要用户配置）
	viper.SetDefault("encryption.key", "")

	// 日志默认配置
	viper.SetDefault("log.level", "info")
	viper.SetDefault("log.format", "json")
	viper.SetDefault("log.output_path", "stdout")

	// VictoriaMetrics 默认配置
	viper.SetDefault("victoriametrics.write_url", "http://localhost:8428/insert/0/prometheus")
	viper.SetDefault("victoriametrics.query_url", "http://localhost:8428/select/0/prometheus")
	viper.SetDefault("victoriametrics.enabled", true)

	// SyncWorker 默认配置
	viper.SetDefault("syncworker.interval", 30)
}

// validate 验证配置
func validate(cfg *Config) error {
	if cfg.Server.Port <= 0 || cfg.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", cfg.Server.Port)
	}

	if cfg.Database.Type != "mysql" && cfg.Database.Type != "postgres" {
		return fmt.Errorf("unsupported database type: %s", cfg.Database.Type)
	}

	if cfg.JWT.Secret == "" || cfg.JWT.Secret == "change-this-secret-key" {
		return fmt.Errorf("JWT secret must be configured")
	}

	if cfg.Encryption.Key == "" {
		return fmt.Errorf("encryption key must be configured")
	}

	// 验证加密密钥长度（Base64 编码后应该是 44 字符，对应 32 字节）
	if len(cfg.Encryption.Key) != 44 {
		return fmt.Errorf("encryption key must be 32 bytes (44 characters in Base64)")
	}

	return nil
}
