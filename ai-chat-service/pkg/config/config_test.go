package config

import (
	"os"
	"path/filepath"
	"testing"
)

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
