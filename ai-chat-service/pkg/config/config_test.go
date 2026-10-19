package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigureProvider(t *testing.T) {
	for _, tt := range []struct {
		provider string
		key      string
		wantErr  string
	}{
		{provider: ""},
		{provider: "mock", key: "unused-key"},
		{provider: "openai", key: "test-key"},
		{provider: "openai", key: " ", wantErr: "OPENAI_API_KEY"},
		{provider: "typo", wantErr: "AI_PROVIDER"},
	} {
		t.Run(tt.provider+"/"+tt.wantErr, func(t *testing.T) {
			t.Setenv("AI_PROVIDER", tt.provider)
			t.Setenv("OPENAI_API_KEY", tt.key)
			conf := &Config{}
			conf.Chat.ApiKey = "local-mock-token"
			conf.Chat.BaseUrl = "http://mock-openai-api:8083/v1"
			conf.Chat.Model = "gpt-3.5-turbo"
			err := conf.configureProvider()
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("configureProvider() = %v, want %s", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tt.provider == "openai" {
				if conf.Chat.ApiKey != tt.key || conf.Chat.BaseUrl != "https://api.openai.com/v1" || conf.Chat.Model != "gpt-6-luna" {
					t.Fatal("official provider did not select the official endpoint, model and key")
				}
			} else if conf.Chat.Provider != "mock" || conf.Chat.ApiKey != "local-mock-token" || conf.Chat.BaseUrl != "http://mock-openai-api:8083/v1" || conf.Chat.Model != "gpt-3.5-turbo" {
				t.Fatal("mock configuration changed")
			}
		})
	}
}

func TestVectorDBEnabled(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{name: "existing config", content: "{}\n", want: true},
		{name: "explicitly enabled", content: "vectorDB:\n  enabled: true\n", want: true},
		{name: "local mode", content: "vectorDB:\n  enabled: false\n", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(tt.content), 0600); err != nil {
				t.Fatal(err)
			}
			InitConfig(path)
			if got := GetConfig().VectorDB.Enabled; got != tt.want {
				t.Fatalf("VectorDB.Enabled = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVectorDBDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("{}\n"), 0600); err != nil {
		t.Fatal(err)
	}

	InitConfig(path)
	vectorDB := GetConfig().VectorDB
	if vectorDB.Address != "127.0.0.1:50051" {
		t.Fatalf("VectorDB.Address = %q", vectorDB.Address)
	}
	if vectorDB.Timeout != 5 {
		t.Fatalf("VectorDB.Timeout = %d", vectorDB.Timeout)
	}
	if vectorDB.Threshold != 0.01 {
		t.Fatalf("VectorDB.Threshold = %v", vectorDB.Threshold)
	}
}
