package loadrun

import (
	"testing"
	"time"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mload"
)

func TestConfigDescribe(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
		want string
	}{
		{
			name: "constant-vus has its own header fields",
			cfg:  Config{VUs: 4, Duration: time.Minute},
			want: "",
		},
		{
			name: "ramping-vus",
			cfg: Config{
				Executor: mload.ExecutorRampingVUs,
				StartVUs: 0,
				Stages: []mload.Stage{
					{Duration: 30 * time.Second, Target: 20},
					{Duration: time.Minute, Target: 20},
					{Duration: 30 * time.Second, Target: 0},
				},
			},
			want: "VUs 0->20->20->0 over 2m0s",
		},
		{
			name: "constant-arrival-rate with think time",
			cfg: Config{
				Executor: mload.ExecutorConstantArrivalRate, Rate: 12.5, TimeUnit: time.Minute,
				Duration: 5 * time.Minute, PreAllocatedVUs: 2, MaxVUs: 10,
				ThinkTime: mload.ThinkTime{Min: time.Second, Max: 3 * time.Second},
			},
			want: "12.5 iterations/1m0s for 5m0s, VUs 2-10, think time 1s-3s",
		},
		{
			name: "ramping-arrival-rate",
			cfg: Config{
				Executor: mload.ExecutorRampingArrivalRate, StartRate: 0,
				Stages:          []mload.Stage{{Duration: 300 * time.Millisecond, Target: 60}},
				PreAllocatedVUs: 2,
				ThinkTime:       mload.ThinkTime{Min: time.Second, Max: time.Second},
			},
			want: "0->60 iterations/1s over 300ms, VUs 2, think time 1s",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.cfg.Describe(); got != tc.want {
				t.Errorf("Describe() = %q, want %q", got, tc.want)
			}
		})
	}
}
