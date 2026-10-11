package yamlflowsimplev2

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// A flow file in a repo is written by people and tools: it has comments, its own key order and
// sections the database does not store (load:, cleanup:, run:). MarshalSimplifiedYAML cannot
// reproduce that text, so saving an edited flow file works on the text instead:
//
//   - source is the file as fetched (or last saved);
//   - clean is the export of its flows right after they were imported from source;
//   - current is the export now.
//
// clean and current come from the same exporter, so comparing them shows exactly what was edited.
// MergeFlowFile applies only those edits to source, leaving every other byte of it as it was.
// Both exports are compared in a canonical form (see canonicalFlowFile) that inlines use_request
// templates, drops node positions and keeps only what a flow file author owns.

// canonicalRootKeys are the top-level keys compared and merged. The others in an export
// (workspace_name, run, requests, credentials, environments, ...) describe the workspace the
// flows were imported into, not the file.
var canonicalRootKeys = map[string]bool{"flows": true, "judge": true, "quality": true}

// positionKeys are node positions on the canvas. Studio lays imported flows out itself, so a
// position is not an edit worth writing back.
var positionKeys = map[string]bool{"position_x": true, "position_y": true}

// errNotMergeable means the source text cannot take the edit in place; the caller replaces the
// enclosing entry instead.
var errNotMergeable = errors.New("not mergeable in place")

// FlowFileChanged reports whether current, an export of a flow file's flows, differs from clean,
// the export taken right after the file was imported or saved.
func FlowFileChanged(clean, current []byte) (bool, error) {
	c, err := canonicalFlowFile(clean)
	if err != nil {
		return false, fmt.Errorf("clean export: %w", err)
	}
	n, err := canonicalFlowFile(current)
	if err != nil {
		return false, fmt.Errorf("current export: %w", err)
	}
	return !nodesEqual(c, n), nil
}

// MergeFlowFile returns source with the edits between clean and current applied. Flows, steps
// and keys that were not edited keep their original text, comments included; an edited step
// value is rewritten in the exporter's form, and a new flow or step is written self-contained.
func MergeFlowFile(source, clean, current []byte) ([]byte, error) {
	cleanRoot, err := canonicalFlowFile(clean)
	if err != nil {
		return nil, fmt.Errorf("clean export: %w", err)
	}
	curRoot, err := canonicalFlowFile(current)
	if err != nil {
		return nil, fmt.Errorf("current export: %w", err)
	}
	if nodesEqual(cleanRoot, curRoot) {
		return source, nil
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(source, &doc); err != nil {
		return nil, fmt.Errorf("source: %w", err)
	}
	srcRoot := documentRoot(&doc)
	if srcRoot == nil || srcRoot.Kind != yaml.MappingNode || srcRoot.Style&yaml.FlowStyle != 0 {
		return nil, errors.New("source: not a block mapping")
	}

	text := string(source)
	trailingNewline := strings.HasSuffix(text, "\n")
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")

	m := &merger{lines: lines, edits: &editSet{replace: map[int]replaceEdit{}, insert: map[int][]string{}}}
	if err := m.mergeMapping(srcRoot, cleanRoot, curRoot, len(lines)); err != nil {
		return nil, fmt.Errorf("merge: %w", err)
	}

	out := strings.Join(m.edits.apply(lines), "\n")
	if trailingNewline {
		out += "\n"
	}

	// The result must still be a flow file.
	if _, err := parseYAMLData([]byte(out)); err != nil {
		return nil, fmt.Errorf("merged flow file does not parse: %w", err)
	}
	return []byte(out), nil
}

// canonicalFlowFile parses an export and keeps the parts a flow file author owns, with each
// step's use_request template inlined and node positions removed.
func canonicalFlowFile(data []byte) (*yaml.Node, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	out := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	root := documentRoot(&doc)
	if root == nil {
		return out, nil
	}
	if root.Kind != yaml.MappingNode {
		return nil, errors.New("not a mapping")
	}

	templates := map[string]map[string]*yaml.Node{
		"request": namedItems(mappingValue(root, "requests")),
		"graphql": namedItems(mappingValue(root, "graphql_requests")),
	}

	for i := 0; i+1 < len(root.Content); i += 2 {
		key, val := root.Content[i], root.Content[i+1]
		if !canonicalRootKeys[key.Value] {
			continue
		}
		v := cloneNode(val)
		if key.Value == "flows" && v.Kind == yaml.SequenceNode {
			for _, flow := range v.Content {
				for _, list := range []string{"steps", "cleanup"} {
					if steps := mappingValue(flow, list); steps != nil && steps.Kind == yaml.SequenceNode {
						for _, step := range steps.Content {
							canonicalStep(step, templates)
						}
					}
				}
			}
		}
		out.Content = append(out.Content, cloneNode(key), v)
	}
	return out, nil
}

// canonicalStep drops a step's positions and replaces its use_request with the template's
// fields, keeping the step's own fields where both set one.
func canonicalStep(step *yaml.Node, templates map[string]map[string]*yaml.Node) {
	if step.Kind != yaml.MappingNode || len(step.Content) != 2 {
		return
	}
	kind, body := step.Content[0].Value, step.Content[1]
	if body.Kind != yaml.MappingNode {
		return
	}

	own := map[string]bool{}
	for i := 0; i+1 < len(body.Content); i += 2 {
		own[body.Content[i].Value] = true
	}

	content := make([]*yaml.Node, 0, len(body.Content))
	for i := 0; i+1 < len(body.Content); i += 2 {
		key, val := body.Content[i], body.Content[i+1]
		switch {
		case positionKeys[key.Value]:
			continue
		case key.Value == "use_request":
			tmpl := templates[kind][val.Value]
			if tmpl == nil {
				content = append(content, key, val)
				continue
			}
			for j := 0; j+1 < len(tmpl.Content); j += 2 {
				tk := tmpl.Content[j]
				if tk.Value == "name" || own[tk.Value] {
					continue
				}
				content = append(content, cloneNode(tk), cloneNode(tmpl.Content[j+1]))
			}
		default:
			content = append(content, key, val)
		}
	}
	body.Content = content
}

// merger collects line edits that turn the source text into the merged text.
type merger struct {
	lines []string
	edits *editSet
}

// entry is a source entry: a mapping key and its value, or a sequence item.
type entry struct {
	key   *yaml.Node // mapping entries only
	value *yaml.Node
	start int // first line, 1-based
	end   int // last line, 1-based
	col   int // column of the key or of the sequence dash, 0-based
}

// replaceWith rewrites the whole entry with a new value.
func (m *merger) replaceWith(e entry, value *yaml.Node) error {
	lines, err := renderEntry(e.key, value, e.col)
	if err != nil {
		return err
	}
	m.edits.replaceLines(e.start, e.end, lines)
	return nil
}

// mergeEntry applies the edit from clean to cur to a source entry, in place where it can and by
// rewriting the entry otherwise.
func (m *merger) mergeEntry(e entry, clean, cur *yaml.Node) error {
	if nodesEqual(clean, cur) {
		return nil
	}
	// A JSON body is a string in the export; compare it the way the source writes it, so only
	// the edited fields change.
	if c, n := bodyInSourceForm(e, clean), bodyInSourceForm(e, cur); c != clean && n != cur {
		clean, cur = c, n
	}
	src := e.value
	if src.Style&yaml.FlowStyle == 0 && src.Kind == clean.Kind && clean.Kind == cur.Kind {
		var err error
		switch src.Kind {
		case yaml.MappingNode:
			err = m.mergeMapping(src, clean, cur, e.end)
		case yaml.SequenceNode:
			err = m.mergeSequence(src, clean, cur, e.end)
		case yaml.DocumentNode, yaml.ScalarNode, yaml.AliasNode:
			err = errNotMergeable
		}
		if err == nil {
			return nil
		}
		if !errors.Is(err, errNotMergeable) {
			return err
		}
	}
	return m.replaceWith(e, bodyInSourceForm(e, cur))
}

// bodyInSourceForm writes an edited JSON body the way the source wrote it (a type: json body or
// a plain mapping) rather than as the exporter's JSON string.
func bodyInSourceForm(e entry, cur *yaml.Node) *yaml.Node {
	if e.key == nil || e.key.Value != "body" || e.value.Kind != yaml.MappingNode || cur.Kind != yaml.ScalarNode {
		return cur
	}
	obj, err := jsonObjectNode(cur.Value)
	if err != nil {
		return cur
	}
	typ := mappingValue(e.value, "type")
	if typ == nil {
		return obj
	}
	if typ.Value != BodyTypeJSON {
		return cur
	}
	return &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{
		strNode("type"), strNode(BodyTypeJSON), strNode("json"), obj,
	}}
}

// jsonObjectNode converts a JSON object to a YAML mapping, keeping key order.
func jsonObjectNode(text string) (*yaml.Node, error) {
	dec := json.NewDecoder(strings.NewReader(text))
	dec.UseNumber()
	n, err := jsonValueNode(dec)
	if err != nil {
		return nil, err
	}
	if n.Kind != yaml.MappingNode {
		return nil, errors.New("not a JSON object")
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("trailing data after JSON object")
	}
	return n, nil
}

func jsonValueNode(dec *json.Decoder) (*yaml.Node, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch v := tok.(type) {
	case json.Delim:
		n := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		if v == '{' {
			n = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		}
		for dec.More() {
			if n.Kind == yaml.MappingNode {
				key, err := dec.Token()
				if err != nil {
					return nil, err
				}
				k, ok := key.(string)
				if !ok {
					return nil, errors.New("invalid JSON object key")
				}
				n.Content = append(n.Content, strNode(k))
			}
			child, err := jsonValueNode(dec)
			if err != nil {
				return nil, err
			}
			n.Content = append(n.Content, child)
		}
		if _, err := dec.Token(); err != nil { // closing delimiter
			return nil, err
		}
		return n, nil
	case string:
		return strNode(v), nil
	case json.Number:
		tag := "!!int"
		if strings.ContainsAny(v.String(), ".eE") {
			tag = "!!float"
		}
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: tag, Value: v.String()}, nil
	case bool:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: strconv.FormatBool(v)}, nil
	case nil:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"}, nil
	default:
		return nil, fmt.Errorf("unexpected JSON token %v", tok)
	}
}

func strNode(v string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v}
}

// mergeMapping merges a block mapping whose entries end at line end. Edited keys are merged or
// rewritten, removed keys deleted and new keys appended; keys only the source has are kept.
func (m *merger) mergeMapping(src, clean, cur *yaml.Node, end int) error {
	entries, err := m.mappingEntries(src, end)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return errNotMergeable
	}
	srcByKey := make(map[string]entry, len(entries))
	for _, e := range entries {
		srcByKey[e.key.Value] = e
	}

	// Check first, so a mapping that cannot take the edit is left untouched for the caller.
	type change struct {
		key        string
		src        entry
		inSrc      bool
		clean, cur *yaml.Node
	}
	var changes []change
	seen := map[string]bool{}
	for i := 0; i+1 < len(cur.Content); i += 2 {
		k := cur.Content[i].Value
		seen[k] = true
		cv, clv := cur.Content[i+1], mappingValue(clean, k)
		if clv != nil && nodesEqual(clv, cv) {
			continue
		}
		s, inSrc := srcByKey[k]
		if !inSrc && clv != nil {
			// The source writes this value under another key or shape.
			return errNotMergeable
		}
		changes = append(changes, change{key: k, src: s, inSrc: inSrc, clean: clv, cur: cv})
	}
	for i := 0; i+1 < len(clean.Content); i += 2 {
		k := clean.Content[i].Value
		if seen[k] {
			continue
		}
		s, inSrc := srcByKey[k]
		changes = append(changes, change{key: k, src: s, inSrc: inSrc, clean: clean.Content[i+1]})
	}

	var appended []string
	for _, c := range changes {
		switch {
		case c.cur == nil && c.inSrc:
			m.edits.replaceLines(c.src.start, c.src.end, nil)
		case c.cur == nil:
			// Removed, and the source never had it.
		case c.inSrc && c.clean != nil:
			if err := m.mergeEntry(c.src, c.clean, c.cur); err != nil {
				return err
			}
		case c.inSrc:
			if err := m.replaceWith(c.src, c.cur); err != nil {
				return err
			}
		default:
			lines, err := renderEntry(&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: c.key}, c.cur, entries[0].col)
			if err != nil {
				return err
			}
			appended = append(appended, lines...)
		}
	}
	if len(appended) > 0 {
		m.edits.insertAfter(entries[len(entries)-1].end, appended)
	}
	return nil
}

// mergeSequence merges a block sequence of named items (flows, steps) whose items end at line
// end. Other sequences are not merged in place.
func (m *merger) mergeSequence(src, clean, cur *yaml.Node, end int) error {
	srcNames, ok := itemNames(src)
	if !ok {
		return errNotMergeable
	}
	cleanNames, ok := itemNames(clean)
	if !ok {
		return errNotMergeable
	}
	curNames, ok := itemNames(cur)
	if !ok || len(src.Content) == 0 {
		return errNotMergeable
	}
	entries, err := m.sequenceEntries(src, end)
	if err != nil {
		return err
	}

	srcIdx := indexOf(srcNames)
	cleanIdx := indexOf(cleanNames)
	curIdx := indexOf(curNames)

	// Check first: an item the source lacks but clean has cannot be edited in place.
	for i, name := range curNames {
		_, inSrc := srcIdx[name]
		ci, inClean := cleanIdx[name]
		if !inSrc && inClean && !nodesEqual(clean.Content[ci], cur.Content[i]) {
			return errNotMergeable
		}
	}

	anchor := entries[0].start - 1 // new items before any source item go first
	for i, name := range curNames {
		si, inSrc := srcIdx[name]
		ci, inClean := cleanIdx[name]
		switch {
		case inSrc && inClean:
			if err := m.mergeEntry(entries[si], clean.Content[ci], cur.Content[i]); err != nil {
				return err
			}
			anchor = entries[si].end
		case inSrc:
			if err := m.replaceWith(entries[si], cur.Content[i]); err != nil {
				return err
			}
			anchor = entries[si].end
		case inClean:
			// Unchanged and not in the source: nothing to write.
		default:
			lines, err := renderEntry(nil, cur.Content[i], entries[0].col)
			if err != nil {
				return err
			}
			m.edits.insertAfter(anchor, lines)
		}
	}
	for name := range cleanIdx {
		if _, kept := curIdx[name]; kept {
			continue
		}
		if si, inSrc := srcIdx[name]; inSrc {
			m.edits.replaceLines(entries[si].start, entries[si].end, nil)
		}
	}
	return nil
}

// mappingEntries returns the line ranges of a block mapping's entries; the last one ends at end.
func (m *merger) mappingEntries(n *yaml.Node, end int) ([]entry, error) {
	var out []entry
	for i := 0; i+1 < len(n.Content); i += 2 {
		k := n.Content[i]
		if k.Line == 0 {
			return nil, errNotMergeable
		}
		out = append(out, entry{key: k, value: n.Content[i+1], start: k.Line, col: k.Column - 1})
	}
	m.closeEntries(out, end)
	return out, nil
}

// sequenceEntries returns the line ranges of a block sequence's items; the last one ends at end.
func (m *merger) sequenceEntries(n *yaml.Node, end int) ([]entry, error) {
	var out []entry
	for _, item := range n.Content {
		if item.Line == 0 || item.Line > len(m.lines) {
			return nil, errNotMergeable
		}
		// The item starts after "- " on its line.
		line := m.lines[item.Line-1]
		limit := min(item.Column-1, len(line))
		dash := strings.LastIndex(line[:limit], "-")
		if dash < 0 || strings.TrimSpace(line[:dash]) != "" {
			return nil, errNotMergeable
		}
		out = append(out, entry{value: item, start: item.Line, col: dash})
	}
	m.closeEntries(out, end)
	return out, nil
}

// closeEntries sets each entry's last line: the line before the next entry (or end), less the
// blank and comment lines in between that sit at or left of the entry's column, which belong to
// whatever follows.
func (m *merger) closeEntries(entries []entry, end int) {
	for i := range entries {
		last := end
		if i+1 < len(entries) {
			last = entries[i+1].start - 1
		}
		for last > entries[i].start && m.trailingLine(last, entries[i].col) {
			last--
		}
		entries[i].end = last
	}
}

func (m *merger) trailingLine(n, col int) bool {
	line := m.lines[n-1]
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return true
	}
	return strings.HasPrefix(trimmed, "#") && len(line)-len(strings.TrimLeft(line, " \t")) <= col
}

// renderEntry writes "key: value" (or "- value" without a key) indented by col spaces.
func renderEntry(key, value *yaml.Node, col int) ([]string, error) {
	var n *yaml.Node
	if key != nil {
		n = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{plainNode(key), plainNode(value)}}
	} else {
		n = &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Content: []*yaml.Node{plainNode(value)}}
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(n); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	pad := strings.Repeat(" ", col)
	lines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	for i, l := range lines {
		if l != "" {
			lines[i] = pad + l
		}
	}
	return lines, nil
}

// editSet holds non-overlapping line replacements and insertions, by 1-based line number.
type editSet struct {
	replace map[int]replaceEdit
	insert  map[int][]string // inserted after the line (0 = before the first line)
}

type replaceEdit struct {
	end   int
	lines []string
}

func (s *editSet) replaceLines(start, end int, lines []string) {
	s.replace[start] = replaceEdit{end: end, lines: lines}
}

func (s *editSet) insertAfter(line int, lines []string) {
	s.insert[line] = append(s.insert[line], lines...)
}

func (s *editSet) apply(lines []string) []string {
	out := make([]string, 0, len(lines))
	for n := 1; n <= len(lines); {
		out = append(out, s.insert[n-1]...)
		if r, ok := s.replace[n]; ok {
			out = append(out, r.lines...)
			n = r.end + 1
			continue
		}
		out = append(out, lines[n-1])
		n++
	}
	return append(out, s.insert[len(lines)]...)
}

// itemNames names a sequence's items: flows and variables by name, steps by type and name. ok is
// false when an item has no name or two share one.
func itemNames(seq *yaml.Node) ([]string, bool) {
	names := make([]string, 0, len(seq.Content))
	seen := map[string]bool{}
	for _, item := range seq.Content {
		name := itemName(item)
		if name == "" || seen[name] {
			return nil, false
		}
		seen[name] = true
		names = append(names, name)
	}
	return names, true
}

func itemName(item *yaml.Node) string {
	if item.Kind != yaml.MappingNode {
		return ""
	}
	if n := mappingValue(item, "name"); n != nil && n.Kind == yaml.ScalarNode {
		return n.Value
	}
	if len(item.Content) == 2 {
		if n := mappingValue(item.Content[1], "name"); n != nil && n.Kind == yaml.ScalarNode {
			return item.Content[0].Value + "/" + n.Value
		}
	}
	return ""
}

func indexOf(names []string) map[string]int {
	idx := make(map[string]int, len(names))
	for i, n := range names {
		idx[n] = i
	}
	return idx
}

// namedItems indexes a sequence of mappings by their name.
func namedItems(seq *yaml.Node) map[string]*yaml.Node {
	out := map[string]*yaml.Node{}
	if seq == nil || seq.Kind != yaml.SequenceNode {
		return out
	}
	for _, item := range seq.Content {
		if n := mappingValue(item, "name"); n != nil && item.Kind == yaml.MappingNode {
			out[n.Value] = item
		}
	}
	return out
}

func documentRoot(doc *yaml.Node) *yaml.Node {
	if doc.Kind == yaml.DocumentNode {
		if len(doc.Content) == 0 {
			return nil
		}
		return doc.Content[0]
	}
	return doc
}

// nodesEqual compares two nodes by kind, resolved tag and value, ignoring style, comments and
// positions.
func nodesEqual(a, b *yaml.Node) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.Kind == yaml.AliasNode {
		return nodesEqual(a.Alias, b)
	}
	if b.Kind == yaml.AliasNode {
		return nodesEqual(a, b.Alias)
	}
	if a.Kind != b.Kind || len(a.Content) != len(b.Content) {
		return false
	}
	if a.Kind == yaml.ScalarNode && (a.ShortTag() != b.ShortTag() || a.Value != b.Value) {
		return false
	}
	for i := range a.Content {
		if !nodesEqual(a.Content[i], b.Content[i]) {
			return false
		}
	}
	return true
}

// cloneNode deep-copies a node, resolving aliases and dropping comments and positions.
func cloneNode(n *yaml.Node) *yaml.Node {
	if n == nil {
		return nil
	}
	if n.Kind == yaml.AliasNode && n.Alias != nil {
		return cloneNode(n.Alias)
	}
	c := &yaml.Node{Kind: n.Kind, Style: n.Style, Tag: n.Tag, Value: n.Value}
	if len(n.Content) > 0 {
		c.Content = make([]*yaml.Node, len(n.Content))
		for i, child := range n.Content {
			c.Content[i] = cloneNode(child)
		}
	}
	return c
}

// plainNode copies a node for writing in block style.
func plainNode(n *yaml.Node) *yaml.Node {
	c := cloneNode(n)
	var unflow func(*yaml.Node)
	unflow = func(x *yaml.Node) {
		if x.Kind == yaml.MappingNode || x.Kind == yaml.SequenceNode {
			x.Style &^= yaml.FlowStyle
		}
		for _, child := range x.Content {
			unflow(child)
		}
	}
	unflow(c)
	return c
}
