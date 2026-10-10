package yamlflowsimplev2

import (
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/aicheck"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/httpclient"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/ioworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mexpect"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"
)

// aiChecksConverter collects a file's AI checks and stream settings while its flows convert.
// Cleanup steps' settings go into their cleanup's own bundle, next to their nodes.
type aiChecksConverter struct {
	checks  *mexpect.Checks
	streams []mhttp.HTTPStream
	baseDir string
}

// stepTarget is where a step's node lives: its node and HTTP IDs, and the cleanup bundle
// that holds it (nil for a normal step).
type stepTarget struct {
	nodeID  idwrap.IDWrap
	httpID  *idwrap.IDWrap
	cleanup *ioworkspace.WorkspaceBundle
}

// newAIChecksConverter decodes and validates the file-level judge: and quality:.
func newAIChecksConverter(yf *YamlFlowFormatV2, baseDir string) (*aiChecksConverter, error) {
	c := &aiChecksConverter{
		checks:  &mexpect.Checks{Flows: map[idwrap.IDWrap]mexpect.FlowSettings{}, Steps: map[idwrap.IDWrap]mexpect.Expect{}},
		baseDir: baseDir,
	}
	if aicheck.Present(yf.Judge) {
		j, err := aicheck.DecodeJudge(&yf.Judge)
		if err != nil {
			return nil, fmt.Errorf("file judge: %w", err)
		}
		c.checks.Judge = j
	}
	if aicheck.Present(yf.Quality) {
		q, err := aicheck.DecodeQuality(&yf.Quality)
		if err != nil {
			return nil, fmt.Errorf("file quality: %w", err)
		}
		c.checks.Quality = q
	}
	return c, nil
}

// addFlow decodes a converted flow's settings, its steps' expect: blocks and stream: options.
func (c *aiChecksConverter) addFlow(entry YamlFlowFlowV2, flowData *ioworkspace.WorkspaceBundle) error {
	if len(flowData.Flows) == 0 {
		return nil
	}
	flowID := flowData.Flows[0].ID
	settings := mexpect.FlowSettings{Iterations: entry.Iterations}
	if aicheck.Present(entry.Judge) {
		j, err := aicheck.DecodeJudge(&entry.Judge)
		if err != nil {
			return fmt.Errorf("flow %s judge: %w", entry.Name, err)
		}
		settings.Judge = j
	}
	if aicheck.Present(entry.Quality) {
		q, err := aicheck.DecodeQuality(&entry.Quality)
		if err != nil {
			return fmt.Errorf("flow %s quality: %w", entry.Name, err)
		}
		settings.Quality = q
	}
	// Validates the merged judge and quality and the iterations.
	if _, err := aicheck.NewFlow(entry.Name, c.checks, settings); err != nil {
		return err
	}
	if settings.Iterations != nil || settings.Judge != nil || settings.Quality != nil {
		c.checks.Flows[flowID] = settings
	}

	targets := map[string]stepTarget{}
	addTargets := func(b *ioworkspace.WorkspaceBundle, cleanup *ioworkspace.WorkspaceBundle) {
		httpIDs := map[idwrap.IDWrap]*idwrap.IDWrap{}
		for _, rn := range b.FlowRequestNodes {
			httpIDs[rn.FlowNodeID] = rn.HttpID
		}
		for _, n := range b.FlowNodes {
			targets[n.Name] = stepTarget{nodeID: n.ID, httpID: httpIDs[n.ID], cleanup: cleanup}
		}
	}
	addTargets(flowData, nil)
	for _, cl := range flowData.FlowCleanups {
		if cl.Bundle != nil {
			addTargets(cl.Bundle, cl.Bundle)
		}
	}

	for _, list := range [][]YamlStepWrapper{entry.Steps, entry.Cleanup} {
		for _, sw := range list {
			if err := c.addStep(entry.Name, sw, targets); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *aiChecksConverter) addStep(flowName string, sw YamlStepWrapper, targets map[string]stepTarget) error {
	common := getStepCommon(sw)
	if common == nil {
		return nil
	}
	target, known := targets[common.Name]
	if aicheck.Present(common.Expect) {
		where := fmt.Sprintf("flow %s, step %s: expect:", flowName, common.Name)
		if sw.Request == nil && sw.GraphQL == nil {
			return fmt.Errorf("%s works on request and graphql steps only", where)
		}
		e, err := aicheck.DecodeExpect(&common.Expect)
		if err != nil {
			return fmt.Errorf("%s %w", where, err)
		}
		if _, err := aicheck.CompileSpec(e, c.baseDir); err != nil {
			return fmt.Errorf("%s %w", where, err)
		}
		if known {
			if target.cleanup != nil {
				if target.cleanup.AIChecks == nil {
					target.cleanup.AIChecks = &mexpect.Checks{Steps: map[idwrap.IDWrap]mexpect.Expect{}}
				}
				target.cleanup.AIChecks.Steps[target.nodeID] = e
			} else {
				c.checks.Steps[target.nodeID] = e
			}
		}
	}
	if r := sw.Request; r != nil && (r.Stream != "" || r.StreamTimeoutMS != nil) {
		opts, err := streamOptions(r)
		if err != nil {
			return fmt.Errorf("flow %s, step %s: %w", flowName, common.Name, err)
		}
		if known && target.httpID != nil {
			st := mhttp.HTTPStream{HttpID: *target.httpID, Preset: opts.Preset, TimeoutMs: opts.Timeout.Milliseconds()}
			if target.cleanup != nil {
				target.cleanup.HTTPStreams = append(target.cleanup.HTTPStreams, st)
			} else {
				c.streams = append(c.streams, st)
			}
		}
	}
	return nil
}

func streamOptions(r *YamlStepRequest) (httpclient.StreamOptions, error) {
	opts := httpclient.StreamOptions{Preset: r.Stream}
	if opts.Preset != "" && !httpclient.IsStreamPreset(opts.Preset) {
		return opts, fmt.Errorf("stream must be %s (got %q)", presetList(), opts.Preset)
	}
	if r.StreamTimeoutMS != nil {
		if *r.StreamTimeoutMS <= 0 {
			return opts, fmt.Errorf("stream_timeout_ms must be positive (got %d)", *r.StreamTimeoutMS)
		}
		opts.Timeout = time.Duration(*r.StreamTimeoutMS) * time.Millisecond
	}
	return opts, nil
}

func presetList() string {
	p := httpclient.StreamPresets
	return strings.Join(p[:len(p)-1], ", ") + " or " + p[len(p)-1]
}

// apply stores what was collected in the bundle.
func (c *aiChecksConverter) apply(result *ioworkspace.WorkspaceBundle) {
	if len(c.checks.Flows) == 0 {
		c.checks.Flows = nil
	}
	if len(c.checks.Steps) == 0 {
		c.checks.Steps = nil
	}
	if !c.checks.IsEmpty() {
		result.AIChecks = c.checks
	}
	if len(c.streams) > 0 {
		result.HTTPStreams = c.streams
	}
}

// encodeNode turns a value into a YAML node for export; nil for a nil value.
func encodeNode(v any) (*yaml.Node, error) {
	n := &yaml.Node{}
	if err := n.Encode(v); err != nil {
		return nil, err
	}
	return n, nil
}

// exportStepAIChecks writes a step's expect: and stream: back into its YAML. httpID is the
// request step's HTTP request (nil for other steps).
func exportStepAIChecks(expects map[idwrap.IDWrap]mexpect.Expect, streams map[idwrap.IDWrap]mhttp.HTTPStream, nodeID idwrap.IDWrap, httpID *idwrap.IDWrap, sw *YamlStepWrapper) error {
	if sw.Request == nil && sw.GraphQL == nil {
		return nil
	}
	if e, ok := expects[nodeID]; ok {
		n, err := encodeNode(e)
		if err != nil {
			return err
		}
		if sw.Request != nil {
			sw.Request.Expect = *n
		} else {
			sw.GraphQL.Expect = *n
		}
	}
	if httpID == nil || sw.Request == nil {
		return nil
	}
	if s, ok := streams[*httpID]; ok {
		sw.Request.Stream = s.Preset
		if s.TimeoutMs > 0 {
			ms := s.TimeoutMs
			sw.Request.StreamTimeoutMS = &ms
		}
	}
	return nil
}

// exportFileAIChecks writes the file-level judge: and quality:.
func exportFileAIChecks(data *ioworkspace.WorkspaceBundle, yf *YamlFlowFormatV2) error {
	if data.AIChecks == nil {
		return nil
	}
	return encodeSettings(data.AIChecks.Judge, data.AIChecks.Quality, &yf.Judge, &yf.Quality)
}

// encodeSettings encodes a judge: and a quality: block into their YAML nodes.
func encodeSettings(judge *mexpect.JudgeConfig, quality *mexpect.Quality, judgeNode, qualityNode *yaml.Node) error {
	if judge != nil {
		n, err := encodeNode(judge)
		if err != nil {
			return err
		}
		*judgeNode = *n
	}
	if quality != nil {
		n, err := encodeNode(quality)
		if err != nil {
			return err
		}
		*qualityNode = *n
	}
	return nil
}

// exportFlowAIChecks writes a flow's iterations:, judge: and quality:.
func exportFlowAIChecks(data *ioworkspace.WorkspaceBundle, flowID idwrap.IDWrap, fy *YamlFlowFlowV2) error {
	if data.AIChecks == nil {
		return nil
	}
	s, ok := data.AIChecks.Flows[flowID]
	if !ok {
		return nil
	}
	fy.Iterations = s.Iterations
	return encodeSettings(s.Judge, s.Quality, &fy.Judge, &fy.Quality)
}
