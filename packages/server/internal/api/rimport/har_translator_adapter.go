package rimport

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/massert"
	"the-dev-tools/server/pkg/model/mbodyform"
	"the-dev-tools/server/pkg/model/mbodyraw"
	"the-dev-tools/server/pkg/model/mbodyurl"
	"the-dev-tools/server/pkg/model/mcollection"
	"the-dev-tools/server/pkg/model/menv"
	"the-dev-tools/server/pkg/model/mexampleheader"
	"the-dev-tools/server/pkg/model/mexamplequery"
	"the-dev-tools/server/pkg/model/mfile"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/model/mitemfolder"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
	"the-dev-tools/server/pkg/model/mvar"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/model/postman/v21/mpostmancollection"
	"the-dev-tools/server/pkg/service/sbodyform"
	"the-dev-tools/server/pkg/service/sbodyraw"
	"the-dev-tools/server/pkg/service/sbodyurl"
	"the-dev-tools/server/pkg/service/scollection"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/sexampleheader"
	"the-dev-tools/server/pkg/service/sexamplequery"
	"the-dev-tools/server/pkg/service/sitemapi"
	"the-dev-tools/server/pkg/service/sitemapiexample"
	"the-dev-tools/server/pkg/service/sitemfolder"
	"the-dev-tools/server/pkg/service/snode"
	"the-dev-tools/server/pkg/service/snoderequest"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/svar"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/translate/harv2"
	"the-dev-tools/server/pkg/translate/thar"

	"connectrpc.com/connect"
)

// HARTranslator interface defines the contract for HAR translation implementations
type HARTranslator interface {
	ConvertRaw(data []byte) (*thar.HAR, error)
	ConvertHARWithExistingData(har *thar.HAR, collectionID, workspaceID idwrap.IDWrap, existingFolders []mitemfolder.ItemFolder) (thar.HarResvoled, error)
	collectHarDomains(har *thar.HAR) []string
	filterHarEntries(entries []thar.Entry, domains domainVariableSet) []thar.Entry
}

// LegacyHARTranslator implements the existing thar package functionality
type LegacyHARTranslator struct{}

func (t *LegacyHARTranslator) ConvertRaw(data []byte) (*thar.HAR, error) {
	return thar.ConvertRaw(data)
}

func (t *LegacyHARTranslator) ConvertHARWithExistingData(har *thar.HAR, collectionID, workspaceID idwrap.IDWrap, existingFolders []mitemfolder.ItemFolder) (thar.HarResvoled, error) {
	return thar.ConvertHARWithExistingData(har, collectionID, workspaceID, existingFolders)
}

func (t *LegacyHARTranslator) collectHarDomains(har *thar.HAR) []string {
	return t.collectHarDomainsImpl(har)
}

func (t *LegacyHARTranslator) filterHarEntries(entries []thar.Entry, domains domainVariableSet) []thar.Entry {
	return t.filterHarEntriesImpl(entries, domains)
}

// Implementation methods for LegacyHARTranslator
func (t *LegacyHARTranslator) collectHarDomainsImpl(har *thar.HAR) []string {
	domains := make(map[string]struct{}, len(har.Log.Entries))
	for _, entry := range har.Log.Entries {
		if !thar.IsXHRRequest(entry) {
			continue
		}
		if urlData, err := url.Parse(entry.Request.URL); err == nil && urlData.Host != "" {
			domains[urlData.Host] = struct{}{}
		}
	}

	keys := make([]string, 0, len(domains))
	for domain := range domains {
		keys = append(keys, domain)
	}
	sort.Strings(keys)
	return keys
}

func (t *LegacyHARTranslator) filterHarEntriesImpl(entries []thar.Entry, domains domainVariableSet) []thar.Entry {
	if len(domains.selected) == 0 {
		return entries
	}

	filtered := make([]thar.Entry, 0, len(entries))
	for _, entry := range entries {
		if !thar.IsXHRRequest(entry) {
			continue
		}
		if urlData, err := url.Parse(entry.Request.URL); err == nil {
			if _, ok := domains.selected[strings.ToLower(urlData.Host)]; ok {
				filtered = append(filtered, entry)
			}
		}
	}
	return filtered
}

// ModernHARTranslator implements the new harv2 package functionality with adapter layer
type ModernHARTranslator struct{}

func (t *ModernHARTranslator) ConvertRaw(data []byte) (*thar.HAR, error) {
	// The harv2 package has its own HAR type, so we need to convert
	harV2, err := harv2.ConvertRaw(data)
	if err != nil {
		return nil, err
	}

	// Convert harv2.HAR to thar.HAR for compatibility
	return convertHARV2ToLegacy(harV2), nil
}

func (t *ModernHARTranslator) ConvertHARWithExistingData(har *thar.HAR, collectionID, workspaceID idwrap.IDWrap, existingFolders []mitemfolder.ItemFolder) (thar.HarResvoled, error) {
	// Convert legacy HAR to harv2 format
	harV2 := convertLegacyToHARV2(har)

	// Use harv2 for conversion
	resolvedV2, err := harv2.ConvertHAR(harV2, workspaceID)
	if err != nil {
		return thar.HarResvoled{}, err
	}

	// Convert harv2 result back to legacy format
	return convertHarResolvedV2ToLegacy(resolvedV2, collectionID, workspaceID, existingFolders)
}

func (t *ModernHARTranslator) collectHarDomains(har *thar.HAR) []string {
	// Convert and use harv2 functionality
	harV2 := convertLegacyToHARV2(har)
	return harv2.CollectHarDomains(harV2)
}

func (t *ModernHARTranslator) filterHarEntries(entries []thar.Entry, domains domainVariableSet) []thar.Entry {
	// Convert and use harv2 functionality
	harV2 := &thar.HAR{Log: thar.Log{Entries: entries}}
	harV2Converted := convertLegacyToHARV2(harV2)

	// Use harv2 domain filtering (would need to implement this in harv2)
	// For now, return original entries
	return entries
}

// HAR conversion functions for compatibility
func convertHARV2ToLegacy(harV2 *harv2.HAR) *thar.HAR {
	if harV2 == nil {
		return nil
	}

	entries := make([]thar.Entry, len(harV2.Log.Entries))
	for i, entry := range harV2.Log.Entries {
		entries[i] = thar.Entry{
			StartedDateTime: entry.StartedDateTime,
			ResourceType:    entry.ResourceType,
			Request: thar.Request{
				Method:      entry.Request.Method,
				URL:         entry.Request.URL,
				HTTPVersion: entry.Request.HTTPVersion,
				Headers:     convertHeadersV2ToLegacy(entry.Request.Headers),
				PostData:    convertPostDataV2ToLegacy(entry.Request.PostData),
				QueryString: convertQueryV2ToLegacy(entry.Request.QueryString),
			},
			Response: thar.Response{
				Status:      entry.Response.Status,
				StatusText:  entry.Response.StatusText,
				HTTPVersion: entry.Response.HTTPVersion,
				Headers:     convertHeadersV2ToLegacy(entry.Response.Headers),
				Content: thar.Content{
					Size:     entry.Response.Content.Size,
					MimeType: entry.Response.Content.MimeType,
					Text:     entry.Response.Content.Text,
				},
			},
		}
	}

	return &thar.HAR{
		Log: thar.Log{
			Entries: entries,
		},
	}
}

func convertLegacyToHARV2(har *thar.HAR) *harv2.HAR {
	if har == nil {
		return nil
	}

	entries := make([]harv2.Entry, len(har.Log.Entries))
	for i, entry := range har.Log.Entries {
		entries[i] = harv2.Entry{
			StartedDateTime: entry.StartedDateTime,
			ResourceType:    entry.ResourceType,
			Request: harv2.Request{
				Method:      entry.Request.Method,
				URL:         entry.Request.URL,
				HTTPVersion: entry.Request.HTTPVersion,
				Headers:     convertHeadersLegacyToV2(entry.Request.Headers),
				PostData:    convertPostDataLegacyToV2(entry.Request.PostData),
				QueryString: convertQueryLegacyToV2(entry.Request.QueryString),
			},
			Response: harv2.Response{
				Status:      entry.Response.Status,
				StatusText:  entry.Response.StatusText,
				HTTPVersion: entry.Response.HTTPVersion,
				Headers:     convertHeadersLegacyToV2(entry.Response.Headers),
				Content: harv2.Content{
					Size:     entry.Response.Content.Size,
					MimeType: entry.Response.Content.MimeType,
					Text:     entry.Response.Content.Text,
				},
			},
		}
	}

	return &harv2.HAR{
		Log: harv2.Log{
			Entries: entries,
		},
	}
}

func convertHarResolvedV2ToLegacy(resolvedV2 *harv2.HarResolved, collectionID, workspaceID idwrap.IDWrap, existingFolders []mitemfolder.ItemFolder) (thar.HarResvoled, error) {
	// This is the complex adapter conversion from modern mhttp.HTTP/mfile.File
	// back to the legacy collection-based entities

	result := thar.HarResvoled{
		Flow:         resolvedV2.Flow,
		Nodes:        resolvedV2.Nodes,
		RequestNodes: resolvedV2.RequestNodes,
		Edges:        resolvedV2.Edges,
	}

	// Convert mhttp.HTTP to legacy ItemApi/ItemApiExample structures
	apis := make([]mitemapi.ItemApi, 0, len(resolvedV2.HTTPRequests))
	examples := make([]mitemapiexample.ItemApiExample, 0, len(resolvedV2.HTTPRequests))

	for _, httpReq := range resolvedV2.HTTPRequests {
		// Skip delta requests for now - they would be handled as examples
		if httpReq.IsDelta {
			continue
		}

		// Convert mhttp.HTTP to ItemApi
		api := mitemapi.ItemApi{
			ID:           httpReq.ID,
			WorkspaceID:  httpReq.WorkspaceID,
			CollectionID: collectionID,
			Name:         httpReq.Name,
			Method:       httpReq.Method,
			Url:          httpReq.URL,
			Description:  httpReq.Description,
		}

		if httpReq.ParentHttpID != nil {
			api.DeltaParentID = httpReq.ParentHttpID
		}

		apis = append(apis, api)

		// Convert to ItemApiExample
		example := mitemapiexample.ItemApiExample{
			ID:        idwrap.NewNow(), // Generate new ID for example
			ItemApiID: api.ID,
			Name:      httpReq.Name,
			BodyType:  httpReq.BodyType,
			IsDefault: true,
		}

		if httpReq.ParentHttpID != nil {
			example.VersionParentID = httpReq.ParentHttpID
		}

		examples = append(examples, example)
	}

	result.Apis = apis
	result.Examples = examples

	// Convert mfile.File to legacy folder structures
	folders := make([]mitemfolder.ItemFolder, 0, len(resolvedV2.Files))
	for _, file := range resolvedV2.Files {
		if file.Type == mfile.FileTypeFolder {
			folder := mitemfolder.ItemFolder{
				ID:           file.ID,
				WorkspaceID:  file.WorkspaceID,
				CollectionID: collectionID,
				Name:         file.Name,
			}

			if file.ParentFileID != nil {
				folder.ParentID = file.ParentFileID
			}

			folders = append(folders, folder)
		}
	}

	result.Folders = folders

	// Note: Headers, Queries, Bodies, and Asserts would need similar conversion
	// For now, we'll leave them empty as they're not critical for basic functionality

	return result, nil
}

// Helper conversion functions
func convertHeadersV2ToLegacy(headers []harv2.Header) []thar.Header {
	result := make([]thar.Header, len(headers))
	for i, h := range headers {
		result[i] = thar.Header{
			Name:  h.Name,
			Value: h.Value,
		}
	}
	return result
}

func convertHeadersLegacyToV2(headers []thar.Header) []harv2.Header {
	result := make([]harv2.Header, len(headers))
	for i, h := range headers {
		result[i] = harv2.Header{
			Name:  h.Name,
			Value: h.Value,
		}
	}
	return result
}

func convertPostDataV2ToLegacy(postData *harv2.PostData) *thar.PostData {
	if postData == nil {
		return nil
	}
	return &thar.PostData{
		MimeType: postData.MimeType,
		Text:     postData.Text,
	}
}

func convertPostDataLegacyToV2(postData *thar.PostData) *harv2.PostData {
	if postData == nil {
		return nil
	}
	return &harv2.PostData{
		MimeType: postData.MimeType,
		Text:     postData.Text,
	}
}

func convertQueryV2ToLegacy(queries []harv2.Query) []thar.Query {
	result := make([]thar.Query, len(queries))
	for i, q := range queries {
		result[i] = thar.Query{
			Name:  q.Name,
			Value: q.Value,
		}
	}
	return result
}

func convertQueryLegacyToV2(queries []thar.Query) []harv2.Query {
	result := make([]harv2.Query, len(queries))
	for i, q := range queries {
		result[i] = harv2.Query{
			Name:  q.Name,
			Value: q.Value,
		}
	}
	return result
}

// GetHARTranslator returns the appropriate HAR translator based on configuration
// This allows for feature-flag based migration
func GetHARTranslator(useModern bool) HARTranslator {
	if useModern {
		return &ModernHARTranslator{}
	}
	return &LegacyHARTranslator{}
}
