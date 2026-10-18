package server

import (
	chat_context "ai-chat-service/chat-server/chat-context"
	"ai-chat-service/pkg/config"
	"ai-chat-service/pkg/log"
	"ai-chat-service/proto"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/sashabaranov/go-openai"
)

type tokenUsageContextCache map[string]*chat_context.ChatMessage

func (c tokenUsageContextCache) Get(key string) (*chat_context.ChatMessage, error) {
	return c[key], nil
}

func (c tokenUsageContextCache) Set(key string, value *chat_context.ChatMessage, _ int) error {
	c[key] = value
	return nil
}

func (c tokenUsageContextCache) Close() {}

func TestSavedTokenUsage(t *testing.T) {
	tests := []struct {
		name          string
		system        string
		enableContext bool
		maxTokens     int
		contextLen    int
		failContent   string
		want          int32
		wantError     bool
	}{
		{name: "question and answer with message overhead", want: 36},
		{name: "system prompt matches API total", system: "system", want: 76},
		{name: "disabled context", system: "system", want: 76},
		{name: "enabled context", system: "system", enableContext: true, want: 104},
		{name: "context limited by budget", system: "system", enableContext: true, maxTokens: 2110, want: 91},
		{name: "context limited by count", system: "system", enableContext: true, contextLen: 1, want: 91},
		{name: "question exceeds budget", system: "system", maxTokens: 2094, wantError: true},
		{name: "system tokenizer failure", system: "system", failContent: "system", wantError: true},
		{name: "question tokenizer failure", failContent: "Hello", wantError: true},
		{name: "answer tokenizer failure", failContent: "answer", wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			counts := map[string]int{"system": 40, "Hello": 5, "answer": 29}
			tokenizerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/tokenizer/test-model" {
					t.Errorf("tokenizer path = %q, want overridden model", r.URL.Path)
				}
				var message openai.ChatCompletionMessage
				if err := json.NewDecoder(r.Body).Decode(&message); err != nil {
					t.Errorf("decode tokenizer request: %v", err)
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				if message.Content == tt.failContent {
					http.Error(w, "tokenizer unavailable", http.StatusServiceUnavailable)
					return
				}
				tokens, ok := counts[message.Content]
				if !ok {
					t.Errorf("unexpected tokenizer message: %#v", message)
					http.Error(w, "unexpected message", http.StatusBadRequest)
					return
				}
				if err := json.NewEncoder(w).Encode(map[string]int{"code": 200, "num_tokens": tokens}); err != nil {
					t.Errorf("encode tokenizer response: %v", err)
				}
			}))
			defer tokenizerServer.Close()

			configPath := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(configPath, []byte(fmt.Sprintf("dependOn:\n  tokenizer:\n    address: %q\n", tokenizerServer.URL)), 0600); err != nil {
				t.Fatal(err)
			}
			config.InitConfig(configPath)
			logger := log.NewLogger()
			logger.SetOutput(io.Discard)
			conf := &config.Config{}
			conf.Chat.Model = "default-model"
			conf.Chat.BotDesc = tt.system
			conf.Chat.MaxTokens = 4096
			if tt.maxTokens != 0 {
				conf.Chat.MaxTokens = tt.maxTokens
			}
			conf.Chat.MinResponseTokens = 2048
			conf.Chat.ContextLen = 4
			if tt.contextLen != 0 {
				conf.Chat.ContextLen = tt.contextLen
			}
			cache := tokenUsageContextCache{
				"previous-answer": {
					PID:     "previous-question",
					Message: openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: "previous answer"},
					Tokens:  13,
				},
				"previous-question": {
					Message: openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: "previous question"},
					Tokens:  11,
				},
			}
			in := &proto.ChatCompletionRequest{
				Message:       "Hello",
				Pid:           "previous-answer",
				EnableContext: tt.enableContext,
				ChatParam:     &proto.ChatParam{Model: "test-model"},
			}
			app := (&chatService{config: conf, log: logger}).newApp(in, cache)
			usage, err := app.savedTokenUsage(in, "answer")
			if tt.wantError {
				if err == nil || usage != nil {
					t.Fatalf("savedTokenUsage() = %v, %v; want nil usage and error", usage, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if usage.SavedTokens != tt.want || usage.ConsumedTokens != 0 {
				t.Fatalf("usage = %v, want saved %d and consumed 0", usage, tt.want)
			}
			_, promptTokens, _, _, err := app.buildChatCompletionRequest(in, true)
			if err != nil {
				t.Fatal(err)
			}
			if want := int32(promptTokens + counts["answer"]); usage.SavedTokens != want {
				t.Fatalf("saved tokens = %d, API consumed tokens = %d", usage.SavedTokens, want)
			}
			if len(cache) != 2 {
				t.Fatal("estimating saved tokens must not write context")
			}
		})
	}
}
