package setup

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
	"github.com/tahardi/bearclave/tee"
)

type Config struct {
	Platform tee.Platform `mapstructure:"platform"`
	Enclave  Enclave      `mapstructure:"enclave"`
	Nonclave Nonclave     `mapstructure:"nonclave"`
	Proxy    Proxy        `mapstructure:"proxy"`
}

type Enclave struct {
	Addr    string         `mapstructure:"addr"`
	AddrTLS string         `mapstructure:"addr_tls"`
	Args    map[string]any `mapstructure:"args,omitempty"`
}

func (e Enclave) GetArg(key string, defaultVal any) any {
	if e.Args == nil {
		return defaultVal
	}

	if val, ok := e.Args[key]; ok {
		return val
	}
	return defaultVal
}

type Proxy struct {
	Addr       string `mapstructure:"addr"`
	AddrTLS    string `mapstructure:"addr_tls"`
	RevAddr    string `mapstructure:"rev_addr"`
	RevAddrTLS string `mapstructure:"rev_addr_tls"`
}

type Nonclave struct {
	Measurement string         `mapstructure:"measurement"`
	Args        map[string]any `mapstructure:"args,omitempty"`
}

func (n Nonclave) GetArg(key string, defaultVal any) any {
	if n.Args == nil {
		return defaultVal
	}

	if val, ok := n.Args[key]; ok {
		return val
	}
	return defaultVal
}

func LoadConfig(configFile string) (*Config, error) {
	config := &Config{}
	if _, err := os.Stat(configFile); err != nil {
		return nil, fmt.Errorf("finding config %s: %w", configFile, err)
	}

	v := viper.New()
	v.SetConfigFile(configFile)

	ext := filepath.Ext(configFile)
	if ext != "" {
		v.SetConfigType(ext[1:]) // Remove the dot from the extension
	}

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	if err := v.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}
	return config, nil
}
