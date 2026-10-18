package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sashabaranov/go-openai"
	"log"
	"math/rand"
	"mock-openai-api/middleware"
	"mock-openai-api/pkg/config"
	"net/http"
	"time"
)

var (
	configFile = flag.String("config", "dev.config.yaml", "")
)

func main() {
	flag.Parse()
	config.InitConfig(*configFile)
	cnf := config.GetConfig()
	gin.SetMode(cnf.Http.Mode)
	r := gin.Default()
	r.GET("/health", func(c *gin.Context) {})

	r.Use(middleware.Auth())
	v1 := r.Group("/v1")
	v1.POST("/chat/completions", chatCompletion)
	err := r.Run(fmt.Sprintf("%s:%d", cnf.Http.Host, cnf.Http.Port))
	if err != nil {
		log.Println(err)
	}
}

var msgList = []string{
	"你好，有什么可以帮助你的吗？",
	"今天的天气是真的很不错，你应该好好的放松一下了？",
	"希望你学习golang，能够找到一份心仪的工作？",
}

func chatCompletion(ctx *gin.Context) {
	req := openai.ChatCompletionRequest{}
	ctx.ShouldBindJSON(&req)

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	index := r.Intn(len(msgList))
	msg := msgList[index]
	if !req.Stream {
		res := buildChatCompletionResponse(msg)
		ctx.JSON(http.StatusOK, res)
		return
	}
	list := buildChatCompletionStreamResponseList(msg)
	ctx.Header("Content-Type", "text/event-stream")
	ctx.Header("Cache-Control", "no-cache")
	for i := 0; i < len(list); i++ {
		item := list[i]
		bytes, err := json.Marshal(item)
		if err != nil {
			log.Printf("encode stream event: %v", err)
			return
		}
		if _, err := fmt.Fprintf(ctx.Writer, "data: %s\n\n", bytes); err != nil {
			log.Printf("write stream event: %v", err)
			return
		}
		ctx.Writer.Flush()
		time.Sleep(time.Millisecond * 100)
	}
	if _, err := ctx.Writer.Write([]byte("data: [DONE]\n\n")); err != nil {
		log.Printf("write stream terminator: %v", err)
		return
	}
	ctx.Writer.Flush()
}

func buildChatCompletionResponse(msg string) openai.ChatCompletionResponse {
	res := openai.ChatCompletionResponse{
		ID:      uuid.New().String(),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   "gpt-3.5-turbo-0301",
		Choices: []openai.ChatCompletionChoice{
			{
				Message: openai.ChatCompletionMessage{
					Role:    openai.ChatMessageRoleAssistant,
					Content: msg,
				},
				FinishReason: "stop",
			},
		},
		Usage: openai.Usage{
			PromptTokens:     10,
			CompletionTokens: 20,
			TotalTokens:      30,
		},
	}
	return res
}

func buildChatCompletionStreamResponseList(msg string) []openai.ChatCompletionStreamResponse {
	id := uuid.New().String()
	list := make([]openai.ChatCompletionStreamResponse, 0)
	startRes := buildChatCompletionStreamResponse(id, "", "")
	list = append(list, startRes)
	for _, delta := range msg {
		list = append(list, buildChatCompletionStreamResponse(id, string(delta), ""))
	}
	endRes := buildChatCompletionStreamResponse(id, "", "stop")
	list = append(list, endRes)
	return list
}

func buildChatCompletionStreamResponse(id, delta, finishReason string) openai.ChatCompletionStreamResponse {
	res := openai.ChatCompletionStreamResponse{
		ID:      id,
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   "gpt-3.5-turbo-0301",
		Choices: []openai.ChatCompletionStreamChoice{
			{
				Index: 0,
				Delta: openai.ChatCompletionStreamChoiceDelta{
					Content: delta,
					Role:    openai.ChatMessageRoleAssistant,
				},
				FinishReason: finishReason,
			},
		},
	}
	return res
}
