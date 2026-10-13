package vector_data

import (
	"ai-chat-service/proto"
	"context"
	"testing"
	"time"

	"google.golang.org/grpc"
)

type ragClientStub struct {
	vset    func(context.Context, *proto.VsetRequest) (*proto.VsetResponse, error)
	vsearch func(context.Context, *proto.VsearchRequest) (*proto.VsearchResponse, error)
	set     func(context.Context, *proto.SetRequest) (*proto.SetResponse, error)
	get     func(context.Context, *proto.GetRequest) (*proto.GetResponse, error)
}

func (client *ragClientStub) Vset(ctx context.Context, request *proto.VsetRequest, _ ...grpc.CallOption) (*proto.VsetResponse, error) {
	return client.vset(ctx, request)
}

func (client *ragClientStub) Vsearch(ctx context.Context, request *proto.VsearchRequest, _ ...grpc.CallOption) (*proto.VsearchResponse, error) {
	return client.vsearch(ctx, request)
}

func (client *ragClientStub) Set(ctx context.Context, request *proto.SetRequest, _ ...grpc.CallOption) (*proto.SetResponse, error) {
	return client.set(ctx, request)
}

func (client *ragClientStub) Get(ctx context.Context, request *proto.GetRequest, _ ...grpc.CallOption) (*proto.GetResponse, error) {
	return client.get(ctx, request)
}

func TestSaveUsesVectorUUIDAsAnswerKey(t *testing.T) {
	client := &ragClientStub{
		vset: func(_ context.Context, request *proto.VsetRequest) (*proto.VsetResponse, error) {
			if request.Text != "go,channel" {
				t.Fatalf("Vset text = %q", request.Text)
			}
			return &proto.VsetResponse{Uuid: "record-uuid"}, nil
		},
		set: func(_ context.Context, request *proto.SetRequest) (*proto.SetResponse, error) {
			if request.Uuid != "record-uuid" || request.Text != "cached answer" {
				t.Fatalf("Set request = %#v", request)
			}
			return &proto.SetResponse{Ok: true}, nil
		},
	}

	cache := NewChatRecordsData(client, time.Second, 0.01)
	if err := cache.Save(context.Background(), "go,channel", "cached answer"); err != nil {
		t.Fatal(err)
	}
}

func TestFindFetchesAnswerForClosestUUID(t *testing.T) {
	const threshold = float32(0.01)
	client := &ragClientStub{
		vsearch: func(_ context.Context, request *proto.VsearchRequest) (*proto.VsearchResponse, error) {
			if request.Text != "go,channel" || request.Topk != 1 || request.Threshold != threshold {
				t.Fatalf("Vsearch request = %#v", request)
			}
			return &proto.VsearchResponse{Uuids: []string{"record-uuid"}}, nil
		},
		get: func(_ context.Context, request *proto.GetRequest) (*proto.GetResponse, error) {
			if request.Uuid != "record-uuid" {
				t.Fatalf("Get UUID = %q", request.Uuid)
			}
			return &proto.GetResponse{Text: "cached answer"}, nil
		},
	}

	cache := NewChatRecordsData(client, time.Second, threshold)
	answer, found, err := cache.Find(context.Background(), "go,channel")
	if err != nil {
		t.Fatal(err)
	}
	if !found || answer != "cached answer" {
		t.Fatalf("Find() = %q, %v", answer, found)
	}
}

func TestFindDoesNotReadKVStoreOnMiss(t *testing.T) {
	client := &ragClientStub{
		vsearch: func(context.Context, *proto.VsearchRequest) (*proto.VsearchResponse, error) {
			return &proto.VsearchResponse{}, nil
		},
		get: func(context.Context, *proto.GetRequest) (*proto.GetResponse, error) {
			t.Fatal("Get called for an empty vector search result")
			return nil, nil
		},
	}

	cache := NewChatRecordsData(client, time.Second, 0.01)
	answer, found, err := cache.Find(context.Background(), "missing")
	if err != nil {
		t.Fatal(err)
	}
	if found || answer != "" {
		t.Fatalf("Find() = %q, %v", answer, found)
	}
}
