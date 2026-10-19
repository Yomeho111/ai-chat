package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sashabaranov/go-openai"
)

func TestCompletionHTTPProtocol(t *testing.T) {
	for _, official := range []bool{false, true} {
		for _, stream := range []bool{false, true} {
			t.Run(fmt.Sprintf("official=%v/stream=%v", official, stream), func(t *testing.T) {
				upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method != http.MethodPost || r.URL.Path != "/v1/chat/completions" ||
						r.Header.Get("Authorization") != "Bearer test-key" || r.Header.Get("Content-Type") != "application/json" {
						t.Errorf("unexpected request: %s %s, headers=%v", r.Method, r.URL.Path, r.Header)
					}
					var payload map[string]json.RawMessage
					if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
						t.Error(err)
						http.Error(w, "bad JSON", http.StatusBadRequest)
						return
					}
					model := "gpt-3.5-turbo"
					if official {
						model = "gpt-6-luna"
					}
					if string(payload["model"]) != fmt.Sprintf("%q", model) ||
						string(payload["messages"]) != `[{"role":"user","content":"Hello"}]` {
						t.Errorf("unexpected model or messages: %v", payload)
					}
					if official {
						if string(payload["max_completion_tokens"]) != "123" || string(payload["reasoning_effort"]) != `"none"` {
							t.Error("missing official completion token limit or reasoning setting")
						}
						for _, key := range []string{"max_tokens", "temperature", "top_p", "presence_penalty", "frequency_penalty"} {
							if _, found := payload[key]; found {
								t.Errorf("official request included legacy parameter %s", key)
							}
						}
					} else if string(payload["max_tokens"]) != "123" || string(payload["temperature"]) != "0.8" {
						t.Error("mock request parameters changed")
					}
					if stream {
						if string(payload["stream"]) != "true" || r.Header.Get("Accept") != "text/event-stream" {
							t.Error("missing streaming settings")
						}
						w.Header().Set("Content-Type", "text/event-stream")
						fmt.Fprint(w, ": keepalive\r\n\r\ndata: {\"id\":\"answer\",\"choices\":[{\"delta\":{\"role\":\"assistant\",\"content\":\"Hello\"},\"finish_reason\":null}]}\r\n\r\n")
						w.(http.Flusher).Flush()
						fmt.Fprint(w, "data: {\"id\":\"answer\",\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
					} else {
						fmt.Fprintf(w, `{"id":"answer","model":%q,"choices":[{"message":{"role":"assistant","content":"Hello"},"finish_reason":"stop"}]}`, model)
					}
				}))
				defer upstream.Close()
				client := &completionClient{baseURL: upstream.URL + "/v1/", apiKey: "test-key", official: official}
				request := openai.ChatCompletionRequest{
					Model: "gpt-3.5-turbo", Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "Hello"}},
					MaxTokens: 123, Temperature: 0.8, TopP: 0.9, PresencePenalty: 0.8, FrequencyPenalty: 0.5,
				}
				if official {
					request.Model = "gpt-6-luna"
				}
				if !stream {
					response, err := client.CreateChatCompletion(context.Background(), request)
					if err != nil || len(response.Choices) != 1 || response.Choices[0].Message.Content != "Hello" {
						t.Fatalf("CreateChatCompletion() = %#v, %v", response, err)
					}
					return
				}
				response, err := client.CreateChatCompletionStream(context.Background(), request)
				if err != nil {
					t.Fatal(err)
				}
				defer response.Close()
				first, err := response.Recv()
				if err != nil || len(first.Choices) != 1 || first.Choices[0].Delta.Content != "Hello" {
					t.Fatalf("first chunk = %#v, %v", first, err)
				}
				last, err := response.Recv()
				if err != nil || len(last.Choices) != 1 || last.Choices[0].FinishReason != "stop" {
					t.Fatalf("last chunk = %#v, %v", last, err)
				}
				for i := 0; i < 2; i++ {
					if _, err := response.Recv(); err != io.EOF {
						t.Fatalf("Recv after DONE = %v, want EOF", err)
					}
				}
			})
		}
	}
}

func TestCompletionHTTPError(t *testing.T) {
	for _, code := range []int{401, 429, 500} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, `{"error":{"message":"upstream rejected request"}}`, code)
			}))
			defer upstream.Close()
			client := &completionClient{baseURL: upstream.URL, official: true}
			if _, err := client.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{}); err == nil || !strings.Contains(err.Error(), fmt.Sprint(code)) {
				t.Fatalf("HTTP error = %v", err)
			}
			if _, err := client.CreateChatCompletionStream(context.Background(), openai.ChatCompletionRequest{}); err == nil || !strings.Contains(err.Error(), fmt.Sprint(code)) {
				t.Fatalf("stream HTTP error = %v", err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if _, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{}); !errors.Is(err, context.Canceled) {
				t.Fatalf("canceled request = %v", err)
			}
		})
	}
}

func TestCompletionStreamErrors(t *testing.T) {
	for _, tt := range []struct {
		name string
		body string
		want string
	}{
		{name: "truncated", body: "data: {}", want: "unexpected EOF"},
		{name: "invalid JSON", body: "data: not-json\n\n", want: "decode model API stream"},
		{name: "API error", body: "data: {\"error\":{\"message\":\"rate limit exceeded\",\"type\":\"rate_limit_error\"}}\n\n", want: "rate limit exceeded"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, tt.body)
			}))
			defer upstream.Close()
			client := &completionClient{baseURL: upstream.URL, official: true}
			stream, err := client.CreateChatCompletionStream(context.Background(), openai.ChatCompletionRequest{})
			if err != nil {
				t.Fatal(err)
			}
			defer stream.Close()
			if _, err := stream.Recv(); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Recv() = %v, want %s", err, tt.want)
			}
		})
	}
}
