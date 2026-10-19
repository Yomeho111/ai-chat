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

func TestOfficialModelAndContextIsolation(t *testing.T) {
	conf := &config.Config{}
	conf.Chat.Provider = "openai"
	conf.Chat.Model = "gpt-6-luna"
	conf.Chat.ContextLen = 4
	input := &proto.ChatCompletionRequest{ChatParam: &proto.ChatParam{Model: "gpt-3.5-turbo"}}
	cache := tokenUsageContextCache{
		"mock-answer": {Message: openai.ChatCompletionMessage{Role: "assistant", Content: "mock"}},
	}
	app := (&chatService{config: conf}).newApp(input, cache)
	if app.openaiConf.Model != "gpt-6-luna" || app.cacheScope() != "openai/gpt-6-luna" {
		t.Fatal("backend overrode official model or cache scope")
	}
	if len(app.getContext("mock-answer")) != 0 {
		t.Fatal("official mode reused mock history")
	}
	if err := app.saveContext(&chat_context.ChatMessage{ID: "official-answer"}); err != nil {
		t.Fatal(err)
	}
	if cache["official-answer"] != nil || cache["openai/gpt-6-luna:official-answer"] == nil {
		t.Fatal("official history was not isolated")
	}
	conf.Chat.Provider = "mock"
	app = (&chatService{config: conf}).newApp(input, cache)
	if app.openaiConf.Model != "gpt-3.5-turbo" || app.cacheScope() != "" || len(app.getContext("mock-answer")) != 1 {
		t.Fatal("mock model override or history behavior changed")
	}
}

func TestSavedTokenUsage(t *testing.T) {
	tests := []struct {
		name            string
		system          string
		enableContext   bool
		originalContext bool
		maxTokens       int
		contextLen      int
		failContent     string
		recordedTokens  int32
		wantError       bool
	}{
		{name: "question and answer with message overhead", recordedTokens: 49},
		{name: "system prompt matches original API total", system: "system", recordedTokens: 89},
		{name: "disabled context", system: "system", recordedTokens: 89},
		{name: "new context must not inflate saved tokens", system: "system", enableContext: true, recordedTokens: 89},
		{name: "context limited by budget", system: "system", enableContext: true, maxTokens: 2139, recordedTokens: 89},
		{name: "context limited by count", system: "system", enableContext: true, contextLen: 1, recordedTokens: 89},
		{name: "original generation included different context", system: "system", originalContext: true, recordedTokens: 140},
		{name: "question exceeds budget", system: "system", maxTokens: 2094, recordedTokens: 89, wantError: true},
		{name: "system tokenizer failure", system: "system", failContent: "system", recordedTokens: 89, wantError: true},
		{name: "question tokenizer failure", failContent: "Hello", recordedTokens: 49, wantError: true},
		{name: "cached answer is not retokenized", failContent: "answer", recordedTokens: 49},
		{name: "missing generation count", wantError: true},
		{name: "negative generation count", recordedTokens: -1, wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			counts := map[string]int{"system": 40, "Hello": 18, "answer": 29}
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
					Message: openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: "answer"},
					Tokens:  counts["answer"],
				},
				"previous-question": {
					Message: openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: "Hello"},
					Tokens:  counts["Hello"],
				},
			}
			in := &proto.ChatCompletionRequest{
				Message:       "Hello",
				Pid:           "previous-answer",
				EnableContext: tt.enableContext,
				ChatParam:     &proto.ChatParam{Model: "test-model"},
			}
			app := (&chatService{config: conf, log: logger}).newApp(in, cache)
			usage, err := app.savedTokenUsage(in, tt.recordedTokens)
			if tt.wantError {
				if err == nil || usage != nil {
					t.Fatalf("savedTokenUsage() = %v, %v; want nil usage and error", usage, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if usage.SavedTokens != tt.recordedTokens || usage.ConsumedTokens != 0 {
				t.Fatalf("usage = %v, want saved %d and consumed 0", usage, tt.recordedTokens)
			}
			originalRequest := *in
			originalRequest.EnableContext = tt.originalContext
			_, originalPromptTokens, _, _, err := app.buildChatCompletionRequest(&originalRequest, true)
			if err != nil {
				t.Fatal(err)
			}
			if want := int32(originalPromptTokens + counts["answer"]); usage.SavedTokens != want {
				t.Fatalf("saved tokens = %d, original API consumed tokens = %d", usage.SavedTokens, want)
			}
			if tt.name == "new context must not inflate saved tokens" {
				_, repeatedPromptTokens, _, _, err := app.buildChatCompletionRequest(in, true)
				if err != nil {
					t.Fatal(err)
				}
				if repeatedTotal := repeatedPromptTokens + counts["answer"]; repeatedTotal != 140 {
					t.Fatalf("repeated request estimate = %d, want regression fixture of 140 (original cost 89)", repeatedTotal)
				}
			}
			if len(cache) != 2 {
				t.Fatal("estimating saved tokens must not write context")
			}
		})
	}
}
