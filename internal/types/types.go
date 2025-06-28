package types

// Secret represents a single secret configuration
type Secret struct {
	Path      string `json:"path"`
	Reference string `json:"reference"`
	Owner     string `json:"owner,omitempty"`
	Group     string `json:"group,omitempty"`
	Mode      string `json:"mode,omitempty"`
}

// Config represents the complete secrets configuration
type Config struct {
	Secrets []Secret `json:"secrets"`
}
