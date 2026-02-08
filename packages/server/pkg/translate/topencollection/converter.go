package topencollection

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/ioworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mfile"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mworkspace"
)

// ConvertOptions configures the OpenCollection import.
type ConvertOptions struct {
	WorkspaceID idwrap.IDWrap
	Logger      *slog.Logger
}

// ConvertOpenCollection walks the given directory, parses each .yml file, and converts
// to DevTools models. Only info.type == "http" requests are imported.
// GraphQL, WebSocket, and gRPC types are skipped with a log warning.
func ConvertOpenCollection(collectionPath string, opts ConvertOptions) (*ioworkspace.WorkspaceBundle, error) {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	// Parse collection root
	rootPath := filepath.Join(collectionPath, "opencollection.yml")
	rootData, err := os.ReadFile(rootPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read opencollection.yml: %w", err)
	}

	var root OpenCollectionRoot
	if err := yaml.Unmarshal(rootData, &root); err != nil {
		return nil, fmt.Errorf("failed to parse opencollection.yml: %w", err)
	}

	bundle := &ioworkspace.WorkspaceBundle{
		Workspace: mworkspace.Workspace{
			ID:   opts.WorkspaceID,
			Name: root.Info.Name,
		},
	}

	now := time.Now().UnixMilli()

	// Walk directory tree recursively
	if err := walkCollection(collectionPath, collectionPath, nil, opts.WorkspaceID, now, bundle, logger); err != nil {
		return nil, fmt.Errorf("failed to walk collection: %w", err)
	}

	// Parse environments
	envDir := filepath.Join(collectionPath, "environments")
	if info, err := os.Stat(envDir); err == nil && info.IsDir() {
		entries, err := os.ReadDir(envDir)
		if err != nil {
			return nil, fmt.Errorf("failed to read environments directory: %w", err)
		}

		for _, entry := range entries {
			if entry.IsDir() || !isYAMLFile(entry.Name()) {
				continue
			}

			envData, err := os.ReadFile(filepath.Join(envDir, entry.Name()))
			if err != nil {
				logger.Warn("failed to read environment file", "file", entry.Name(), "error", err)
				continue
			}

			var ocEnv OCEnvironment
			if err := yaml.Unmarshal(envData, &ocEnv); err != nil {
				logger.Warn("failed to parse environment file", "file", entry.Name(), "error", err)
				continue
			}

			env, vars := convertEnvironment(ocEnv, opts.WorkspaceID)
			bundle.Environments = append(bundle.Environments, env)
			bundle.EnvironmentVars = append(bundle.EnvironmentVars, vars...)
		}
	}

	return bundle, nil
}

// walkCollection recursively walks a directory in the collection, creating
// file entries for folders and converting request files.
func walkCollection(
	rootPath string,
	dirPath string,
	parentID *idwrap.IDWrap,
	workspaceID idwrap.IDWrap,
	now int64,
	bundle *ioworkspace.WorkspaceBundle,
	logger *slog.Logger,
) error {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %w", dirPath, err)
	}

	// Separate folders and files, sort by name for consistent ordering
	var dirs []os.DirEntry
	var files []os.DirEntry
	for _, entry := range entries {
		if entry.IsDir() {
			// Skip environments dir at root level (handled separately)
			relPath, _ := filepath.Rel(rootPath, filepath.Join(dirPath, entry.Name()))
			if relPath == "environments" {
				continue
			}
			// Skip hidden directories
			if strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			dirs = append(dirs, entry)
		} else if isYAMLFile(entry.Name()) {
			// Skip opencollection.yml and folder.yml
			if entry.Name() == "opencollection.yml" || entry.Name() == "folder.yml" {
				continue
			}
			files = append(files, entry)
		}
	}

	// Process request files first
	order := float64(1)
	for _, fileEntry := range files {
		filePath := filepath.Join(dirPath, fileEntry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			logger.Warn("failed to read file", "file", filePath, "error", err)
			continue
		}

		var ocReq OCRequest
		if err := yaml.Unmarshal(data, &ocReq); err != nil {
			logger.Warn("failed to parse request file", "file", filePath, "error", err)
			continue
		}

		// Check request type — only import HTTP
		switch strings.ToLower(ocReq.Info.Type) {
		case "http":
			// Supported — continue
		case "graphql":
			logger.Warn("skipping graphql request (not supported)", "name", ocReq.Info.Name, "file", filePath)
			continue
		case "ws":
			logger.Warn("skipping websocket request (not supported)", "name", ocReq.Info.Name, "file", filePath)
			continue
		case "grpc":
			logger.Warn("skipping grpc request (not supported)", "name", ocReq.Info.Name, "file", filePath)
			continue
		default:
			logger.Warn("skipping unknown request type", "type", ocReq.Info.Type, "name", ocReq.Info.Name, "file", filePath)
			continue
		}

		// Use seq for ordering if available
		fileOrder := order
		if ocReq.Info.Seq > 0 {
			fileOrder = float64(ocReq.Info.Seq)
		}

		relPath, _ := filepath.Rel(rootPath, filePath)
		convertRequest(ocReq, workspaceID, parentID, fileOrder, now, relPath, bundle)
		order++
	}

	// Process subdirectories
	for _, dirEntry := range dirs {
		subDirPath := filepath.Join(dirPath, dirEntry.Name())
		relPath, _ := filepath.Rel(rootPath, subDirPath)
		pathHash := computePathHash(relPath)

		// Create a folder file entry
		folderID := idwrap.NewNow()
		folderContentID := folderID
		folderFile := mfile.File{
			ID:          folderID,
			WorkspaceID: workspaceID,
			ParentID:    parentID,
			ContentID:   &folderContentID,
			ContentType: mfile.ContentTypeFolder,
			Name:        dirEntry.Name(),
			Order:       order,
			PathHash:    &pathHash,
			UpdatedAt:   time.UnixMilli(now),
		}
		bundle.Files = append(bundle.Files, folderFile)

		// Recurse into subdirectory
		if err := walkCollection(rootPath, subDirPath, &folderID, workspaceID, now, bundle, logger); err != nil {
			return err
		}
		order++
	}

	return nil
}

// convertRequest converts a single OpenCollection request into DevTools models.
func convertRequest(
	ocReq OCRequest,
	workspaceID idwrap.IDWrap,
	parentID *idwrap.IDWrap,
	order float64,
	now int64,
	relPath string,
	bundle *ioworkspace.WorkspaceBundle,
) {
	httpID := idwrap.NewNow()

	method := "GET"
	url := ""
	if ocReq.HTTP != nil {
		method = strings.ToUpper(ocReq.HTTP.Method)
		if method == "" {
			method = "GET"
		}
		url = ocReq.HTTP.URL
	}

	// Determine body kind
	bodyKind := mhttp.HttpBodyKindNone
	if ocReq.HTTP != nil && ocReq.HTTP.Body != nil {
		bodyKind, _, _, _ = convertBody(ocReq.HTTP.Body, httpID)
	}

	// Create HTTP request
	httpReq := mhttp.HTTP{
		ID:          httpID,
		WorkspaceID: workspaceID,
		Name:        ocReq.Info.Name,
		Url:         url,
		Method:      method,
		Description: ocReq.Docs,
		BodyKind:    bodyKind,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	bundle.HTTPRequests = append(bundle.HTTPRequests, httpReq)

	// Create file entry for this request
	contentID := httpID
	fileID := idwrap.NewNow()
	pathHash := computePathHash(relPath)
	file := mfile.File{
		ID:          fileID,
		WorkspaceID: workspaceID,
		ParentID:    parentID,
		ContentID:   &contentID,
		ContentType: mfile.ContentTypeHTTP,
		Name:        ocReq.Info.Name,
		Order:       order,
		PathHash:    &pathHash,
		UpdatedAt:   time.UnixMilli(now),
	}
	bundle.Files = append(bundle.Files, file)

	if ocReq.HTTP == nil {
		return
	}

	// Convert headers
	for i, h := range ocReq.HTTP.Headers {
		bundle.HTTPHeaders = append(bundle.HTTPHeaders, mhttp.HTTPHeader{
			ID:           idwrap.NewNow(),
			HttpID:       httpID,
			Key:          h.Name,
			Value:        h.Value,
			Enabled:      !h.Disabled,
			DisplayOrder: float32(i + 1),
			CreatedAt:    now,
			UpdatedAt:    now,
		})
	}

	// Convert params
	for i, p := range ocReq.HTTP.Params {
		if strings.ToLower(p.Type) == "query" || p.Type == "" {
			bundle.HTTPSearchParams = append(bundle.HTTPSearchParams, mhttp.HTTPSearchParam{
				ID:           idwrap.NewNow(),
				HttpID:       httpID,
				Key:          p.Name,
				Value:        p.Value,
				Enabled:      !p.Disabled,
				DisplayOrder: float64(i + 1),
				CreatedAt:    now,
				UpdatedAt:    now,
			})
		}
		// Path params are embedded in the URL — no separate model
	}

	// Convert auth → headers/params
	authHeaders, authParams := convertAuth(ocReq.HTTP.Auth, httpID)
	bundle.HTTPHeaders = append(bundle.HTTPHeaders, authHeaders...)
	bundle.HTTPSearchParams = append(bundle.HTTPSearchParams, authParams...)

	// Convert body
	_, bodyRaw, bodyForms, bodyUrlencoded := convertBody(ocReq.HTTP.Body, httpID)
	if bodyRaw != nil {
		bodyRaw.CreatedAt = now
		bodyRaw.UpdatedAt = now
		bundle.HTTPBodyRaw = append(bundle.HTTPBodyRaw, *bodyRaw)
	}
	bundle.HTTPBodyForms = append(bundle.HTTPBodyForms, bodyForms...)
	bundle.HTTPBodyUrlencoded = append(bundle.HTTPBodyUrlencoded, bodyUrlencoded...)

	// Convert assertions
	if ocReq.Runtime != nil {
		for i, a := range ocReq.Runtime.Assertions {
			expr := a.Expression
			if a.Operator != "" {
				expr = fmt.Sprintf("%s %s %s", a.Expression, a.Operator, a.Value)
			}
			bundle.HTTPAsserts = append(bundle.HTTPAsserts, mhttp.HTTPAssert{
				ID:           idwrap.NewNow(),
				HttpID:       httpID,
				Value:        strings.TrimSpace(expr),
				Enabled:      true,
				DisplayOrder: float32(i + 1),
				CreatedAt:    now,
				UpdatedAt:    now,
			})
		}
	}
}

// computePathHash returns a SHA-256 hash of the given path for deduplication.
func computePathHash(relPath string) string {
	h := sha256.Sum256([]byte(relPath))
	return hex.EncodeToString(h[:])
}

// isYAMLFile checks if a filename has a YAML extension.
func isYAMLFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".yml" || ext == ".yaml"
}
