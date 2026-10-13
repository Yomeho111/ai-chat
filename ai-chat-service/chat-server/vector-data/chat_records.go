package vector_data

import (
	"ai-chat-service/proto"
	"context"
	"errors"
	"time"
)

type IChatRecordsData interface {
	Save(ctx context.Context, keywords string, answer string) error
	Find(ctx context.Context, keywords string) (answer string, found bool, err error)
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

func (data *chatRecordsData) Save(ctx context.Context, keywords string, answer string) error {
	ctx, cancel := context.WithTimeout(ctx, data.timeout)
	defer cancel()

	vectorResponse, err := data.client.Vset(ctx, &proto.VsetRequest{Text: keywords})
	if err != nil {
		return err
	}
	if vectorResponse.Uuid == "" {
		return errors.New("vector database returned an empty UUID")
	}

	response, err := data.client.Set(ctx, &proto.SetRequest{
		Uuid: vectorResponse.Uuid,
		Text: answer,
	})
	if err != nil {
		return err
	}
	if !response.Ok {
		return errors.New("kvstore rejected the answer")
	}
	return nil
}

func (data *chatRecordsData) Find(ctx context.Context, keywords string) (string, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, data.timeout)
	defer cancel()

	searchResponse, err := data.client.Vsearch(ctx, &proto.VsearchRequest{
		Topk:      1,
		Threshold: data.threshold,
		Text:      keywords,
	})
	if err != nil {
		return "", false, err
	}
	if len(searchResponse.Uuids) == 0 {
		return "", false, nil
	}

	response, err := data.client.Get(ctx, &proto.GetRequest{Uuid: searchResponse.Uuids[0]})
	if err != nil {
		return "", false, err
	}
	return response.Text, true, nil
}
