package arturia

import (
	"encoding/json"
	"testing"
)

func TestGeneratedMasterSpecsAreUniqueAndPresentInDefaultTemplate(t *testing.T) {
	var payload masterParameterSpecFile
	if err := json.Unmarshal(rawMasterParameterSpecs, &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Specs) != 152 {
		t.Fatalf("generated master specs=%d, want 152", len(payload.Specs))
	}

	preset, err := NewFromDefault()
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]bool, len(payload.Specs))
	for _, spec := range payload.Specs {
		if spec.ID == "" {
			t.Fatal("generated master spec has empty ID")
		}
		if seen[spec.ID] {
			t.Fatalf("duplicate generated master spec %q", spec.ID)
		}
		seen[spec.ID] = true
		if _, ok := preset.block.params[spec.ID]; !ok {
			t.Fatalf("generated master spec %q is not present in the default template", spec.ID)
		}
	}
}

func TestCombinedKnownParameterCatalogHasUniqueIDs(t *testing.T) {
	seen := make(map[string]bool, len(KnownParameterSpecs))
	for _, spec := range KnownParameterSpecs {
		if seen[spec.ID] {
			t.Fatalf("duplicate known parameter spec %q", spec.ID)
		}
		seen[spec.ID] = true
	}
}

func TestEnumDisplayUsesDeterministicCanonicalAlias(t *testing.T) {
	cases := map[string]struct {
		id   string
		raw  float64
		want string
	}{
		"sample engine type": {id: "Engine1_ModuleType", raw: 0.5, want: "sample (0.5)"},
		"bit crush shaper":   {id: "Engine1_SampleGranularOsc_Effect Type", raw: 0.8, want: "bit crush (0.8)"},
		"tape echo effect":   {id: "FX1_ModuleType", raw: 0.53846157, want: "tape echo (0.53846157)"},
		"saw waveform":       {id: "LFO1_Waveform", raw: 0.66666669, want: "saw (0.66666669)"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := displayValue(tc.id, tc.raw); got != tc.want {
				t.Fatalf("displayValue(%q, %v)=%q, want %q", tc.id, tc.raw, got, tc.want)
			}
		})
	}
}
