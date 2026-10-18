package main

import (
	"context"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sashabaranov/go-openai"
)

func TestChatCompletionStreamFinishes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/v1/chat/completions", chatCompletion)
	server := httptest.NewServer(router)
	defer server.Close()

	config := openai.DefaultConfig("test-token")
	config.BaseURL = server.URL + "/v1"
	client := openai.NewClientWithConfig(config)
	stream, err := client.CreateChatCompletionStream(context.Background(), openai.ChatCompletionRequest{
		Model: openai.GPT3Dot5Turbo,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: "hello"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()

	var content strings.Builder
	finishReason := ""
	for {
		response, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if len(response.Choices) != 1 {
			t.Fatalf("got %d choices, want 1", len(response.Choices))
		}
		content.WriteString(response.Choices[0].Delta.Content)
		if response.Choices[0].FinishReason != "" {
			finishReason = response.Choices[0].FinishReason
		}
	}
	if content.Len() == 0 {
		t.Fatal("stream returned no content")
	}
	if finishReason != "stop" {
		t.Fatalf("finish reason = %q, want stop", finishReason)
	}
}
