package workflow

// Format represents the supported serialization formats
type Format int

const (
	FormatUnspecified Format = iota
	FormatYAML
	FormatJSON
	FormatTOML
)

func (f Format) String() string {
	switch f {
	case FormatYAML:
		return "yaml"
	case FormatJSON:
		return "json"
	case FormatTOML:
		return "toml"
	default:
		return "unspecified"
	}
}