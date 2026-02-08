package openyaml

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/ioworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/menv"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mfile"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mworkspace"
	yfs "github.com/the-dev-tools/dev-tools/packages/server/pkg/translate/yamlflowsimplev2"
)

const (
	flowsDir        = "flows"
	environmentsDir = "environments"
	yamlExt         = ".yaml"
)

// ReadOptions configures directory reading.
type ReadOptions struct {
	WorkspaceID idwrap.IDWrap
}

// httpLookup holds pre-built index maps for writing HTTP requests to disk.
type httpLookup struct {
	HTTP       map[idwrap.IDWrap]mhttp.HTTP
	Headers    map[idwrap.IDWrap][]mhttp.HTTPHeader
	Params     map[idwrap.IDWrap][]mhttp.HTTPSearchParam
	BodyRaw    map[idwrap.IDWrap]mhttp.HTTPBodyRaw
	BodyForm   map[idwrap.IDWrap][]mhttp.HTTPBodyForm
	BodyURL    map[idwrap.IDWrap][]mhttp.HTTPBodyUrlencoded
	Assertions map[idwrap.IDWrap][]mhttp.HTTPAssert
}

func buildHTTPLookup(bundle *ioworkspace.WorkspaceBundle) *httpLookup {
	lk := &httpLookup{
		HTTP:       make(map[idwrap.IDWrap]mhttp.HTTP, len(bundle.HTTPRequests)),
		Headers:    make(map[idwrap.IDWrap][]mhttp.HTTPHeader),
		Params:     make(map[idwrap.IDWrap][]mhttp.HTTPSearchParam),
		BodyRaw:    make(map[idwrap.IDWrap]mhttp.HTTPBodyRaw),
		BodyForm:   make(map[idwrap.IDWrap][]mhttp.HTTPBodyForm),
		BodyURL:    make(map[idwrap.IDWrap][]mhttp.HTTPBodyUrlencoded),
		Assertions: make(map[idwrap.IDWrap][]mhttp.HTTPAssert),
	}
	for _, h := range bundle.HTTPRequests {
		lk.HTTP[h.ID] = h
	}
	for _, h := range bundle.HTTPHeaders {
		lk.Headers[h.HttpID] = append(lk.Headers[h.HttpID], h)
	}
	for _, p := range bundle.HTTPSearchParams {
		lk.Params[p.HttpID] = append(lk.Params[p.HttpID], p)
	}
	for _, b := range bundle.HTTPBodyRaw {
		lk.BodyRaw[b.HttpID] = b
	}
	for _, f := range bundle.HTTPBodyForms {
		lk.BodyForm[f.HttpID] = append(lk.BodyForm[f.HttpID], f)
	}
	for _, u := range bundle.HTTPBodyUrlencoded {
		lk.BodyURL[u.HttpID] = append(lk.BodyURL[u.HttpID], u)
	}
	for _, a := range bundle.HTTPAsserts {
		lk.Assertions[a.HttpID] = append(lk.Assertions[a.HttpID], a)
	}
	return lk
}

// ReadDirectory reads an OpenYAML folder into a WorkspaceBundle.
// Directory structure:
//   - *.yaml files in root/subdirs -> YamlRequestDefV2 -> mhttp models
//   - flows/*.yaml -> YamlFlowFlowV2 -> mflow models
//   - environments/*.yaml -> YamlEnvironmentV2 -> menv models
//   - Subdirectories -> mfile.File (ContentTypeFolder)
func ReadDirectory(dirPath string, opts ReadOptions) (*ioworkspace.WorkspaceBundle, error) {
	bundle := &ioworkspace.WorkspaceBundle{
		Workspace: mworkspace.Workspace{
			ID:   opts.WorkspaceID,
			Name: filepath.Base(dirPath),
		},
	}

	now := time.Now()

	// Read environments
	envDir := filepath.Join(dirPath, environmentsDir)
	if info, err := os.Stat(envDir); err == nil && info.IsDir() {
		if err := readEnvironments(envDir, opts.WorkspaceID, bundle); err != nil {
			return nil, fmt.Errorf("read environments: %w", err)
		}
	}

	// Read flows
	flowDir := filepath.Join(dirPath, flowsDir)
	if info, err := os.Stat(flowDir); err == nil && info.IsDir() {
		if err := readFlows(flowDir, opts.WorkspaceID, bundle); err != nil {
			return nil, fmt.Errorf("read flows: %w", err)
		}
	}

	// Read requests recursively (excluding flows/ and environments/ dirs)
	if err := readRequestsRecursive(dirPath, dirPath, nil, opts.WorkspaceID, now, bundle); err != nil {
		return nil, fmt.Errorf("read requests: %w", err)
	}

	return bundle, nil
}

// WriteDirectory exports a WorkspaceBundle to an OpenYAML folder.
// Creates one .yaml file per request, flow, and environment.
// Directory structure mirrors the mfile.File hierarchy.
func WriteDirectory(dirPath string, bundle *ioworkspace.WorkspaceBundle) error {
	if err := os.MkdirAll(dirPath, 0o750); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	lk := buildHTTPLookup(bundle)

	// Write environments
	if len(bundle.Environments) > 0 {
		envDir := filepath.Join(dirPath, environmentsDir)
		if err := os.MkdirAll(envDir, 0o750); err != nil {
			return fmt.Errorf("create environments dir: %w", err)
		}

		envVarsByEnv := make(map[idwrap.IDWrap][]menv.Variable)
		for _, v := range bundle.EnvironmentVars {
			envVarsByEnv[v.EnvID] = append(envVarsByEnv[v.EnvID], v)
		}

		for _, env := range bundle.Environments {
			yamlEnv := yfs.YamlEnvironmentV2{
				Name:        env.Name,
				Description: env.Description,
				Variables:   make(map[string]string),
			}

			vars := envVarsByEnv[env.ID]
			sort.Slice(vars, func(i, j int) bool { return vars[i].Order < vars[j].Order })
			for _, v := range vars {
				yamlEnv.Variables[v.VarKey] = v.Value
			}

			data, err := WriteSingleEnvironment(yamlEnv)
			if err != nil {
				return fmt.Errorf("marshal environment %q: %w", env.Name, err)
			}

			filename := sanitizeFilename(env.Name) + yamlExt
			if err := atomicWrite(filepath.Join(envDir, filename), data); err != nil {
				return fmt.Errorf("write environment %q: %w", env.Name, err)
			}
		}
	}

	// Write flows
	if len(bundle.Flows) > 0 {
		flowDir := filepath.Join(dirPath, flowsDir)
		if err := os.MkdirAll(flowDir, 0o750); err != nil {
			return fmt.Errorf("create flows dir: %w", err)
		}

		for _, flow := range bundle.Flows {
			yamlFlow := exportFlow(flow, bundle)
			data, err := WriteSingleFlow(yamlFlow)
			if err != nil {
				return fmt.Errorf("marshal flow %q: %w", flow.Name, err)
			}

			filename := sanitizeFilename(flow.Name) + yamlExt
			if err := atomicWrite(filepath.Join(flowDir, filename), data); err != nil {
				return fmt.Errorf("write flow %q: %w", flow.Name, err)
			}
		}
	}

	// Write requests organized by file hierarchy
	childrenByParent := make(map[string][]mfile.File)
	for _, f := range bundle.Files {
		parentKey := ""
		if f.ParentID != nil {
			parentKey = f.ParentID.String()
		}
		childrenByParent[parentKey] = append(childrenByParent[parentKey], f)
	}

	return writeFilesRecursive(dirPath, "", childrenByParent, lk)
}

func readEnvironments(envDir string, workspaceID idwrap.IDWrap, bundle *ioworkspace.WorkspaceBundle) error {
	entries, err := os.ReadDir(envDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || !isYAMLFile(entry.Name()) {
			continue
		}

		data, err := os.ReadFile(filepath.Join(envDir, entry.Name())) //nolint:gosec // Intentional: reading from user-specified sync directory
		if err != nil {
			return fmt.Errorf("read %s: %w", entry.Name(), err)
		}

		yamlEnv, err := ReadSingleEnvironment(data)
		if err != nil {
			return fmt.Errorf("parse %s: %w", entry.Name(), err)
		}

		envID := idwrap.NewNow()
		env := menv.Env{
			ID:          envID,
			WorkspaceID: workspaceID,
			Type:        menv.EnvNormal,
			Name:        yamlEnv.Name,
			Description: yamlEnv.Description,
		}
		bundle.Environments = append(bundle.Environments, env)

		// Sort keys for deterministic ordering
		var keys []string
		for k := range yamlEnv.Variables {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for i, k := range keys {
			bundle.EnvironmentVars = append(bundle.EnvironmentVars, menv.Variable{
				ID:      idwrap.NewNow(),
				EnvID:   envID,
				VarKey:  k,
				Value:   yamlEnv.Variables[k],
				Enabled: true,
				Order:   float64(i + 1),
			})
		}
	}

	return nil
}

func readFlows(flowDir string, workspaceID idwrap.IDWrap, bundle *ioworkspace.WorkspaceBundle) error {
	entries, err := os.ReadDir(flowDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || !isYAMLFile(entry.Name()) {
			continue
		}

		data, err := os.ReadFile(filepath.Join(flowDir, entry.Name())) //nolint:gosec // Intentional: reading from user-specified sync directory
		if err != nil {
			return fmt.Errorf("read %s: %w", entry.Name(), err)
		}

		yamlFlow, err := ReadSingleFlow(data)
		if err != nil {
			return fmt.Errorf("parse %s: %w", entry.Name(), err)
		}

		flowID := idwrap.NewNow()
		flow := mflow.Flow{
			ID:          flowID,
			WorkspaceID: workspaceID,
			Name:        yamlFlow.Name,
		}
		bundle.Flows = append(bundle.Flows, flow)

		// Create file entry for the flow
		contentID := flowID
		bundle.Files = append(bundle.Files, mfile.File{
			ID:          idwrap.NewNow(),
			WorkspaceID: workspaceID,
			ContentID:   &contentID,
			ContentType: mfile.ContentTypeFlow,
			Name:        yamlFlow.Name,
		})

		// Convert flow variables
		for _, v := range yamlFlow.Variables {
			bundle.FlowVariables = append(bundle.FlowVariables, mflow.FlowVariable{
				ID:     idwrap.NewNow(),
				FlowID: flowID,
				Name:   v.Name,
				Value:  v.Value,
			})
		}
	}

	return nil
}

func readRequestsRecursive(
	rootDir string,
	dirPath string,
	parentID *idwrap.IDWrap,
	workspaceID idwrap.IDWrap,
	now time.Time,
	bundle *ioworkspace.WorkspaceBundle,
) error {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return err
	}

	order := float64(1)
	for _, entry := range entries {
		name := entry.Name()

		// Skip special directories and hidden files
		if strings.HasPrefix(name, ".") {
			continue
		}

		if entry.IsDir() {
			// Skip reserved directories
			rel, _ := filepath.Rel(rootDir, filepath.Join(dirPath, name))
			if rel == flowsDir || rel == environmentsDir {
				continue
			}

			// Create folder file entry
			folderID := idwrap.NewNow()
			folderContentID := folderID
			pathHash := computePathHash(rel)
			bundle.Files = append(bundle.Files, mfile.File{
				ID:          folderID,
				WorkspaceID: workspaceID,
				ParentID:    parentID,
				ContentID:   &folderContentID,
				ContentType: mfile.ContentTypeFolder,
				Name:        name,
				Order:       order,
				PathHash:    &pathHash,
				UpdatedAt:   now,
			})

			if err := readRequestsRecursive(rootDir, filepath.Join(dirPath, name), &folderID, workspaceID, now, bundle); err != nil {
				return err
			}
			order++
			continue
		}

		if !isYAMLFile(name) {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dirPath, name)) //nolint:gosec // Intentional: reading from user-specified sync directory
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}

		yamlReq, err := ReadSingleRequest(data)
		if err != nil {
			return fmt.Errorf("parse %s: %w", name, err)
		}

		fileOrder := order
		if yamlReq.Order > 0 {
			fileOrder = yamlReq.Order
		}

		httpID := idwrap.NewNow()
		nowMs := now.UnixMilli()

		// Determine body kind
		bodyKind := mhttp.HttpBodyKindNone
		if yamlReq.Body != nil {
			switch strings.ToLower(yamlReq.Body.Type) {
			case "form_data", "form-data":
				bodyKind = mhttp.HttpBodyKindFormData
			case "urlencoded":
				bodyKind = mhttp.HttpBodyKindUrlEncoded
			case "raw", "json", "xml", "text":
				bodyKind = mhttp.HttpBodyKindRaw
			}
		}

		httpReq := mhttp.HTTP{
			ID:          httpID,
			WorkspaceID: workspaceID,
			Name:        yamlReq.Name,
			Url:         yamlReq.URL,
			Method:      strings.ToUpper(yamlReq.Method),
			Description: yamlReq.Description,
			BodyKind:    bodyKind,
			CreatedAt:   nowMs,
			UpdatedAt:   nowMs,
		}
		if httpReq.Method == "" {
			httpReq.Method = "GET"
		}
		bundle.HTTPRequests = append(bundle.HTTPRequests, httpReq)

		// Create file entry
		contentID := httpID
		relPath, _ := filepath.Rel(rootDir, filepath.Join(dirPath, name))
		pathHash := computePathHash(relPath)
		bundle.Files = append(bundle.Files, mfile.File{
			ID:          idwrap.NewNow(),
			WorkspaceID: workspaceID,
			ParentID:    parentID,
			ContentID:   &contentID,
			ContentType: mfile.ContentTypeHTTP,
			Name:        yamlReq.Name,
			Order:       fileOrder,
			PathHash:    &pathHash,
			UpdatedAt:   now,
		})

		// Convert headers
		for i, h := range yamlReq.Headers {
			bundle.HTTPHeaders = append(bundle.HTTPHeaders, mhttp.HTTPHeader{
				ID:           idwrap.NewNow(),
				HttpID:       httpID,
				Key:          h.Name,
				Value:        h.Value,
				Enabled:      h.Enabled,
				Description:  h.Description,
				DisplayOrder: float32(i + 1),
				CreatedAt:    nowMs,
				UpdatedAt:    nowMs,
			})
		}

		// Convert query params
		for i, p := range yamlReq.QueryParams {
			bundle.HTTPSearchParams = append(bundle.HTTPSearchParams, mhttp.HTTPSearchParam{
				ID:           idwrap.NewNow(),
				HttpID:       httpID,
				Key:          p.Name,
				Value:        p.Value,
				Enabled:      p.Enabled,
				Description:  p.Description,
				DisplayOrder: float64(i + 1),
				CreatedAt:    nowMs,
				UpdatedAt:    nowMs,
			})
		}

		// Convert body
		if yamlReq.Body != nil {
			convertYAMLBody(yamlReq.Body, httpID, nowMs, bundle)
		}

		// Convert assertions
		for i, a := range yamlReq.Assertions {
			bundle.HTTPAsserts = append(bundle.HTTPAsserts, mhttp.HTTPAssert{
				ID:           idwrap.NewNow(),
				HttpID:       httpID,
				Value:        a.Expression,
				Enabled:      a.Enabled,
				DisplayOrder: float32(i + 1),
				CreatedAt:    nowMs,
				UpdatedAt:    nowMs,
			})
		}

		order++
	}

	return nil
}

func convertYAMLBody(body *yfs.YamlBodyUnion, httpID idwrap.IDWrap, nowMs int64, bundle *ioworkspace.WorkspaceBundle) {
	switch strings.ToLower(body.Type) {
	case "form_data", "form-data":
		for i, f := range body.Form {
			bundle.HTTPBodyForms = append(bundle.HTTPBodyForms, mhttp.HTTPBodyForm{
				ID:           idwrap.NewNow(),
				HttpID:       httpID,
				Key:          f.Name,
				Value:        f.Value,
				Enabled:      f.Enabled,
				Description:  f.Description,
				DisplayOrder: float32(i + 1),
				CreatedAt:    nowMs,
				UpdatedAt:    nowMs,
			})
		}
	case "urlencoded":
		for i, u := range body.UrlEncoded {
			bundle.HTTPBodyUrlencoded = append(bundle.HTTPBodyUrlencoded, mhttp.HTTPBodyUrlencoded{
				ID:           idwrap.NewNow(),
				HttpID:       httpID,
				Key:          u.Name,
				Value:        u.Value,
				Enabled:      u.Enabled,
				Description:  u.Description,
				DisplayOrder: float32(i + 1),
				CreatedAt:    nowMs,
				UpdatedAt:    nowMs,
			})
		}
	default:
		// Raw body (json, xml, text, raw)
		rawData := body.Raw
		if rawData == "" && body.JSON != nil {
			// Marshal JSON map back to string
			b, _ := yaml.Marshal(body.JSON)
			rawData = string(b)
		}
		if rawData != "" {
			bundle.HTTPBodyRaw = append(bundle.HTTPBodyRaw, mhttp.HTTPBodyRaw{
				ID:        idwrap.NewNow(),
				HttpID:    httpID,
				RawData:   []byte(rawData),
				CreatedAt: nowMs,
				UpdatedAt: nowMs,
			})
		}
	}
}

func writeFilesRecursive(
	currentDir string,
	parentIDStr string,
	childrenByParent map[string][]mfile.File,
	lk *httpLookup,
) error {
	children := childrenByParent[parentIDStr]
	sort.Slice(children, func(i, j int) bool { return children[i].Order < children[j].Order })

	for _, f := range children {
		switch f.ContentType {
		case mfile.ContentTypeFolder:
			subDir := filepath.Join(currentDir, sanitizeFilename(f.Name))
			if err := os.MkdirAll(subDir, 0o750); err != nil {
				return fmt.Errorf("create dir %q: %w", f.Name, err)
			}
			if err := writeFilesRecursive(subDir, f.ID.String(), childrenByParent, lk); err != nil {
				return err
			}

		case mfile.ContentTypeHTTP:
			if f.ContentID == nil {
				continue
			}
			httpReq, ok := lk.HTTP[*f.ContentID]
			if !ok {
				continue
			}

			yamlReq := exportHTTPRequest(httpReq, f.Order, lk)
			data, err := WriteSingleRequest(yamlReq)
			if err != nil {
				return fmt.Errorf("marshal request %q: %w", httpReq.Name, err)
			}

			filename := sanitizeFilename(httpReq.Name) + yamlExt
			if err := atomicWrite(filepath.Join(currentDir, filename), data); err != nil {
				return fmt.Errorf("write request %q: %w", httpReq.Name, err)
			}

		case mfile.ContentTypeHTTPDelta, mfile.ContentTypeFlow, mfile.ContentTypeCredential:
			// These content types are not exported to OpenYAML format
		}
	}

	return nil
}

func exportHTTPRequest(httpReq mhttp.HTTP, order float64, lk *httpLookup) yfs.YamlRequestDefV2 {
	req := yfs.YamlRequestDefV2{
		Name:        httpReq.Name,
		Method:      httpReq.Method,
		URL:         httpReq.Url,
		Description: httpReq.Description,
		Order:       order,
	}

	// Headers
	headers := lk.Headers[httpReq.ID]
	if len(headers) > 0 {
		var pairs []yfs.YamlNameValuePairV2
		for _, h := range headers {
			pairs = append(pairs, yfs.YamlNameValuePairV2{
				Name:        h.Key,
				Value:       h.Value,
				Description: h.Description,
				Enabled:     h.Enabled,
			})
		}
		req.Headers = yfs.HeaderMapOrSlice(pairs)
	}

	// Query params
	params := lk.Params[httpReq.ID]
	if len(params) > 0 {
		var pairs []yfs.YamlNameValuePairV2
		for _, p := range params {
			pairs = append(pairs, yfs.YamlNameValuePairV2{
				Name:        p.Key,
				Value:       p.Value,
				Description: p.Description,
				Enabled:     p.Enabled,
			})
		}
		req.QueryParams = yfs.HeaderMapOrSlice(pairs)
	}

	// Body
	switch httpReq.BodyKind {
	case mhttp.HttpBodyKindFormData:
		forms := lk.BodyForm[httpReq.ID]
		if len(forms) > 0 {
			var pairs []yfs.YamlNameValuePairV2
			for _, f := range forms {
				pairs = append(pairs, yfs.YamlNameValuePairV2{
					Name:        f.Key,
					Value:       f.Value,
					Description: f.Description,
					Enabled:     f.Enabled,
				})
			}
			req.Body = &yfs.YamlBodyUnion{
				Type: "form_data",
				Form: yfs.HeaderMapOrSlice(pairs),
			}
		}
	case mhttp.HttpBodyKindUrlEncoded:
		urls := lk.BodyURL[httpReq.ID]
		if len(urls) > 0 {
			var pairs []yfs.YamlNameValuePairV2
			for _, u := range urls {
				pairs = append(pairs, yfs.YamlNameValuePairV2{
					Name:        u.Key,
					Value:       u.Value,
					Description: u.Description,
					Enabled:     u.Enabled,
				})
			}
			req.Body = &yfs.YamlBodyUnion{
				Type:       "urlencoded",
				UrlEncoded: yfs.HeaderMapOrSlice(pairs),
			}
		}
	case mhttp.HttpBodyKindRaw:
		if raw, ok := lk.BodyRaw[httpReq.ID]; ok && len(raw.RawData) > 0 {
			req.Body = &yfs.YamlBodyUnion{
				Type: "raw",
				Raw:  string(raw.RawData),
			}
		}
	}

	// Assertions
	asserts := lk.Assertions[httpReq.ID]
	if len(asserts) > 0 {
		var yamlAsserts []yfs.YamlAssertionV2
		for _, a := range asserts {
			yamlAsserts = append(yamlAsserts, yfs.YamlAssertionV2{
				Expression: a.Value,
				Enabled:    a.Enabled,
			})
		}
		req.Assertions = yfs.AssertionsOrSlice(yamlAsserts)
	}

	return req
}

func exportFlow(flow mflow.Flow, bundle *ioworkspace.WorkspaceBundle) yfs.YamlFlowFlowV2 {
	yamlFlow := yfs.YamlFlowFlowV2{
		Name: flow.Name,
	}

	// Variables
	for _, v := range bundle.FlowVariables {
		if v.FlowID.Compare(flow.ID) == 0 {
			yamlFlow.Variables = append(yamlFlow.Variables, yfs.YamlFlowVariableV2{
				Name:  v.Name,
				Value: v.Value,
			})
		}
	}

	return yamlFlow
}

// computePathHash returns a SHA-256 hash of the given path for deduplication.
func computePathHash(relPath string) string {
	h := sha256.Sum256([]byte(relPath))
	return hex.EncodeToString(h[:])
}

// atomicWrite writes data to a temp file then renames for safety.
func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".openyaml-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}

	return os.Rename(tmpName, path)
}

// sanitizeFilename makes a string safe for use as a filename.
func sanitizeFilename(name string) string {
	if name == "" {
		return "untitled"
	}

	// Replace characters that are problematic in filenames
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
	)
	name = replacer.Replace(name)
	name = strings.TrimSpace(name)

	// Convert to lowercase kebab-case for consistency
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "-")

	if name == "" {
		return "untitled"
	}

	if len(name) > 255 {
		name = name[:255]
	}

	return name
}

func isYAMLFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".yaml" || ext == ".yml"
}
