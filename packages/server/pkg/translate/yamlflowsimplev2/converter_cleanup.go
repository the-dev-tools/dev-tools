package yamlflowsimplev2

import (
	"fmt"
	"slices"
	"strings"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/compress"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/expression"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/ioworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/varsystem"
)

// cleanupFlowNameSuffix names the hidden flow that holds a flow's cleanup
// nodes. The name is informational only: runners find the flow by ID.
const cleanupFlowNameSuffix = " (cleanup)"

// processCleanup converts a flow's `cleanup:` steps into a FlowCleanup.
//
// The cleanup nodes go into their own hidden flow with no start node, so they
// are never part of the owning flow's graph. depends_on between cleanup steps
// becomes an edge in that flow (for export); the execution order is computed
// here and stored in FlowCleanup.Steps.
func processCleanup(
	flowEntry YamlFlowFlowV2,
	stepNames map[string]*nodeInfo,
	templates map[string]YamlRequestDefV2,
	graphqlTemplates map[string]YamlGraphQLDefV2,
	flowID idwrap.IDWrap,
	opts ConvertOptionsV2,
) (*ioworkspace.FlowCleanup, error) {
	order, err := orderCleanupSteps(flowEntry.Cleanup, stepNames)
	if err != nil {
		return nil, err
	}

	cleanupFlowID := idwrap.NewNow()
	sub := &ioworkspace.WorkspaceBundle{
		Flows: []mflow.Flow{{
			ID:          cleanupFlowID,
			WorkspaceID: opts.WorkspaceID,
			Name:        flowEntry.Name + cleanupFlowNameSuffix,
		}},
	}

	// Cleanup steps get no file tree entries, and their bodies are never
	// compressed: nothing but a runner ever reads them.
	subOpts := opts
	subOpts.GenerateFiles = false
	subOpts.EnableCompression = false

	// Every name a cleanup template may read: normal steps and cleanup steps.
	allNames := make([]string, 0, len(stepNames)+len(order))
	for name := range stepNames {
		allNames = append(allNames, name)
	}

	nodeIDs := make(map[string]idwrap.IDWrap, len(order))
	for _, idx := range order {
		common := getStepCommon(flowEntry.Cleanup[idx])
		nodeIDs[common.Name] = idwrap.NewNow()
		allNames = append(allNames, common.Name)
	}
	slices.Sort(allNames)

	cleanup := &ioworkspace.FlowCleanup{
		FlowID:        flowID,
		CleanupFlowID: cleanupFlowID,
		Steps:         make([]ioworkspace.FlowCleanupStep, 0, len(order)),
		Bundle:        sub,
	}

	for _, idx := range order {
		step := flowEntry.Cleanup[idx]
		common := getStepCommon(step)
		nodeID := nodeIDs[common.Name]

		switch {
		case step.Request != nil:
			httpReq, associated, err := processRequestStep(common.Name, nodeID, cleanupFlowID, step.Request, templates, varsystem.NewVarMap(nil), subOpts)
			if err != nil {
				return nil, err
			}
			sub.HTTPRequests = append(sub.HTTPRequests, *httpReq)
			mergeAssociatedData(sub, associated)
		case step.GraphQL != nil:
			if err := processGraphQLStructStep(step.GraphQL, nodeID, cleanupFlowID, graphqlTemplates, subOpts, sub); err != nil {
				return nil, err
			}
		}

		dependsOn := []string(common.DependsOn)
		for _, dep := range dependsOn {
			sub.FlowEdges = append(sub.FlowEdges, createEdge(nodeIDs[dep], nodeID, cleanupFlowID, mflow.HandleUnspecified))
		}

		templateStrs, exprStrs := cleanupStepStrings(sub, nodeID)
		refs := stepReferences(templateStrs, allNames, common.Name)
		for _, ref := range exprReferences(exprStrs, allNames, common.Name) {
			if !slices.Contains(refs, ref) {
				refs = append(refs, ref)
			}
		}
		slices.Sort(refs)

		cleanup.Steps = append(cleanup.Steps, ioworkspace.FlowCleanupStep{
			NodeID:     nodeID,
			Name:       common.Name,
			DependsOn:  slices.Clone(dependsOn),
			References: refs,
		})
	}

	return cleanup, nil
}

// orderCleanupSteps validates the cleanup steps and returns their indexes in
// execution order: listed order, except that a step runs after the cleanup
// steps it depends on.
func orderCleanupSteps(steps []YamlStepWrapper, stepNames map[string]*nodeInfo) ([]int, error) {
	index := make(map[string]int, len(steps))
	for i, step := range steps {
		common := getStepCommon(step)
		if common == nil {
			return nil, NewYamlFlowErrorV2(fmt.Sprintf("cleanup step %d is empty", i+1), "cleanup", i)
		}
		if common.Name == "" {
			return nil, NewYamlFlowErrorV2(fmt.Sprintf("cleanup step %d is missing a name", i+1), "cleanup", i)
		}
		if step.Request == nil && step.GraphQL == nil {
			return nil, NewYamlFlowErrorV2(fmt.Sprintf("cleanup step '%s': only request and graphql steps are supported in cleanup", common.Name), "cleanup", common.Name)
		}
		if _, ok := stepNames[common.Name]; ok {
			return nil, NewYamlFlowErrorV2(fmt.Sprintf("cleanup step '%s' has the same name as a step in steps; cleanup and normal steps share one variable namespace", common.Name), "cleanup", common.Name)
		}
		if _, dup := index[common.Name]; dup {
			return nil, NewYamlFlowErrorV2(fmt.Sprintf("duplicate cleanup step name '%s'", common.Name), "cleanup", common.Name)
		}
		index[common.Name] = i
	}

	for _, step := range steps {
		common := getStepCommon(step)
		for _, dep := range common.DependsOn {
			if _, ok := index[dep]; !ok {
				return nil, NewYamlFlowErrorV2(fmt.Sprintf("cleanup step '%s' depends on '%s', which is not a cleanup step (cleanup steps always run after every normal step)", common.Name, dep), "depends_on", dep)
			}
		}
	}

	order := make([]int, 0, len(steps))
	placed := make([]bool, len(steps))
	for len(order) < len(steps) {
		progressed := false
		for i, step := range steps {
			if placed[i] || !dependenciesPlaced(getStepCommon(step).DependsOn, index, placed) {
				continue
			}
			placed[i] = true
			order = append(order, i)
			progressed = true
			break
		}
		if !progressed {
			return nil, NewYamlFlowErrorV2("cleanup steps have a dependency cycle", "depends_on", nil)
		}
	}
	return order, nil
}

func dependenciesPlaced(deps []string, index map[string]int, placed []bool) bool {
	for _, dep := range deps {
		if !placed[index[dep]] {
			return false
		}
	}
	return true
}

// cleanupStepStrings collects what a converted cleanup node sends: templated
// strings ({{ }} interpolation) and assertion expressions (bare expr-lang).
func cleanupStepStrings(sub *ioworkspace.WorkspaceBundle, nodeID idwrap.IDWrap) (templates []string, exprs []string) {
	for _, rn := range sub.FlowRequestNodes {
		if rn.FlowNodeID != nodeID || rn.HttpID == nil {
			continue
		}
		httpID := *rn.HttpID
		for _, h := range sub.HTTPRequests {
			if h.ID == httpID {
				templates = append(templates, h.Url)
			}
		}
		for _, h := range sub.HTTPHeaders {
			if h.HttpID == httpID {
				templates = append(templates, h.Key, h.Value)
			}
		}
		for _, p := range sub.HTTPSearchParams {
			if p.HttpID == httpID {
				templates = append(templates, p.Key, p.Value)
			}
		}
		for _, b := range sub.HTTPBodyRaw {
			if b.HttpID == httpID {
				templates = append(templates, rawBodyText(b.RawData, b.CompressionType))
			}
		}
		for _, f := range sub.HTTPBodyForms {
			if f.HttpID == httpID {
				templates = append(templates, f.Key, f.Value)
			}
		}
		for _, u := range sub.HTTPBodyUrlencoded {
			if u.HttpID == httpID {
				templates = append(templates, u.Key, u.Value)
			}
		}
		for _, a := range sub.HTTPAsserts {
			if a.HttpID == httpID {
				exprs = append(exprs, a.Value)
			}
		}
	}

	for _, gn := range sub.FlowGraphQLNodes {
		if gn.FlowNodeID != nodeID || gn.GraphQLID == nil {
			continue
		}
		gqlID := *gn.GraphQLID
		for _, g := range sub.GraphQLRequests {
			if g.ID == gqlID {
				templates = append(templates, g.Url, g.Query, g.Variables)
			}
		}
		for _, h := range sub.GraphQLHeaders {
			if h.GraphQLID == gqlID {
				templates = append(templates, h.Key, h.Value)
			}
		}
		for _, a := range sub.GraphQLAsserts {
			if a.GraphQLID == gqlID {
				exprs = append(exprs, a.Value)
			}
		}
	}
	return templates, exprs
}

func rawBodyText(data []byte, compressionType int8) string {
	if compressionType != compress.CompressTypeNone {
		if decompressed, err := compress.Decompress(data, compressionType); err == nil {
			return string(decompressed)
		}
	}
	return string(data)
}

// stepReferences returns the names (from names, excluding self) that the
// {{ }} templates in strs read as a top-level variable, e.g. PostProducts in
// "{{ PostProducts.response.body.id }}". Environment and file references
// ({{ #env:X }}, {{ #file:X }}) are not step outputs and are ignored.
func stepReferences(strs []string, names []string, self string) []string {
	var exprs []string
	for _, s := range strs {
		for _, ref := range expression.ExtractVarRefs(s) {
			if strings.HasPrefix(ref, "#") {
				continue
			}
			exprs = append(exprs, ref)
		}
	}
	return exprReferences(exprs, names, self)
}

// exprReferences returns the names (from names, excluding self) used as a
// top-level identifier in the expr-lang expressions: not after a '.', and not
// inside a string literal.
func exprReferences(exprs []string, names []string, self string) []string {
	known := make(map[string]bool, len(names))
	for _, n := range names {
		if n != self {
			known[n] = true
		}
	}

	seen := make(map[string]bool)
	var refs []string
	for _, e := range exprs {
		for _, ident := range topLevelIdentifiers(e) {
			if known[ident] && !seen[ident] {
				seen[ident] = true
				refs = append(refs, ident)
			}
		}
	}
	slices.Sort(refs)
	return refs
}

func topLevelIdentifiers(e string) []string {
	var idents []string
	var prev byte // last non-space byte outside an identifier
	for i := 0; i < len(e); {
		c := e[i]
		switch {
		case c == '"' || c == '\'' || c == '`':
			// Skip the string literal.
			j := i + 1
			for j < len(e) && e[j] != c {
				if e[j] == '\\' {
					j++
				}
				j++
			}
			i = j + 1
			prev = c
		case isIdentStartByte(c):
			start := i
			for i < len(e) && isIdentByte(e[i]) {
				i++
			}
			if prev != '.' {
				idents = append(idents, e[start:i])
			}
			prev = 'a'
		default:
			if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
				prev = c
			}
			i++
		}
	}
	return idents
}

func isIdentStartByte(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_'
}

func isIdentByte(c byte) bool {
	return isIdentStartByte(c) || (c >= '0' && c <= '9')
}
