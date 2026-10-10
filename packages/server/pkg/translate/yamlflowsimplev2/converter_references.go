package yamlflowsimplev2

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/expression"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

// Step reference validation.
//
// A step that reads another step's output ({{ Login.response.body.token }}, or
// Login.response.status in an assertion or condition) must run after that step. Without a
// depends_on path to it, the step is wired to the flow start, runs in parallel with the step
// it reads, sees the reference unresolved and the flow can still pass. The converter rejects
// such a flow and names the missing depends_on rather than inferring the edge: depends_on is
// the flow's explicit ordering contract (it decides parallelism, and with an explicit
// manual_start a step without it intentionally never runs), so adding edges silently would
// change what runs, in what order, and the graph the flow exports back to.

// referenceSkipFields are YAML fields holding step names, template names or code rather than
// values that are interpolated at run time. break_condition is collected on its own: it is
// evaluated after each iteration, so it may read the loop body.
var referenceSkipFields = map[string]bool{
	"depends_on": true, "provider": true, "memory": true, "tools": true, "then": true, "else": true,
	"loop": true, "code": true, "use_request": true, "ws_connection_node_name": true, "flow": true,
	"credential": true, "model": true, "custom_model": true, "position_x": true, "position_y": true,
	"break_condition": true,
}

// referenceExprFields are YAML fields evaluated as a whole expr-lang expression, where a
// bare Login.response.status reads another step.
var referenceExprFields = map[string]bool{
	"condition": true, "iter_count": true, "items": true, "expression": true,
}

// validateStepReferences rejects a step that reads a step it doesn't (transitively) depend
// on. Steps that can't run (not reachable from the start node) are not checked.
func validateStepReferences(
	steps []YamlStepWrapper,
	templates map[string]YamlRequestDefV2,
	graphqlTemplates map[string]YamlGraphQLDefV2,
	variables []YamlFlowVariableV2,
	nodeList []*nodeInfo,
	startNodeID idwrap.IDWrap,
	edges []mflow.Edge,
) error {
	stepIDs := make(map[string]idwrap.IDWrap, len(nodeList))
	for _, n := range nodeList {
		stepIDs[n.name] = n.id
	}
	for _, v := range variables {
		// A flow variable shadows a step of the same name; don't guess which one is meant.
		delete(stepIDs, v.Name)
	}

	forward := make(map[idwrap.IDWrap][]idwrap.IDWrap)
	backward := make(map[idwrap.IDWrap][]idwrap.IDWrap)
	for _, e := range edges {
		forward[e.SourceID] = append(forward[e.SourceID], e.TargetID)
		backward[e.TargetID] = append(backward[e.TargetID], e.SourceID)
	}
	reachable := walkGraph(startNodeID, forward)

	for _, n := range nodeList {
		if _, runs := reachable[n.id]; !runs || n.id == startNodeID {
			continue
		}
		refs, afterBody := stepReadPaths(steps[n.index], templates, graphqlTemplates)
		if len(refs) == 0 && len(afterBody) == 0 {
			continue
		}
		ancestors := walkGraph(n.id, backward)
		var body map[idwrap.IDWrap]struct{}
		if len(afterBody) > 0 {
			body = walkGraph(n.id, forward)
		}
		for i, ref := range append(refs, afterBody...) {
			target := referencedStep(ref, stepIDs)
			if target == "" || target == n.name {
				continue
			}
			if _, ok := ancestors[stepIDs[target]]; ok {
				continue
			}
			if _, ok := body[stepIDs[target]]; ok && i >= len(refs) {
				continue
			}
			return NewYamlFlowErrorV2(
				fmt.Sprintf("step '%s' uses '%s' but does not depend on step '%s', so it would run before '%s' has a result; add \"depends_on: %s\" (or depend on a step that runs after it)",
					n.name, ref, target, target, target),
				"depends_on", n.name)
		}
	}
	return nil
}

// walkGraph returns every node reachable from start along adj, start included.
func walkGraph(start idwrap.IDWrap, adj map[idwrap.IDWrap][]idwrap.IDWrap) map[idwrap.IDWrap]struct{} {
	seen := map[idwrap.IDWrap]struct{}{start: {}}
	queue := []idwrap.IDWrap{start}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, next := range adj[cur] {
			if _, ok := seen[next]; !ok {
				seen[next] = struct{}{}
				queue = append(queue, next)
			}
		}
	}
	return seen
}

// referencedStep returns the step a reference reads, or "" when it reads no step.
func referencedStep(ref string, stepIDs map[string]idwrap.IDWrap) string {
	head := ref
	if i := strings.IndexAny(head, ".["); i >= 0 {
		head = head[:i]
	}
	if _, ok := stepIDs[head]; ok {
		return head
	}
	return ""
}

// stepReadPaths lists the variable paths a step reads at run time, including those in the
// request or GraphQL template it uses, in the order they appear. afterBody are the paths a
// loop's break_condition reads, which may also be steps of the loop body.
func stepReadPaths(sw YamlStepWrapper, templates map[string]YamlRequestDefV2, graphqlTemplates map[string]YamlGraphQLDefV2) (refs, afterBody []string) {
	c := &referenceCollector{seen: make(map[string]struct{})}
	v := reflect.ValueOf(sw)
	for i := range v.NumField() {
		if f := v.Field(i); !f.IsNil() {
			c.walk(f.Elem(), false)
		}
	}
	if sw.Request != nil && sw.Request.UseRequest != "" {
		if tmpl, ok := templates[sw.Request.UseRequest]; ok {
			c.walk(reflect.ValueOf(tmpl), false)
		}
	}
	if sw.GraphQL != nil && sw.GraphQL.UseRequest != "" {
		if tmpl, ok := graphqlTemplates[sw.GraphQL.UseRequest]; ok {
			c.walk(reflect.ValueOf(tmpl), false)
		}
	}

	brk := &referenceCollector{seen: make(map[string]struct{})}
	if sw.For != nil {
		brk.collectString(sw.For.BreakCondition, true)
	}
	if sw.ForEach != nil {
		brk.collectString(sw.ForEach.BreakCondition, true)
	}
	return c.refs, brk.refs
}

type referenceCollector struct {
	refs []string
	seen map[string]struct{}
}

func (c *referenceCollector) add(path string) {
	if _, ok := c.seen[path]; ok {
		return
	}
	c.seen[path] = struct{}{}
	c.refs = append(c.refs, path)
}

func (c *referenceCollector) walk(v reflect.Value, isExpr bool) {
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		if !v.IsNil() {
			c.walk(v.Elem(), isExpr)
		}
	case reflect.Struct:
		t := v.Type()
		for i := range t.NumField() {
			field := t.Field(i)
			if field.Type == reflect.TypeFor[YamlStepCommon]() {
				continue // name, depends_on, position
			}
			tag, _, _ := strings.Cut(field.Tag.Get("yaml"), ",")
			if referenceSkipFields[tag] {
				continue
			}
			c.walk(v.Field(i), referenceExprFields[tag])
		}
	case reflect.Slice, reflect.Array:
		for i := range v.Len() {
			c.walk(v.Index(i), isExpr)
		}
	case reflect.Map:
		iter := v.MapRange()
		for iter.Next() {
			c.walk(iter.Value(), isExpr)
		}
	case reflect.String:
		c.collectString(v.String(), isExpr)
	}
}

func (c *referenceCollector) collectString(s string, isExpr bool) {
	if s == "" {
		return
	}
	for _, ref := range expression.ExtractVarRefs(s) {
		c.collectExpr(ref)
	}
	if isExpr && !expression.HasVars(s) {
		c.collectExpr(s)
	}
}

func (c *referenceCollector) collectExpr(exprStr string) {
	paths := expression.ExtractExprPaths(exprStr)
	if len(paths) == 0 {
		// Not parseable as an expression (e.g. a step name with a hyphen); keep the raw
		// reference so its leading name can still be matched.
		c.add(strings.TrimSpace(exprStr))
		return
	}
	sort.SliceStable(paths, func(i, j int) bool {
		return strings.Index(exprStr, paths[i]) < strings.Index(exprStr, paths[j])
	})
	for _, p := range paths {
		c.add(p)
	}
}
