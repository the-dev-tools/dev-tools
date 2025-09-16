package cachettl

import (
	"strconv"
	"testing"
	"time"
)

func BenchmarkCacheSet(b *testing.B) {
	cache := New[string, int](time.Minute, 0)
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		cache.SetWithTTL(strconv.Itoa(i), i, time.Minute)
	}
}

func BenchmarkCacheSetGetParallel(b *testing.B) {
	cache := New[string, int](time.Minute, 0)
	for i := 0; i < 1024; i++ {
		cache.SetWithTTL(strconv.Itoa(i), i, time.Minute)
	}

	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := strconv.Itoa(i & 1023)
			cache.SetWithTTL(key, i, time.Minute)
			cache.Get(key)
			i++
		}
	})
}
