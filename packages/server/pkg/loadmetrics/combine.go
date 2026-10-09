package loadmetrics

import (
	"fmt"

	hdrhistogram "github.com/HdrHistogram/hdrhistogram-go"
)

// Combine folds any number of Frames - typically one interval's flush from
// each of many concurrent Aggregators - into a single Frame, without
// deriving anything. Unlike Merge it keeps a histogram per (step,
// status-class) entry, so the result can be combined again later (for
// example across machines) with no loss.
//
// The combined frame covers the union of the inputs' time ranges. The
// inputs' histograms are only read; every entry of the result owns a freshly
// allocated one.
func Combine(frames []Frame) Frame {
	out := Frame{Entries: make(map[Key]Entry)}

	for i, f := range frames {
		end := f.IntervalStart.Add(f.Interval)
		if i == 0 {
			out.IntervalStart = f.IntervalStart
			out.Interval = f.Interval
		} else {
			if f.IntervalStart.Before(out.IntervalStart) {
				out.Interval += out.IntervalStart.Sub(f.IntervalStart)
				out.IntervalStart = f.IntervalStart
			}
			if outEnd := out.IntervalStart.Add(out.Interval); end.After(outEnd) {
				out.Interval = end.Sub(out.IntervalStart)
			}
		}

		for k, e := range f.Entries {
			acc, ok := out.Entries[k]
			if !ok {
				acc = Entry{Hist: newHistogram()}
			}
			acc.Count += e.Count
			acc.ErrorCount += e.ErrorCount
			acc.Bytes += e.Bytes
			if e.Hist != nil {
				acc.Hist.Merge(e.Hist)
			}
			out.Entries[k] = acc
		}
	}

	return out
}

// EncodeHistogram serializes a histogram in HdrHistogram's compressed V2
// format, the encoding the LoadMetricEntry wire model carries.
func EncodeHistogram(h *hdrhistogram.Histogram) ([]byte, error) {
	encoded, err := h.Encode(hdrhistogram.V2CompressedEncodingCookieBase)
	if err != nil {
		return nil, fmt.Errorf("loadmetrics: encode histogram: %w", err)
	}
	return encoded, nil
}

// DecodeHistogram is the inverse of EncodeHistogram.
func DecodeHistogram(encoded []byte) (*hdrhistogram.Histogram, error) {
	h, err := hdrhistogram.Decode(encoded)
	if err != nil {
		return nil, fmt.Errorf("loadmetrics: decode histogram: %w", err)
	}
	return h, nil
}
