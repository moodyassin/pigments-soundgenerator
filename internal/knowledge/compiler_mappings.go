package knowledge

// compilerIDsForControl returns only the canonical IDs admitted by the
// generated mapping overlay for conservative automatic editing. Candidate,
// conditional, inverse-semantics, UI-only, and asset/object mappings are
// intentionally excluded.
func compilerIDsForControl(controlID string) []string {
	parameter, ok := MappingForControl(controlID)
	if !ok || !parameter.AutomaticEdit || len(parameter.CanonicalIDs) == 0 {
		return nil
	}
	return append([]string(nil), parameter.CanonicalIDs...)
}
