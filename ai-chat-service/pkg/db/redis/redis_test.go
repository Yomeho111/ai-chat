package redis

import (
	"ai-chat-service/pkg/config"
	"testing"

	redis "github.com/redis/go-redis/v9"
)

func TestClientOptionsAreCompatibleWithKVStore(t *testing.T) {
	cnf := &config.Config{}
	cnf.Redis.Host = "context-kvstore"
	cnf.Redis.Port = 6379

	clientPool := getPool(cnf).(*redisPool)
	client := clientPool.pool.New().(*redis.Client)
	defer client.Close()

	options := client.Options()
	if options.Addr != "context-kvstore:6379" {
		t.Fatalf("Addr = %q", options.Addr)
	}
	if options.Password != "" {
		t.Fatal("Password must be empty")
	}
	if options.Protocol != 2 {
		t.Fatalf("Protocol = %d", options.Protocol)
	}
	if !options.DisableIndentity {
		t.Fatal("client identity commands must be disabled")
	}
}
