# tcurlv2 Package

The `tcurlv2` package provides curl command parsing and conversion functionality using the new HTTP model architecture, replacing the legacy collection-based models used in the original `tcurl` package.

## Purpose

This package converts curl commands into the modern HTTP request model (`mhttp.HTTP`) and integrates with the unified file system (`mfile.File`). It supports workspace and folder hierarchy, as well as the delta system for request variations.

## Key Differences from tcurl

| Feature | tcurl (Legacy) | tcurlv2 (New) |
|---------|----------------|---------------|
| Main Model | `mitemapi.ItemApi` | `mhttp.HTTP` |
| File System | Collection-based | Unified file system with `mfile.File` |
| Context | CollectionID | WorkspaceID + FolderID |
| Headers | `mexampleheader.Header` | `mhttp.HTTPHeader` |
| Body Types | `mbodyraw`, `mbodyform`, `mbodyurl` | `mhttp.HTTPBodyRaw`, `mhttp.HTTPBodyForm`, `mhttp.HTTPBodyUrlencoded` |
| Query Params | `mexamplequery.Query` | `mhttp.HTTPSearchParam` |
| Delta System | Not supported | Full support with `ParentHttpID`, `IsDelta` |
| File Integration | Limited | Full integration with `ContentKind.HTTP` |

## Usage

### Basic Conversion

```go
package main

import (
    "fmt"
    "the-dev-tools/server/pkg/idwrap"
    "the-dev-tools/server/pkg/model/mworkspace"
    "the-dev-tools/server/pkg/translate/tcurlv2"
)

func main() {
    // Create workspace
    workspace := mworkspace.Workspace{
        ID:     idwrap.NewNow(),
        Name:   "My Workspace",
        ActiveEnv: idwrap.NewNow(),
        GlobalEnv: idwrap.NewNow(),
    }

    // Define conversion options
    opts := tcurlv2.ConvertCurlOptions{
        WorkspaceID: workspace.ID,
        // FolderID:    &folderID, // Optional
        // ParentHttpID: &parentID, // For delta system
        // IsDelta:     false,
        Filename:     "create-user", // Optional
    }

    // Convert curl command
    curlCmd := `curl -X POST https://api.example.com/users \
        -H 'Content-Type: application/json' \
        -d '{"name":"John Doe","email":"john@example.com"}'`

    resolved, err := tcurlv2.ConvertCurl(curlCmd, workspace, opts)
    if err != nil {
        panic(err)
    }

    // Access the HTTP request
    fmt.Printf("Method: %s\n", resolved.HTTP.Method)
    fmt.Printf("URL: %s\n", resolved.HTTP.Url)
    fmt.Printf("Name: %s\n", resolved.HTTP.Name)

    // Access headers
    for _, header := range resolved.Headers {
        fmt.Printf("Header: %s: %s\n", header.HeaderKey, header.HeaderValue)
    }

    // Access raw body
    if resolved.BodyRaw != nil {
        fmt.Printf("Body: %s\n", string(resolved.BodyRaw.RawData))
    }
}
```

### Delta System Support

```go
// Create a delta variation of an existing request
opts := tcurlv2.ConvertCurlOptions{
    WorkspaceID: workspace.ID,
    ParentHttpID: &originalRequestID, // Reference to parent HTTP request
    IsDelta:      true,
    DeltaName:    stringPtr("Production Environment"),
    Filename:     "create-user-prod",
}

resolved, err := tcurlv2.ConvertCurl(curlCmd, workspace, opts)
```

### File System Integration

```go
// Create FileWithContent for the unified file system
fileWithContent := tcurlv2.CreateFileWithContent(resolved)

// Validate the file and content
if err := fileWithContent.Validate(); err != nil {
    panic(err)
}

// Access file metadata
fmt.Printf("File ID: %s\n", fileWithContent.File.ID)
fmt.Printf("Content Kind: %s\n", fileWithContent.File.ContentKind.String())

// Access HTTP content
if httpContent, ok := fileWithContent.Content.(*mfile.HTTPAdapter); ok {
    fmt.Printf("HTTP Content Name: %s\n", httpContent.GetName())
}
```

### Building Curl Commands

```go
// Reconstruct curl command from resolved data
curlStr, err := tcurlv2.BuildCurl(resolved)
if err != nil {
    panic(err)
}

fmt.Println(curlStr)
// Output: curl 'https://api.example.com/users' \
//   -X POST \
//   -H 'Content-Type: application/json' \
//   --data-raw '{"name":"John Doe","email":"john@example.com"}'
```

## Supported Curl Features

- **HTTP Methods**: GET, POST, PUT, DELETE, PATCH, HEAD, OPTIONS
- **Headers**: `-H` or `--header` flags
- **Cookies**: `-b` or `--cookie` flags (converted to Cookie headers)
- **Data Types**:
  - Raw data: `-d`, `--data`, `--data-raw`, `--data-binary`
  - URL-encoded: `--data-urlencode`
  - Form data: `-F` or `--form`
- **Query Parameters**: Extracted from URLs
- **Multi-line**: Support for line continuations with `\`

## Data Structures

### CurlResolvedV2

```go
type CurlResolvedV2 struct {
    // Primary HTTP request
    HTTP mhttp.HTTP

    // Associated data structures
    SearchParams    []mhttp.HTTPSearchParam
    Headers        []mhttp.HTTPHeader
    BodyForms      []mhttp.HTTPBodyForm
    BodyUrlencoded []mhttp.HTTPBodyUrlencoded
    BodyRaw        *mhttp.HTTPBodyRaw

    // File system integration
    File      mfile.File
    Workspace mworkspace.Workspace
}
```

### ConvertCurlOptions

```go
type ConvertCurlOptions struct {
    WorkspaceID    idwrap.IDWrap
    FolderID       *idwrap.IDWrap    // Optional parent folder
    ParentHttpID   *idwrap.IDWrap    // For delta system
    IsDelta        bool             // Whether this is a delta variation
    DeltaName      *string          // Optional delta name
    Filename       string           // Optional filename
}
```

## Migration from tcurl

When migrating from the original `tcurl` package:

1. **Replace return types**: Use `CurlResolvedV2` instead of `CurlResolved`
2. **Update options**: Use `ConvertCurlOptions` instead of direct `collectionID` parameter
3. **Handle workspace**: Provide workspace context instead of collection context
4. **File integration**: Use `CreateFileWithContent()` to work with the unified file system
5. **Delta support**: Leverage `ParentHttpID` and `IsDelta` for request variations

## Testing

The package includes comprehensive tests:

```bash
# Run all tests
go test ./packages/server/pkg/translate/tcurlv2/...

# Run with coverage
go test -cover ./packages/server/pkg/translate/tcurlv2/...
```

## Examples

See the test files for detailed usage examples covering:
- Simple GET requests
- POST requests with headers and bodies
- Form data uploads
- URL-encoded parameters
- Cookie handling
- Delta system integration

## Dependencies

- `the-dev-tools/server/pkg/compress` - For data compression handling
- `the-dev-tools/server/pkg/idwrap` - For ID generation and management
- `the-dev-tools/server/pkg/model/mfile` - For file system integration
- `the-dev-tools/server/pkg/model/mhttp` - For HTTP request models
- `the-dev-tools/server/pkg/model/mworkspace` - For workspace context