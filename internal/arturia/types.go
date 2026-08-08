package arturia

import "time"

// Metadata describes the user-visible identity stored inside a Pigments preset.
type Metadata struct {
	Name   string `json:"name"`
	Bank   string `json:"bank"`
	Author string `json:"author"`
}

// ParameterChange is a single safe mutation requested by the planning model.
// Value is intentionally a string so enum names and numeric values share one schema.
type ParameterChange struct {
	ParameterID string `json:"parameter_id"`
	Operation   string `json:"operation"` // set, add, multiply, toggle
	Value       string `json:"value"`
	Unit        string `json:"unit"` // normalized, raw, percent, hz, db, semitones, bits, enum, boolean
	Reason      string `json:"reason,omitempty"`
	AllowAdd    bool   `json:"allow_add,omitempty"`
}

// MacroNames contains optional 16-byte Pigments macro labels.
type MacroNames struct {
	Macro1 string `json:"macro1,omitempty"`
	Macro2 string `json:"macro2,omitempty"`
	Macro3 string `json:"macro3,omitempty"`
	Macro4 string `json:"macro4,omitempty"`
}

// PresetPlan is the schema-locked response produced by the planning model and consumed by the deterministic preset editor.
type PresetPlan struct {
	PatchName string            `json:"patch_name"`
	Summary   string            `json:"summary"`
	Macros    MacroNames        `json:"macro_names"`
	Changes   []ParameterChange `json:"changes"`
	Warnings  []string          `json:"warnings"`

	// BankOverride and AuthorOverride are server-controlled fields. They are
	// deliberately excluded from model JSON so untrusted prompts cannot choose
	// privileged filesystem paths or impersonate an official sound bank.
	BankOverride   string `json:"-"`
	AuthorOverride string `json:"-"`
}

// AppliedChange records exactly what changed in the serialized preset.
type AppliedChange struct {
	RequestedID string `json:"requested_id"`
	ParameterID string `json:"parameter_id"`
	Operation   string `json:"operation"`
	Unit        string `json:"unit"`
	OldRaw      string `json:"old_raw,omitempty"`
	NewRaw      string `json:"new_raw"`
	OldDisplay  string `json:"old_display,omitempty"`
	NewDisplay  string `json:"new_display,omitempty"`
	Reason      string `json:"reason,omitempty"`
	Added       bool   `json:"added,omitempty"`
	Approximate bool   `json:"approximate,omitempty"`
}

// ApplyReport is returned by generation and modification operations.
type ApplyReport struct {
	SourcePath string          `json:"source_path,omitempty"`
	OutputPath string          `json:"output_path"`
	Metadata   Metadata        `json:"metadata"`
	Summary    string          `json:"summary,omitempty"`
	Changes    []AppliedChange `json:"changes"`
	Warnings   []string        `json:"warnings,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
}

// ParameterView is a compact parameter representation for UI and MCP inspection.
type ParameterView struct {
	ID           string `json:"id"`
	RawValue     string `json:"raw_value"`
	DisplayValue string `json:"display_value,omitempty"`
	FriendlyName string `json:"friendly_name,omitempty"`
	Description  string `json:"description,omitempty"`
	Unit         string `json:"unit,omitempty"`
}

// PresetInspection summarizes a .pgtx file without modifying it.
type PresetInspection struct {
	Path           string          `json:"path,omitempty"`
	Metadata       Metadata        `json:"metadata"`
	InnerPath      string          `json:"inner_path"`
	ParameterCount int             `json:"parameter_count"`
	Parameters     []ParameterView `json:"parameters,omitempty"`
	ArchiveEntries []string        `json:"archive_entries,omitempty"`
	Warnings       []string        `json:"warnings,omitempty"`
}

// ParameterSpec documents a known Pigments parameter and optional display conversion.
type ParameterSpec struct {
	ID          string            `json:"id"`
	Friendly    string            `json:"friendly"`
	Aliases     []string          `json:"aliases,omitempty"`
	Unit        string            `json:"unit"` // normalized, hz, db, semitones, bits, enum, boolean, raw
	Min         float64           `json:"min,omitempty"`
	Max         float64           `json:"max,omitempty"`
	Curve       string            `json:"curve,omitempty"` // linear or log
	EnumValues  map[string]string `json:"enum_values,omitempty"`
	Description string            `json:"description,omitempty"`
}

// ParameterDifference records a serialized value that differs between two
// preset snapshots. It powers the research workflow used to map Pigments UI
// controls to stable .pgtx parameter IDs.
type ParameterDifference struct {
	ParameterID   string `json:"parameter_id"`
	BeforeRaw     string `json:"before_raw,omitempty"`
	AfterRaw      string `json:"after_raw,omitempty"`
	BeforeDisplay string `json:"before_display,omitempty"`
	AfterDisplay  string `json:"after_display,omitempty"`
	FriendlyName  string `json:"friendly_name,omitempty"`
	Unit          string `json:"unit,omitempty"`
	Status        string `json:"status"` // changed, added, removed
}

// PresetDiff is a safe, compact comparison of two .pgtx files.
type PresetDiff struct {
	BeforeMetadata Metadata              `json:"before_metadata"`
	AfterMetadata  Metadata              `json:"after_metadata"`
	BeforeCount    int                   `json:"before_parameter_count"`
	AfterCount     int                   `json:"after_parameter_count"`
	ChangeCount    int                   `json:"change_count"`
	Changes        []ParameterDifference `json:"changes"`
	Truncated      bool                  `json:"truncated"`
	Warnings       []string              `json:"warnings,omitempty"`
}
