package config

import (
	"fmt"
	"github.com/spf13/viper"
	"log"
	"os"
	"strings"
)

type Config struct {
	Server struct {
		IP          string
		Port        int
		AccessToken string
	}
	Log struct {
		Level   string
		LogPath string `mapstructure:"logPath"`
	} `mapstructure:"log"`
	Chat struct {
		Provider          string  `mapstructure:"provider"`
		ApiKey            string  `mapstructure:"api_key"`
		BaseUrl           string  `mapstructure:"base_url"`
		Model             string  `mapstructure:"model"`
		MaxTokens         int     `mapstructure:"max_tokens"`
		Temperature       float32 `mapstructure:"temperature"`
		TopP              float32 `mapstructure:"top_p"`
		PresencePenalty   float32 `mapstructure:"presence_penalty"`
		FrequencyPenalty  float32 `mapstructure:"frequency_penalty"`
		BotDesc           string  `mapstructure:"bot_desc"`
		MinResponseTokens int     `mapstructure:"min_response_tokens"`
		ContextTTL        int     `mapstructure:"context_ttl"`
		ContextLen        int     `mapstructure:"context_len"`
	}
	Redis struct {
		Host string
		Port int
		Pwd  string `mapstructure:"pwd"`
	}
	DependOn struct {
		Sensitive struct {
			Address     string
			AccessToken string
		}
		Tokenizer struct {
			Address string
		}
	}
	VectorDB struct {
		Enabled   bool
		Address   string
		Timeout   int
		Threshold float32
	}
}

var conf *Config

func InitConfig(filePath string, typ ...string) {
	v := viper.New()
	v.SetDefault("vectorDB.enabled", true)
	v.SetDefault("vectorDB.address", "127.0.0.1:50051")
	v.SetDefault("vectorDB.timeout", 5)
	v.SetDefault("vectorDB.threshold", 0.01)
	v.SetConfigFile(filePath)
	if len(typ) > 0 {
		v.SetConfigType(typ[0])
	}
	err := v.ReadInConfig()
	if err != nil {
		log.Fatal(err)
	}
	conf = &Config{}
	err = v.Unmarshal(conf)
	if err != nil {
		log.Fatal(err)
	}
	if err := conf.configureProvider(); err != nil {
		log.Fatal(err)
	}
}

func (c *Config) configureProvider() error {
	if provider, ok := os.LookupEnv("AI_PROVIDER"); ok {
		c.Chat.Provider = provider
	}
	c.Chat.Provider = strings.ToLower(strings.TrimSpace(c.Chat.Provider))
	switch c.Chat.Provider {
	case "", "mock":
		c.Chat.Provider = "mock"
	case "openai":
		key := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
		if key == "" {
			return fmt.Errorf("OPENAI_API_KEY is required when AI_PROVIDER=openai")
		}
		c.Chat.ApiKey = key
		c.Chat.BaseUrl = "https://api.openai.com/v1"
		c.Chat.Model = "gpt-6-luna"
	default:
		return fmt.Errorf("invalid AI_PROVIDER %q: expected mock or openai", c.Chat.Provider)
	}
	return nil
}

func GetConfig() *Config {
	return conf
}
