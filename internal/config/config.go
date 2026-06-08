package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Postgres struct {
		Host     string `yaml:"host" env-default:"127.0.0.1"`
		Port     string `yaml:"port" env-default:"5432"`
		User     string `yaml:"user" env-default:"postgres"`
		Password string `yaml:"password" env-default:"password"`
		DBName   string `yaml:"dbname" env-default:"postgres"`
	} `yaml:"postgres"`

	Logger struct {
		LogLevel    string `yaml:"logLevel" env-default:"info"`
		Mode        string `yaml:"mode" env-default:"development"`
		LogEncoding string `yaml:"encoding" env-default:"console"`
	} `yaml:"logger"`

	Server struct {
		Port int `yaml:"port" env-default:"8080"`
	} `yaml:"server"`

	Gateway struct {
		PublicBaseURL string `yaml:"publicBaseUrl" env:"GATEWAY_PUBLIC_BASE_URL" env-default:"http://localhost:8888"`
	} `yaml:"gateway"`

	Keycloak struct {
		Endpoint     string `yaml:"endpoint" env:"KEYCLOAK_ENDPOINT" env-default:"http://localhost:8080"`
		ClientID     string `yaml:"clientId" env:"KEYCLOAK_CLIENT_ID"`
		ClientSecret string `yaml:"clientSecret" env:"KEYCLOAK_CLIENT_SECRET"`
		PublicKey    string `yaml:"publicKey" env:"KEYCLOAK_PUBLIC_KEY"`
		Realm        string `yaml:"realm" env:"KEYCLOAK_REALM"`
		CustomHeader string `yaml:"customHeaderName" env:"KEYCLOAK_CUSTOM_HEADER_NAME"`
	} `yaml:"keycloak"`

	Otel struct {
		Endpoint string `yaml:"endpoint" env:"OTEL_EXPORTER_OTLP_ENDPOINT" env-default:"localhost:4318"`
		Service  string `yaml:"service" env:"OTEL_SERVICE_NAME" env-default:"app"`
	} `yaml:"otel"`
}

func NewConfig() (*Config, error) {
	var config Config
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		var err error
		configPath, err = defaultConfigPath()
		if err != nil {
			return nil, err
		}
	}

	if err := cleanenv.ReadConfig(configPath, &config); err != nil {
		return nil, err
	}
	if tracesEndpoint := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT")); tracesEndpoint != "" {
		config.Otel.Endpoint = tracesEndpoint
	}
	return &config, nil
}

func defaultConfigPath() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	serviceName := strings.TrimSpace(os.Getenv("SERVICE_NAME"))
	if serviceName != "" {
		configPath := filepath.Join(cwd, "config", serviceName+".yaml")
		if _, err := os.Stat(configPath); err == nil {
			return configPath, nil
		} else if !os.IsNotExist(err) {
			return "", err
		}
	}

	configPath := filepath.Join(cwd, "config.yaml")
	if _, err := os.Stat(configPath); err == nil {
		return configPath, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}

	return filepath.Join(cwd, "config.yml"), nil
}
