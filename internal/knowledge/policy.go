package knowledge

import (
	"sort"
	"strings"
	"sync"
)

// ParameterEditPolicy records whether an internal .pgtx parameter ID is
// governed by the master-database mapping overlay and whether at least one
// documented UI relationship is approved for conservative automatic editing.
//
// A transformed mapping (for example visible Engine Power -> stored Bypass)
// does not govern the direct stored parameter ID. The direct parameter may
// still have independently curated semantics in the compiler.
type ParameterEditPolicy struct {
	ParameterID      string   `json:"parameter_id"`
	Governed         bool     `json:"governed"`
	AutomaticEdit    bool     `json:"automatic_edit"`
	ControlIDs       []string `json:"control_ids,omitempty"`
	MappingStatuses  []string `json:"mapping_statuses,omitempty"`
	ConversionStates []string `json:"conversion_statuses,omitempty"`
}

var (
	parameterPolicyOnce sync.Once
	parameterPolicies   map[string]ParameterEditPolicy
)

func buildParameterPolicies() {
	parameterPolicies = map[string]ParameterEditPolicy{}
	catalog, _, err := loadRuntime()
	if err != nil {
		return
	}
	type accumulator struct {
		allowed     bool
		controls    map[string]bool
		statuses    map[string]bool
		conversions map[string]bool
	}
	acc := map[string]*accumulator{}
	for _, section := range catalog.Sections {
		for _, control := range section.Parameters {
			for _, target := range control.MappingTargets {
				id := strings.TrimSpace(target.ParameterID)
				if id == "" || target.Legacy || strings.TrimSpace(target.Transform) != "" {
					continue
				}
				item := acc[id]
				if item == nil {
					item = &accumulator{controls: map[string]bool{}, statuses: map[string]bool{}, conversions: map[string]bool{}}
					acc[id] = item
				}
				item.allowed = item.allowed || control.AutomaticEdit
				item.controls[control.ControlID] = true
				if value := strings.TrimSpace(control.MappingStatus); value != "" {
					item.statuses[value] = true
				}
				if value := strings.TrimSpace(control.ConversionStatus); value != "" {
					item.conversions[value] = true
				}
			}
		}
	}
	for id, item := range acc {
		policy := ParameterEditPolicy{
			ParameterID:      id,
			Governed:         true,
			AutomaticEdit:    item.allowed,
			ControlIDs:       sortedKeys(item.controls),
			MappingStatuses:  sortedKeys(item.statuses),
			ConversionStates: sortedKeys(item.conversions),
		}
		parameterPolicies[id] = policy
	}
}

func sortedKeys(values map[string]bool) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

// PolicyForParameter returns a mapping-overlay policy when the internal ID is
// covered by at least one non-transformed master mapping.
func PolicyForParameter(parameterID string) (ParameterEditPolicy, bool) {
	parameterPolicyOnce.Do(buildParameterPolicies)
	policy, ok := parameterPolicies[strings.TrimSpace(parameterID)]
	return policy, ok
}

// AutomaticEditAllowed returns true for IDs outside the overlay (the existing
// curated compiler policy remains authoritative) and for overlay-governed IDs
// explicitly approved for automatic editing. It returns false only when the
// master overlay deliberately calibration-locks that ID.
func AutomaticEditAllowed(parameterID string) bool {
	policy, governed := PolicyForParameter(parameterID)
	return !governed || policy.AutomaticEdit
}
