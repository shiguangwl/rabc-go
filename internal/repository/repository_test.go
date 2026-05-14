package repository

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/spf13/viper"
)

func TestNewRedisUsesConfiguredDB(t *testing.T) {
	mr := miniredis.RunT(t)
	conf := viper.New()
	conf.Set("data.redis.addr", mr.Addr())
	conf.Set("data.redis.db", 3)

	rdb := NewRedis(conf)
	defer func() {
		if err := rdb.Close(); err != nil {
			t.Fatalf("close redis client: %v", err)
		}
	}()

	if err := rdb.Set(context.Background(), "selected-db", "ok", 0).Err(); err != nil {
		t.Fatalf("set selected-db: %v", err)
	}
	if !mr.DB(3).Exists("selected-db") {
		t.Fatal("selected-db should be written to Redis DB 3")
	}
	if mr.DB(0).Exists("selected-db") {
		t.Fatal("selected-db should not be written to Redis DB 0")
	}
}
