package vector_data

import (
	"ai-chat-service/proto"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const chatRecordPrefix = "ai-chat-cache:v1\n"

type ChatRecord struct {
	UUID           string `json:"-"`
	CacheScope     string `json:"cache_scope,omitempty"`
	Answer         string `json:"answer"`
	ConsumedTokens int32  `json:"consumed_tokens"`
}

type IChatRecordsData interface {
	Save(ctx context.Context, question string, record ChatRecord) error
	Find(ctx context.Context, question string) (record ChatRecord, found bool, err error)
}

type chatRecordsData struct {
	client    proto.RagClient
	timeout   time.Duration
	threshold float32
}

func NewChatRecordsData(client proto.RagClient, timeout time.Duration, threshold float32) IChatRecordsData {
	return &chatRecordsData{
		client:    client,
		timeout:   timeout,
		threshold: threshold,
	}
}

func (data *chatRecordsData) Save(ctx context.Context, question string, record ChatRecord) error {
	if record.ConsumedTokens <= 0 {
		return errors.New("semantic cache record must include a positive generation token count")
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, data.timeout)
	defer cancel()

	recordUUID := record.UUID
	if recordUUID == "" {
		vectorResponse, err := data.client.Vset(ctx, &proto.VsetRequest{Text: question})
		if err != nil {
			return err
		}
		if vectorResponse.Uuid == "" {
			return errors.New("vector database returned an empty UUID")
		}
		recordUUID = vectorResponse.Uuid
	}

	response, err := data.client.Set(ctx, &proto.SetRequest{
		Uuid: recordUUID,
		Text: chatRecordPrefix + string(encoded),
	})
	if err != nil {
		return err
	}
	if !response.Ok {
		return errors.New("kvstore rejected the answer")
	}
	return nil
}

func (data *chatRecordsData) Find(ctx context.Context, question string) (ChatRecord, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, data.timeout)
	defer cancel()

	searchResponse, err := data.client.Vsearch(ctx, &proto.VsearchRequest{
		Topk:      1,
		Threshold: data.threshold,
		Text:      question,
	})
	if err != nil {
		return ChatRecord{}, false, err
	}
	if len(searchResponse.Uuids) == 0 {
		return ChatRecord{}, false, nil
	}

	// Preserve the UUID so regeneration replaces legacy or corrupt records in place.
	existing := ChatRecord{UUID: searchResponse.Uuids[0]}
	response, err := data.client.Get(ctx, &proto.GetRequest{Uuid: existing.UUID})
	if err != nil {
		return existing, false, err
	}
	if !strings.HasPrefix(response.Text, chatRecordPrefix) {
		return existing, false, errors.New("semantic cache entry has no generation token usage; regenerating the answer")
	}
	record := existing
	if err := json.Unmarshal([]byte(strings.TrimPrefix(response.Text, chatRecordPrefix)), &record); err != nil {
		return existing, false, fmt.Errorf("decode semantic cache record: %w", err)
	}
	if record.ConsumedTokens <= 0 {
		return existing, false, errors.New("semantic cache record has an invalid generation token count")
	}
	return record, true, nil
}
