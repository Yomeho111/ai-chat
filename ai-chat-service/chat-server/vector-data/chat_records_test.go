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
	const question = "How do Go channels work?"
	client := &ragClientStub{
		vset: func(_ context.Context, request *proto.VsetRequest) (*proto.VsetResponse, error) {
			if request.Text != question {
				t.Fatalf("Vset text = %q", request.Text)
			}
			return &proto.VsetResponse{Uuid: "record-uuid"}, nil
		},
		set: func(_ context.Context, request *proto.SetRequest) (*proto.SetResponse, error) {
			if request.Uuid != "record-uuid" || request.Text != "ai-chat-cache:v1\n{\"answer\":\"cached answer\",\"consumed_tokens\":89}" {
				t.Fatalf("Set request = %#v", request)
			}
			return &proto.SetResponse{Ok: true}, nil
		},
	}

	cache := NewChatRecordsData(client, time.Second, 0.01)
	if err := cache.Save(context.Background(), question, ChatRecord{Answer: "cached answer", ConsumedTokens: 89}); err != nil {
		t.Fatal(err)
	}
}

func TestFindFetchesAnswerForClosestUUID(t *testing.T) {
	const threshold = float32(0.01)
	const question = "How do Go channels work?"
	client := &ragClientStub{
		vsearch: func(_ context.Context, request *proto.VsearchRequest) (*proto.VsearchResponse, error) {
			if request.Text != question || request.Topk != 1 || request.Threshold != threshold {
				t.Fatalf("Vsearch request = %#v", request)
			}
			return &proto.VsearchResponse{Uuids: []string{"record-uuid"}}, nil
		},
		get: func(_ context.Context, request *proto.GetRequest) (*proto.GetResponse, error) {
			if request.Uuid != "record-uuid" {
				t.Fatalf("Get UUID = %q", request.Uuid)
			}
			return &proto.GetResponse{Text: "ai-chat-cache:v1\n{\"answer\":\"cached answer\",\"consumed_tokens\":89}"}, nil
		},
	}

	cache := NewChatRecordsData(client, time.Second, threshold)
	record, found, err := cache.Find(context.Background(), question)
	if err != nil {
		t.Fatal(err)
	}
	if !found || record != (ChatRecord{UUID: "record-uuid", Answer: "cached answer", ConsumedTokens: 89}) {
		t.Fatalf("Find() = %#v, %v", record, found)
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
	record, found, err := cache.Find(context.Background(), "missing")
	if err != nil {
		t.Fatal(err)
	}
	if found || record != (ChatRecord{}) {
		t.Fatalf("Find() = %#v, %v", record, found)
	}
}

func TestGenerationTokenUsageSurvivesCacheRoundTrip(t *testing.T) {
	want := ChatRecord{Answer: "answer with\nnewlines, \"quotes\", and \u4e2d\u6587", ConsumedTokens: 89}
	var stored string
	client := &ragClientStub{
		vset: func(context.Context, *proto.VsetRequest) (*proto.VsetResponse, error) {
			return &proto.VsetResponse{Uuid: "record-uuid"}, nil
		},
		set: func(_ context.Context, request *proto.SetRequest) (*proto.SetResponse, error) {
			stored = request.Text
			return &proto.SetResponse{Ok: true}, nil
		},
		vsearch: func(context.Context, *proto.VsearchRequest) (*proto.VsearchResponse, error) {
			return &proto.VsearchResponse{Uuids: []string{"record-uuid"}}, nil
		},
		get: func(context.Context, *proto.GetRequest) (*proto.GetResponse, error) {
			return &proto.GetResponse{Text: stored}, nil
		},
	}
	cache := NewChatRecordsData(client, time.Second, 0.01)
	if err := cache.Save(context.Background(), "question", want); err != nil {
		t.Fatal(err)
	}
	want.UUID = "record-uuid"
	for i := 0; i < 2; i++ {
		got, found, err := cache.Find(context.Background(), "question")
		if err != nil || !found || got != want {
			t.Fatalf("Find() = %#v, %v, %v; want %#v", got, found, err, want)
		}
	}
}

func TestSaveRejectsInvalidGenerationTokenUsage(t *testing.T) {
	client := &ragClientStub{
		vset: func(context.Context, *proto.VsetRequest) (*proto.VsetResponse, error) {
			t.Fatal("invalid records must not be written to the vector database")
			return nil, nil
		},
	}
	cache := NewChatRecordsData(client, time.Second, 0.01)
	for _, tokens := range []int32{0, -1} {
		if err := cache.Save(context.Background(), "question", ChatRecord{Answer: "answer", ConsumedTokens: tokens}); err == nil {
			t.Fatalf("Save() accepted generation token count %d", tokens)
		}
	}
}

func TestFindRejectsLegacyAndInvalidRecords(t *testing.T) {
	for _, tt := range []struct {
		name string
		text string
	}{
		{name: "legacy plain answer", text: "cached answer"},
		{name: "legacy JSON answer", text: `{"answer":"cached answer","consumed_tokens":89}`},
		{name: "malformed record", text: "ai-chat-cache:v1\n{"},
		{name: "missing count", text: "ai-chat-cache:v1\n{\"answer\":\"cached answer\"}"},
		{name: "zero count", text: "ai-chat-cache:v1\n{\"answer\":\"cached answer\",\"consumed_tokens\":0}"},
		{name: "negative count", text: "ai-chat-cache:v1\n{\"answer\":\"cached answer\",\"consumed_tokens\":-1}"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := &ragClientStub{
				vsearch: func(context.Context, *proto.VsearchRequest) (*proto.VsearchResponse, error) {
					return &proto.VsearchResponse{Uuids: []string{"record-uuid"}}, nil
				},
				get: func(context.Context, *proto.GetRequest) (*proto.GetResponse, error) {
					return &proto.GetResponse{Text: tt.text}, nil
				},
			}
			cache := NewChatRecordsData(client, time.Second, 0.01)
			record, found, err := cache.Find(context.Background(), "question")
			if err == nil || found || record != (ChatRecord{UUID: "record-uuid"}) {
				t.Fatalf("Find() = %#v, %v, %v; want cache miss with explicit error", record, found, err)
			}
		})
	}
}

func TestRegeneratedLegacyRecordReusesVectorUUID(t *testing.T) {
	stored := "legacy answer without token usage"
	client := &ragClientStub{
		vset: func(context.Context, *proto.VsetRequest) (*proto.VsetResponse, error) {
			t.Fatal("regeneration must update the original UUID, not create a duplicate vector")
			return nil, nil
		},
		vsearch: func(context.Context, *proto.VsearchRequest) (*proto.VsearchResponse, error) {
			return &proto.VsearchResponse{Uuids: []string{"legacy-uuid"}}, nil
		},
		get: func(context.Context, *proto.GetRequest) (*proto.GetResponse, error) {
			return &proto.GetResponse{Text: stored}, nil
		},
		set: func(_ context.Context, request *proto.SetRequest) (*proto.SetResponse, error) {
			if request.Uuid != "legacy-uuid" {
				t.Fatalf("Set UUID = %q, want original legacy UUID", request.Uuid)
			}
			stored = request.Text
			return &proto.SetResponse{Ok: true}, nil
		},
	}
	cache := NewChatRecordsData(client, time.Second, 0.01)
	record, found, err := cache.Find(context.Background(), "question")
	if err == nil || found || record.UUID != "legacy-uuid" {
		t.Fatalf("Find legacy record = %#v, %v, %v", record, found, err)
	}
	record.Answer = "regenerated answer"
	record.ConsumedTokens = 89
	if err := cache.Save(context.Background(), "question", record); err != nil {
		t.Fatal(err)
	}
	got, found, err := cache.Find(context.Background(), "question")
	if err != nil || !found || got != record {
		t.Fatalf("Find regenerated record = %#v, %v, %v; want %#v", got, found, err, record)
	}
}
