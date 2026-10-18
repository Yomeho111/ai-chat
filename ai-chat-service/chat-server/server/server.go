package server

import (
	chat_context "ai-chat-service/chat-server/chat-context"
	metrics_bus "ai-chat-service/chat-server/metrics-bus"
	vector_data "ai-chat-service/chat-server/vector-data"
	"ai-chat-service/pkg/config"
	"ai-chat-service/pkg/log"
	"ai-chat-service/proto"
	"ai-chat-service/services/tokenizer"
	"context"
	"encoding/json"
	"fmt"
	"github.com/golang/protobuf/jsonpb"
	"github.com/google/uuid"
	"github.com/sashabaranov/go-openai"
	"io"
)

type chatService struct {
	proto.UnimplementedChatServer
	config        *config.Config
	log           log.ILogger
	semanticCache vector_data.IChatRecordsData
	busMetrics    *metrics_bus.BusMetrics
}

func NewChatService(semanticCache vector_data.IChatRecordsData, config *config.Config, log log.ILogger, busMetrics *metrics_bus.BusMetrics) proto.ChatServer {
	return &chatService{
		config:        config,
		log:           log,
		semanticCache: semanticCache,
		busMetrics:    busMetrics,
	}
}

func (s *chatService) ChatCompletion(ctx context.Context, in *proto.ChatCompletionRequest) (*proto.ChatCompletionResponse, error) {
	redisContextCache := chat_context.NewRedisCache()
	defer redisContextCache.Close()

	app := s.newApp(in, redisContextCache)
	//敏感词过滤
	ok, msg, err := app.sensitive(in)
	if err != nil {
		s.log.Error(err)
		return nil, err
	}
	if !ok {
		res := app.buildChatCompletionResponse(msg, answerSourceSensitiveFilter)
		res.TokenUsage = &proto.TokenUsage{}
		return res, nil
	}

	question := in.Message
	if question != "" && s.semanticCache != nil {
		answer, found, err := s.semanticCache.Find(ctx, question)
		if err != nil {
			s.log.Error(err)
		} else if found {
			usage, err := app.savedTokenUsage(in, answer)
			if err != nil {
				s.log.Error(err)
				return nil, err
			}
			res := app.buildChatCompletionResponse(answer, answerSourceVectorDB)
			res.TokenUsage = usage
			return res, nil
		}
	}

	client := app.getOpenaiClient()
	req, promptTokens, currTokens, currMessage, err := app.buildChatCompletionRequest(in, false)
	if err != nil {
		s.log.Error(err)
		return nil, err
	}
	resp, err := client.CreateChatCompletion(ctx, req)
	if err != nil {
		s.log.Error(err)
		return nil, err
	}
	if len(resp.Choices) == 0 {
		err := fmt.Errorf("chat completion returned no choices")
		s.log.Error(err)
		return nil, err
	}
	resultTokens, err := tokenizer.GetTokens(&resp.Choices[0].Message, req.Model)
	if err != nil {
		s.log.Error(err)
		return nil, err
	}
	res := &proto.ChatCompletionResponse{}
	bytes, err := json.Marshal(resp)
	if err != nil {
		s.log.Error(err)
		return nil, err
	}
	err = jsonpb.UnmarshalString(string(bytes), res)
	if err != nil {
		s.log.Error(err)
		return nil, err
	}
	res.Source = answerSourceLLMAPI
	res.TokenUsage = &proto.TokenUsage{ConsumedTokens: int32(promptTokens + resultTokens)}
	go func() {
		reqContext := &chat_context.ChatMessage{
			ID:      in.Id,
			PID:     in.Pid,
			Message: currMessage,
			Tokens:  currTokens,
		}
		err := app.saveContext(reqContext)
		if err != nil {
			s.log.Error(err)
			return
		}
		resContext := &chat_context.ChatMessage{
			ID:      resp.ID,
			PID:     reqContext.ID,
			Message: resp.Choices[0].Message,
			Tokens:  resultTokens,
		}
		err = app.saveContext(resContext)
		if err != nil {
			s.log.Error(err)
			return
		}
	}()
	go func() {
		if question != "" && s.semanticCache != nil {
			err := s.semanticCache.Save(context.Background(), question, resp.Choices[0].Message.Content)
			if err != nil {
				s.log.Error(err)
			}
		}
	}()
	return res, err
}
func (s *chatService) ChatCompletionStream(in *proto.ChatCompletionRequest, stream proto.Chat_ChatCompletionStreamServer) error {
	redisContextCache := chat_context.NewRedisCache()
	defer redisContextCache.Close()

	app := s.newApp(in, redisContextCache)
	//敏感词过滤
	ok, msg, err := app.sensitive(in)
	if err != nil {
		s.busMetrics.ErrQuestionsTotalCounter.Inc()
		s.log.Error(err)
		return err
	}
	if !ok {
		s.busMetrics.SensitiveQuestionsTotalCounter.Inc()
		resId := uuid.New().String()
		startRes := app.buildChatCompletionStreamResponse(resId, "", "", answerSourceSensitiveFilter)
		endRes := app.buildChatCompletionStreamResponse(resId, "", "stop", answerSourceSensitiveFilter)
		endRes.TokenUsage = &proto.TokenUsage{}
		err = stream.Send(startRes)
		if err != nil {
			s.log.Error(err)
			return err
		}
		resList := app.buildChatCompletionStreamResponseList(resId, msg, answerSourceSensitiveFilter)
		for _, res := range resList {
			err = stream.Send(res)
			if err != nil {
				s.log.Error(err)
				return err
			}
		}
		err = stream.Send(endRes)
		if err != nil {
			s.log.Error(err)
			return err
		}
		return nil
	}

	question := in.Message
	if question != "" && s.semanticCache != nil {
		answer, found, err := s.semanticCache.Find(stream.Context(), question)
		if err != nil {
			s.log.Error(err)
		} else if found {
			usage, err := app.savedTokenUsage(in, answer)
			if err != nil {
				s.busMetrics.ErrQuestionsTotalCounter.Inc()
				s.log.Error(err)
				return err
			}
			resId := uuid.New().String()
			startRes := app.buildChatCompletionStreamResponse(resId, "", "", answerSourceVectorDB)
			endRes := app.buildChatCompletionStreamResponse(resId, "", "stop", answerSourceVectorDB)
			endRes.TokenUsage = usage
			err = stream.Send(startRes)
			if err != nil {
				s.log.Error(err)
				return err
			}
			resList := app.buildChatCompletionStreamResponseList(resId, answer, answerSourceVectorDB)
			for _, res := range resList {
				err = stream.Send(res)
				if err != nil {
					s.log.Error(err)
					return err
				}
			}
			err = stream.Send(endRes)
			if err != nil {
				s.log.Error(err)
				return err
			}
			return nil
		}
	}

	client := app.getOpenaiClient()
	req, promptTokens, currTokens, currMessage, err := app.buildChatCompletionRequest(in, true)
	if err != nil {
		s.busMetrics.ErrQuestionsTotalCounter.Inc()
		s.log.Error(err)
		return err
	}
	chatStream, err := client.CreateChatCompletionStream(stream.Context(), req)
	if err != nil {
		s.busMetrics.ErrQuestionsTotalCounter.Inc()
		s.log.Error(err)
		return err
	}
	defer chatStream.Close()
	completionContent := ""
	resultID := ""
	var endRes *proto.ChatCompletionStreamResponse
	for {
		resp, err := chatStream.Recv()
		if err != nil && err != io.EOF {
			s.log.Error(err)
			return err
		}
		if err == io.EOF {
			break
		}
		if len(resp.Choices) == 0 {
			err := fmt.Errorf("chat completion stream returned no choices")
			s.log.Error(err)
			return err
		}
		if resultID == "" {
			resultID = resp.ID
		}
		completionContent += resp.Choices[0].Delta.Content
		res := &proto.ChatCompletionStreamResponse{}
		bytes, err := json.Marshal(resp)
		if err != nil {
			s.log.Error(err)
			return err
		}
		err = jsonpb.UnmarshalString(string(bytes), res)
		if err != nil {
			s.log.Error(err)
			return err
		}
		res.Source = answerSourceLLMAPI
		if res.Choices[0].FinishReason != "" {
			endRes = res
			continue
		}
		err = stream.Send(res)
		if err != nil {
			s.log.Error(err)
			return err
		}
	}
	if endRes == nil {
		err := fmt.Errorf("chat completion stream ended without a finish reason")
		s.log.Error(err)
		return err
	}
	resultMessage := openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleAssistant,
		Content: completionContent,
	}
	model := s.config.Chat.Model
	if in.ChatParam != nil && in.ChatParam.Model != "" {
		model = in.ChatParam.Model
	}
	resultTokens, err := tokenizer.GetTokens(&resultMessage, model)
	if err != nil {
		s.busMetrics.ErrQuestionsTotalCounter.Inc()
		s.log.Error(err)
		return err
	}
	// Send usage with the terminal chunk before the client starts any continuation.
	endRes.TokenUsage = &proto.TokenUsage{ConsumedTokens: int32(promptTokens + resultTokens)}
	if err := stream.Send(endRes); err != nil {
		s.log.Error(err)
		return err
	}

	go func() {
		reqContext := &chat_context.ChatMessage{
			ID:      in.Id,
			PID:     in.Pid,
			Message: currMessage,
			Tokens:  currTokens,
		}
		err := app.saveContext(reqContext)
		if err != nil {
			s.log.Error(err)
			return
		}
		resContext := &chat_context.ChatMessage{
			ID:      resultID,
			PID:     reqContext.ID,
			Message: resultMessage,
			Tokens:  resultTokens,
		}
		err = app.saveContext(resContext)
		if err != nil {
			s.log.Error(err)
			return
		}
	}()
	go func() {
		s.busMetrics.QuestionsTotalCounter.Inc()
		if question != "" && s.semanticCache != nil {
			err := s.semanticCache.Save(context.Background(), question, completionContent)
			if err != nil {
				s.log.Error(err)
			}
		}
	}()
	return nil
}
