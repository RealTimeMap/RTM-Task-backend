package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"

	"RTM-Task/internal/utils/database"
)

type HTTP struct {
	Host            string        `yaml:"host"`
	Port            int           `yaml:"port"`
	ReadTimeout     time.Duration `yaml:"read_timeout"`
	WriteTimeout    time.Duration `yaml:"write_timeout"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
}

func (h HTTP) Address() string {
	return fmt.Sprintf("%s:%d", h.Host, h.Port)
}

// SMTP — доступ к сервису почты платформы.
//
// Пустой BaseURL или ApiKey выключают уведомления: сервис продолжает
// работать, просто письма не уходят. Это удобно локально и не даёт
// упасть, если почта ещё не развёрнута.
type SMTP struct {
	BaseURL string        `yaml:"base_url"`
	ApiKey  string        `yaml:"api_key"`
	Timeout time.Duration `yaml:"timeout"`

	// AppURL подставляется в письма ссылкой на задачу.
	AppURL string `yaml:"app_url"`
}

// Enabled сообщает, настроена ли отправка писем.
func (s SMTP) Enabled() bool {
	return s.BaseURL != "" && s.ApiKey != ""
}

type Config struct {
	Env      string          `yaml:"env"`
	HTTP     HTTP            `yaml:"http"`
	Database database.Config `yaml:"database"`
	SMTP     SMTP            `yaml:"smtp"`
}

const defaultConfigPath = "./config/config.yaml"

// Load читает конфигурацию из YAML-файла. Путь берётся из CONFIG_PATH,
// иначе используется ./config/config.yaml.
func Load() (*Config, error) {
	path := os.Getenv("CONFIG_PATH")
	if path == "" {
		path = defaultConfigPath
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	cfg := defaults()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}

	if cfg.Database.DBName == "" {
		return nil, fmt.Errorf("config: database.db_name is required")
	}

	return cfg, nil
}

// MustLoad читает конфигурацию или паникует — вызывается на старте сервиса.
func MustLoad() *Config {
	cfg, err := Load()
	if err != nil {
		panic(err)
	}
	return cfg
}

func defaults() *Config {
	return &Config{
		Env: "local",
		HTTP: HTTP{
			Host:            "0.0.0.0",
			Port:            8091,
			ReadTimeout:     10 * time.Second,
			WriteTimeout:    10 * time.Second,
			ShutdownTimeout: 10 * time.Second,
		},
		Database: database.Config{
			Host:    "localhost",
			Port:    5432,
			User:    "postgres",
			SSLMode: "disable",
		},
		SMTP: SMTP{
			Timeout: 5 * time.Second,
		},
	}
}
