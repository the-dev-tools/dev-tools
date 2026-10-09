package loadmetrics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCombineIsLossless proves a combined frame is interchangeable with the
// frames it came from: merging it reports exactly what merging them does, per
// step, histogram included.
func TestCombineIsLossless(t *testing.T) {
	login := Key{Step: "PostLogin", StatusClass: StatusClass2xx}
	failed := Key{Step: "PostLogin", StatusClass: StatusClass5xx}
	search := Key{Step: "Search", StatusClass: StatusClass2xx}

	start := time.Unix(1_700_000_000, 0)
	a := NewAggregator(time.Second)
	b := NewAggregator(time.Second)
	a.Flush(start)
	b.Flush(start)
	for ms := 1; ms <= 200; ms++ {
		a.Record(login, time.Duration(ms)*time.Millisecond, 10, false)
		b.Record(search, time.Duration(ms*2)*time.Millisecond, 5, false)
	}
	b.Record(failed, time.Second, 0, true)

	fa := a.Flush(start.Add(5 * time.Second))
	fb := b.Flush(start.Add(5 * time.Second))

	combined := Combine([]Frame{fa, fb})
	require.Len(t, combined.Entries, 3)
	assert.Equal(t, start, combined.IntervalStart)
	assert.Equal(t, 5*time.Second, combined.Interval)

	want := Merge([]Frame{fa, fb})
	got := Merge([]Frame{combined})
	assert.Equal(t, want, got)

	// Every entry carries its own histogram, so a step's percentiles survive
	// a second, cross-machine combine.
	for key, entry := range combined.Entries {
		require.NotNil(t, entry.Hist, "entry %v has no histogram", key)
		assert.Equal(t, entry.Count, entry.Hist.TotalCount(), "entry %v", key)
	}
}

// TestCombineDoesNotAliasInputs guards the shared-histogram trap: combining
// must not mutate the histograms of the frames passed in.
func TestCombineDoesNotAliasInputs(t *testing.T) {
	k := Key{Step: "s", StatusClass: StatusClass2xx}
	a := NewAggregator(time.Second)
	a.Record(k, time.Millisecond, 0, false)
	fa := a.Flush(time.Now())

	_ = Combine([]Frame{fa, fa})
	assert.Equal(t, int64(1), fa.Entries[k].Hist.TotalCount())
}

func TestCombineEmpty(t *testing.T) {
	f := Combine(nil)
	assert.Empty(t, f.Entries)
	assert.Equal(t, time.Duration(0), f.Interval)
}

func TestHistogramEncodingRoundTrip(t *testing.T) {
	k := Key{Step: "s", StatusClass: StatusClass2xx}
	a := NewAggregator(time.Second)
	for ms := 1; ms <= 100; ms++ {
		a.Record(k, time.Duration(ms)*time.Millisecond, 0, false)
	}
	hist := a.Flush(time.Now()).Entries[k].Hist

	encoded, err := EncodeHistogram(hist)
	require.NoError(t, err)
	decoded, err := DecodeHistogram(encoded)
	require.NoError(t, err)

	assert.Equal(t, hist.TotalCount(), decoded.TotalCount())
	assert.Equal(t, hist.ValueAtPercentile(95), decoded.ValueAtPercentile(95))
	assert.Equal(t, hist.Max(), decoded.Max())

	_, err = DecodeHistogram([]byte("not a histogram"))
	assert.Error(t, err)
}
