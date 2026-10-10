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
)

// aiChecksConverter collects a file's AI checks and stream settings while its flows convert.
type aiChecksConverter struct {
	checks  *mexpect.Checks
	streams map[idwrap.IDWrap]httpclient.StreamOptions
	baseDir string
}

// newAIChecksConverter decodes and validates the file-level judge: and quality:.
func newAIChecksConverter(yf *YamlFlowFormatV2, baseDir string) (*aiChecksConverter, error) {
	c := &aiChecksConverter{
		checks:  &mexpect.Checks{Flows: map[idwrap.IDWrap]mexpect.FlowSettings{}, Steps: map[idwrap.IDWrap]mexpect.Expect{}},
		streams: map[idwrap.IDWrap]httpclient.StreamOptions{},
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

	nodeIDs := map[string]idwrap.IDWrap{}
	for _, n := range flowData.FlowNodes {
		nodeIDs[n.Name] = n.ID
	}
	for _, cleanup := range flowData.FlowCleanups {
		for _, s := range cleanup.Steps {
			nodeIDs[s.Name] = s.NodeID
		}
	}

	for _, list := range [][]YamlStepWrapper{entry.Steps, entry.Cleanup} {
		for _, sw := range list {
			if err := c.addStep(entry.Name, sw, nodeIDs); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *aiChecksConverter) addStep(flowName string, sw YamlStepWrapper, nodeIDs map[string]idwrap.IDWrap) error {
	common := getStepCommon(sw)
	if common == nil {
		return nil
	}
	nodeID, known := nodeIDs[common.Name]
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
			c.checks.Steps[nodeID] = e
		}
	}
	if r := sw.Request; r != nil && (r.Stream != "" || r.StreamTimeoutMS != nil) {
		opts, err := streamOptions(r)
		if err != nil {
			return fmt.Errorf("flow %s, step %s: %w", flowName, common.Name, err)
		}
		if known {
			c.streams[nodeID] = opts
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
		result.RequestStreams = c.streams
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

// exportStepAIChecks writes a step's expect: and stream: back into its YAML.
func exportStepAIChecks(expects map[idwrap.IDWrap]mexpect.Expect, streams map[idwrap.IDWrap]httpclient.StreamOptions, nodeID idwrap.IDWrap, sw *YamlStepWrapper) error {
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
	if s, ok := streams[nodeID]; ok && sw.Request != nil {
		sw.Request.Stream = s.Preset
		if s.Timeout > 0 {
			ms := s.Timeout.Milliseconds()
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
