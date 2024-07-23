# Install

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.31.0
export PATH="$PATH:$(go env GOPATH)/bin"
```

# Setup

- Add git submodule to your repo
- Add buf.gen.yaml to your repo

# Usage

## Generate

```bash
buf generate
```
