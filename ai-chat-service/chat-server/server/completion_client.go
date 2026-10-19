package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"
)

var completionHTTPClient = &http.Client{Timeout: 5 * time.Minute}

type completionClient struct {
	baseURL  string
	apiKey   string
	official bool
}

func (c *completionClient) post(ctx context.Context, request openai.ChatCompletionRequest) (*http.Response, error) {
	var payload interface{} = request
	if c.official {
		payload = struct {
			Model               string                         `json:"model"`
			Messages            []openai.ChatCompletionMessage `json:"messages"`
			MaxCompletionTokens int                            `json:"max_completion_tokens"`
			ReasoningEffort     string                         `json:"reasoning_effort"`
			Stream              bool                           `json:"stream"`
		}{request.Model, request.Messages, request.MaxTokens, "none", request.Stream}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.baseURL, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	if request.Stream {
		req.Header.Set("Accept", "text/event-stream")
	}
	resp, err := completionHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		if err != nil {
			return nil, fmt.Errorf("model API HTTP %s: read error: %w", resp.Status, err)
		}
		return nil, fmt.Errorf("model API HTTP %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return resp, nil
}

func (c *completionClient) CreateChatCompletion(ctx context.Context, request openai.ChatCompletionRequest) (response openai.ChatCompletionResponse, err error) {
	request.Stream = false
	resp, err := c.post(ctx, request)
	if err != nil {
		return response, err
	}
	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(&response)
	return
}

func (c *completionClient) CreateChatCompletionStream(ctx context.Context, request openai.ChatCompletionRequest) (*completionStream, error) {
	request.Stream = true
	resp, err := c.post(ctx, request)
	if err != nil {
		return nil, err
	}
	return &completionStream{body: resp.Body, reader: bufio.NewReader(resp.Body)}, nil
}

type completionStream struct {
	body   io.ReadCloser
	reader *bufio.Reader
	done   bool
}

func (s *completionStream) Close() error {
	return s.body.Close()
}

func (s *completionStream) Recv() (response openai.ChatCompletionStreamResponse, err error) {
	if s.done {
		return response, io.EOF
	}
	var data []string
	for {
		line, err := s.reader.ReadString('\n')
		if err == io.EOF {
			return response, io.ErrUnexpectedEOF
		}
		if err != nil {
			return response, err
		}
		line = strings.TrimRight(line, "\r\n")
		if value, ok := strings.CutPrefix(line, "data:"); ok {
			data = append(data, strings.TrimPrefix(value, " "))
		}
		if line != "" || len(data) == 0 {
			continue
		}
		payload := strings.Join(data, "\n")
		if payload == "[DONE]" {
			s.done = true
			return response, io.EOF
		}
		var event struct {
			openai.ChatCompletionStreamResponse
			Error *openai.APIError `json:"error"`
		}
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			return response, fmt.Errorf("decode model API stream: %w", err)
		}
		if event.Error != nil {
			return response, event.Error
		}
		return event.ChatCompletionStreamResponse, nil
	}
}
