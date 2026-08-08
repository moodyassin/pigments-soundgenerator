package knowledge

import (
	"encoding/json"
	"sort"
	"strings"
)

// Catalog is the generated, confidence-rated runtime view of the user's
// Pigments 7 master database. The complete raw evidence remains external to
// this compact structure and is represented by the bounded internal index.
type Catalog = RuntimeMasterCatalog

type Section = RuntimeSection

type Parameter = RuntimeDocumentedControl

func Load() Catalog {
	catalog, _, err := loadRuntime()
	if err != nil {
		return Catalog{}
	}
	return *catalog
}

func JSON() []byte {
	return PublicJSON()
}

// PublicJSON returns the generated runtime catalog plus a browser-safe summary
// of the master evidence database. The 3,525 raw IDs and 15,975 value rows are
// searched server-side rather than bulk-transferred to every client.
func PublicJSON() []byte {
	catalog := Load()
	masterPublic := PublicMasterKnowledge()
	categories := make([]string, 0, len(catalog.SampleBrowser.Banks))
	for _, bank := range catalog.SampleBrowser.Banks {
		categories = append(categories, bank.DisplayName)
	}
	names := make([]string, 0, len(catalog.SampleBrowser.VisibleEntries))
	for _, item := range catalog.SampleBrowser.VisibleEntries {
		names = append(names, item.DisplayName)
	}
	payload := map[string]any{
		"schema_version":  catalog.SchemaVersion,
		"product":         catalog.Product,
		"target_version":  catalog.TargetVersion,
		"source_database": catalog.SourceDatabase,
		"statistics":      catalog.Statistics,
		"notes":           catalog.Notes,
		"sections":        catalog.Sections,
		"sample_browser":  catalog.SampleBrowser,
		// Compatibility field retained for the existing browser UI.
		"manual_visible_sample_browser": map[string]any{
			"source":               "Pigments master database manual-visible excerpt",
			"exhaustive":           catalog.SampleBrowser.Exhaustive,
			"categories_visible":   categories,
			"sample_names_visible": names,
		},
		"master_database": masterPublic,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return []byte(`{"ready":false,"error":"knowledge serialization failed"}`)
	}
	return data
}

// MappingForControl returns the generated mapping overlay for one UI control.
// AutomaticEdit and ConversionStatus must still be honored by callers.
func MappingForControl(controlID string) (Parameter, bool) {
	needle := strings.TrimSpace(controlID)
	if needle == "" {
		return Parameter{}, false
	}
	for _, section := range Load().Sections {
		for _, parameter := range section.Parameters {
			if parameter.ControlID == needle {
				return parameter, true
			}
		}
	}
	return Parameter{}, false
}

func Search(query string, limit int) []Parameter {
	if limit <= 0 {
		limit = 50
	}
	if limit > 300 {
		limit = 300
	}
	q := strings.ToLower(strings.TrimSpace(query))
	tokens := meaningfulSearchTokens(q)
	type scored struct {
		parameter Parameter
		score     int
	}
	var found []scored
	catalog := Load()
	for _, section := range catalog.Sections {
		for _, parameter := range section.Parameters {
			dependencyText := make([]string, 0, len(parameter.Dependencies)*3)
			for _, dependency := range parameter.Dependencies {
				dependencyText = append(dependencyText, dependency.Condition, dependency.Effect, dependency.Status)
			}
			mappingText := make([]string, 0, len(parameter.MappingTargets)*4)
			for _, target := range parameter.MappingTargets {
				mappingText = append(mappingText, target.ParameterID, target.Role, target.EngineSlot, target.Condition)
			}
			text := strings.ToLower(strings.Join([]string{
				section.Label, parameter.ControlID, parameter.UIName, parameter.NormalizedName,
				parameter.Description, parameter.MappingNotes, parameter.MappingStatus,
				parameter.MappingConfidence, parameter.ConversionStatus,
				strings.Join(parameter.CanonicalIDs, " "), strings.Join(parameter.Values, " "),
				strings.Join(parameter.Aliases, " "), strings.Join(dependencyText, " "),
				strings.Join(mappingText, " "),
			}, " "))
			score := searchScore(q, tokens, text, parameter.ControlID, parameter.UIName)
			if q == "" {
				score = 1
			}
			if score > 0 {
				found = append(found, scored{parameter: parameter, score: score})
			}
		}
	}
	sort.SliceStable(found, func(i, j int) bool {
		if found[i].score != found[j].score {
			return found[i].score > found[j].score
		}
		return found[i].parameter.UIName < found[j].parameter.UIName
	})
	if len(found) > limit {
		found = found[:limit]
	}
	result := make([]Parameter, len(found))
	for i := range found {
		result[i] = found[i].parameter
	}
	return result
}

func PromptSummary(query string, limit int) string {
	catalog := Load()
	masterLimit := limit
	if masterLimit <= 0 || masterLimit > 60 {
		masterLimit = 60
	}
	compact := map[string]any{
		"target_version":          catalog.TargetVersion,
		"source_database":         catalog.SourceDatabase,
		"mapping_statistics":      catalog.Statistics,
		"notes":                   catalog.Notes,
		"relevant_ui_mappings":    Search(query, limit),
		"master_database_context": MasterPromptSummary(query, masterLimit),
		"sample_browser":          catalog.SampleBrowser,
		"planner_rules": []string{
			"Only records with automatic_edit=true may be proposed as deterministic write targets.",
			"A high-confidence name match does not prove an exact displayed-unit conversion curve.",
			"Never select asset or serialized-object mappings until their object layer is calibrated.",
		},
	}
	data, _ := json.MarshalIndent(compact, "", "  ")
	return string(data)
}
