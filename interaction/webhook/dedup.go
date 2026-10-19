package webhook

import (
	"time"

	"github.com/jellydator/ttlcache/v3"
)

// TTLDeduper 基于 ttlcache 的事件去重实现。
type TTLDeduper struct {
	cache *ttlcache.Cache[string, struct{}]
	ttl   time.Duration
}

// NewTTLDeduper 创建 TTL 去重器。ttl <= 0 时默认 10 分钟，capacity 为 0 时默认 100000。
func NewTTLDeduper(ttl time.Duration, capacity uint64) *TTLDeduper {
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	if capacity == 0 {
		capacity = 100000
	}
	cache := ttlcache.New[string, struct{}](
		ttlcache.WithTTL[string, struct{}](ttl),
		ttlcache.WithCapacity[string, struct{}](capacity),
	)
	go cache.Start()
	return &TTLDeduper{cache: cache, ttl: ttl}
}

func (d *TTLDeduper) Seen(eventID string) bool {
	if d == nil || d.cache == nil || eventID == "" {
		return false
	}
	return d.cache.Has(eventID)
}

func (d *TTLDeduper) Mark(eventID string) {
	if d == nil || d.cache == nil || eventID == "" {
		return
	}
	d.cache.Set(eventID, struct{}{}, d.ttl)
}

func (d *TTLDeduper) Unmark(eventID string) {
	if d == nil || d.cache == nil || eventID == "" {
		return
	}
	d.cache.Delete(eventID)
}

// Stop 停止后台清理协程。
func (d *TTLDeduper) Stop() {
	if d != nil && d.cache != nil {
		d.cache.Stop()
	}
}
