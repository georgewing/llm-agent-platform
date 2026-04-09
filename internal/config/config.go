package config

import (
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server    ServerConfig
	Log       LogConfig
	Database  DatabaseConfig
	Redis     RedisConfig
	Milvus    MilvusConfig
	ES        ESConfig
	Embedding EmbeddingConfig
	Rerank    RerankConfig
	LLM       LLMConfig
	Agent     AgentConfig
	Gateway   GatewayConfig
	Workflow  WorkflowConfig
}

// ServerConfig HTTP服务配置
type ServerConfig struct {
	Port            string        `mapstructure:"port"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	IdleTimeout     time.Duration `mapstructure:"idle_timeout"`
	MaxHeaderBytes  int           `mapstructure:"max_header_bytes"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
}

// LogConfig 日志配置
type LogConfig struct {
	Level  string `mapstructure:"level"`  // debug, info, warn, error
	Format string `mapstructure:"format"` // json, text
	Output string `mapstructure:"output"` // console, file
}

// DatabaseConfig PostgreSQL配置
type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
	SSLMode  string `mapstructure:"sslmode"`
}

// RedisConfig Redis缓存配置
type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

// MilvusConfig Milvus向量数据库配置
type MilvusConfig struct {
	Host           string `mapstructure:"host"`
	Port           int    `mapstructure:"port"`
	Username       string `mapstructure:"username"`
	Password       string `mapstructure:"password"`
	DBName         string `mapstructure:"dbname"`
	CollectionName string `mapstructure:"collection_name"`
	VectorDim      int    `mapstructure:"vector_dim"`
	MetricType     string `mapstructure:"metric_type"`
	IndexType      string `mapstructure:"index_type"`
}

// ESConfig Elasticsearch配置
type ESConfig struct {
	Addresses []string `mapstructure:"addresses"`
	Username  string   `mapstructure:"username"`
	Password  string   `mapstructure:"password"`
	APIKey    string   `mapstructure:"api_key"`
	IndexName string   `mapstructure:"index_name"`
	Timeout   int      `mapstructure:"timeout"`
}

// EmbeddingConfig Embedding服务配置
type EmbeddingConfig struct {
	APIKey     string `mapstructure:"api_key"`
	BaseURL    string `mapstructure:"base_url"`
	Model      string `mapstructure:"model"`
	Timeout    int    `mapstructure:"timeout"`
	MaxRetries int    `mapstructure:"max_retries"`
}

// RerankConfig Rerank服务配置
type RerankConfig struct {
	APIKey     string `mapstructure:"api_key"`
	BaseURL    string `mapstructure:"base_url"`
	Model      string `mapstructure:"model"`
	Timeout    int    `mapstructure:"timeout"`
	MaxRetries int    `mapstructure:"max_retries"`
}

// LLMConfig 大模型配置
type LLMConfig struct {
	APIKey     string `mapstructure:"api_key"`
	BaseURL    string `mapstructure:"base_url"`
	Model      string `mapstructure:"model"`
	Timeout    int    `mapstructure:"timeout"`
	MaxRetries int    `mapstructure:"max_retries"`
}

// AgentConfig 智能体配置
type AgentConfig struct {
	APIKey     string `mapstructure:"api_key"`
	BaseURL    string `mapstructure:"base_url"`
	Model      string `mapstructure:"model"`
	Timeout    int    `mapstructure:"timeout"`
	MaxRetries int    `mapstructure:"max_retries"`
}

// GatewayConfig LLM代理网关配置
type GatewayConfig struct {
	EnableAuth      bool          `mapstructure:"enable_auth"`
	EnableRateLimit bool          `mapstructure:"enable_rate_limit"`
	EnableCache     bool          `mapstructure:"enable_cache"`
	CacheTTL        time.Duration `mapstructure:"cache_ttl"`
}

// WorkflowConfig
type WorkflowConfig struct {
	MaxWorkers int `mapstructure:"max_workers"`
}

// Load 加载配置
func Load() *Config {
	setDefaults()
	bindEnv()

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		panic(err)
	}
	return &config
}

func setDefaults() {
	// server
	viper.SetDefault("server.port", "8080")
	viper.SetDefault("server.read_timeout", 5*time.Second)
	viper.SetDefault("server.write_timeout", 10*time.Second)
	viper.SetDefault("server.idle_timeout", 5*time.Second)
	viper.SetDefault("server.max_header_bytes", 1<<20)
	viper.SetDefault("server.shutdown_timeout", 5*time.Second)
}

func bindEnv() {
	// Server
	viper.BindEnv("server.port", "PORT")
}
