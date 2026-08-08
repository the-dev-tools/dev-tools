using DevTools;

namespace Api.LoadMetrics;

@doc("Outcome bucket for one recorded load-test request. Mirrors packages/server/pkg/loadmetrics.StatusClass.")
enum LoadStatusClass {
  TwoXx,
  ThreeXx,
  FourXx,
  FiveXx,
  Error,
  Timeout,
}

@doc("One (step, status-class) bucket's aggregated stats for a single reporting interval.")
model LoadMetricEntry {
  @doc("Name of the load-test step this bucket belongs to") step: string;
  statusClass: LoadStatusClass;
  count: int64;
  errorCount: int64;
  bytes: int64;

  @doc("HDR histogram of latencies for this bucket, compressed-encoded (see hdrhistogram-go Histogram.Encode)")
  hdrHistogram: bytes;

  @doc("50th percentile latency, in microseconds") p50Us: int64;
  @doc("90th percentile latency, in microseconds") p90Us: int64;
  @doc("95th percentile latency, in microseconds") p95Us: int64;
  @doc("99th percentile latency, in microseconds") p99Us: int64;
  @doc("Maximum observed latency, in microseconds") maxUs: int64;
}

@doc("One flushed interval's worth of aggregated load-test entries.")
model LoadMetricFrame {
  intervalStart: Protobuf.WellKnown.Timestamp;
  @doc("Duration of the interval covered by this frame, in milliseconds") intervalMs: int64;
  entries: LoadMetricEntry[];
}

@doc("Derived, percentile-level statistics - either the whole run's total, or one step/status-class breakdown.")
model LoadStats {
  count: int64;
  errorCount: int64;
  bytes: int64;
  p50Us: int64;
  p90Us: int64;
  p95Us: int64;
  p99Us: int64;
  maxUs: int64;
  @doc("Requests per second, i.e. count divided by covered wall time") rps: float32;
}

model LoadRunStepStats {
  step: string;
  statusClass: LoadStatusClass;
  ...LoadStats;
}

@doc("Fully-merged report for a completed (or in-progress) load-test run.")
model LoadRunReport {
  total: LoadStats;
  perStep: LoadRunStepStats[];
}

model LoadFailureHeader {
  key: string;
  value: string;
}

model LoadFailureRequest {
  method: string;
  url: string;
  headers: LoadFailureHeader[];
  @doc("Truncated preview of the request body, not the full payload") bodySample: string;
}

model LoadFailureResponse {
  status: int32;
  headers: LoadFailureHeader[];
  @doc("Truncated preview of the response body, not the full payload") bodySample: string;
}

@doc("Captured detail for one failed load-test request, for post-run debugging.")
model LoadFailureArtifact {
  step: string;
  @doc("Virtual user index that produced this failure") vu: int32;
  @doc("Iteration number within the virtual user's loop") iteration: int32;
  request: LoadFailureRequest;

  @doc("Absent when the failure occurred before a response was received (e.g. transport error, timeout)")
  response?: LoadFailureResponse;

  @doc("Variable values resolved into the request at the time it was built") resolvedVariables: Protobuf.Map<
    string,
    string
  >;
  error: string;
  capturedAt: Protobuf.WellKnown.Timestamp;
}
