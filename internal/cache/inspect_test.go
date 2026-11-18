package cache

import (
	"context"
	"testing"
)

func TestDescribeReadsWhatAKeyIs(t *testing.T) {
	c := &Cache{name: CacheDatabase}
	generations := map[string]int64{Languages: 2}

	tests := []struct {
		name string
		key  string
		want Key
	}{
		{
			name: "a value of the current generation",
			key:  "cache:languages:v2:text:ru:id",
			want: Key{Kind: KindEntry, Group: Languages, Entry: "text:ru:id"},
		},
		{
			name: "a value of an older generation",
			key:  "cache:languages:v1:all",
			want: Key{Kind: KindStale, Group: Languages, Entry: "all"},
		},
		{
			name: "a group never forgotten is at generation zero",
			key:  "cache:organization:v0:settings",
			want: Key{Kind: KindEntry, Group: Organization, Entry: "settings"},
		},
		{
			name: "a generation counter",
			key:  "cache:languages:generation",
			want: Key{Kind: KindGeneration, Group: Languages},
		},
		{
			name: "a rate limit for an IPv6 address",
			key:  "ratelimit:public:2001:db8::1",
			want: Key{Kind: KindRateLimit, Group: "public", Entry: "2001:db8::1"},
		},
		{
			name: "something this server does not write",
			key:  "elsewhere",
			want: Key{Kind: KindOther},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c.describe(tt.key, generations)
			tt.want.Name, tt.want.TTL = tt.key, -1
			if got != tt.want {
				t.Errorf("describe(%q) = %+v, want %+v", tt.key, got, tt.want)
			}
		})
	}
}

// Keys lists what a group holds, with its time to live, and Flush empties
// the cache database while leaving the session database alone: what the
// tests that read Redis behind the server's back rely on.
func TestLiveKeysAndFlush(t *testing.T) {
	r := liveRedis(t)
	ctx := context.Background()
	c := r.Cache

	c.Set(ctx, Languages, "all", []string{"en"})
	r.Sessions.Set(ctx, Admins, "ada", map[string]string{"username": "ada"})

	keys, _, err := c.Keys(ctx, KeyQuery{Kind: KindEntry, Group: Languages})
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 || keys[0].Entry != "all" || keys[0].TTL <= 0 {
		t.Fatalf("keys = %+v, want the languages' one value with its time to live", keys)
	}

	held, _, err := r.Sessions.Keys(ctx, KeyQuery{Kind: KindEntry, Group: Admins})
	if err != nil || len(held) != 1 {
		t.Fatalf("session database keys = %+v, %v; want the one administrator", held, err)
	}

	if err := c.Flush(ctx); err != nil {
		t.Fatal(err)
	}
	var all []string
	if c.Get(ctx, Languages, "all", &all) {
		t.Error("a flushed cache still answered")
	}
	var admin map[string]string
	if !r.Sessions.Get(ctx, Admins, "ada", &admin) {
		t.Error("flushing the cache database removed what the session database holds")
	}
}
