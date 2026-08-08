package planner

import (
	"errors"
	"fmt"
	"strings"

	"github.com/audioprompters/pigments-web-mvp/internal/arturia"
	"github.com/audioprompters/pigments-web-mvp/internal/knowledge"
)

func ValidatePlan(plan *arturia.PresetPlan, mode string) error {
	plan.PatchName = strings.TrimSpace(plan.PatchName)
	plan.Summary = strings.TrimSpace(plan.Summary)
	if mode == "generate" && plan.PatchName == "" {
		return errors.New("model returned an empty patch name")
	}
	if len(plan.Changes) == 0 {
		return errors.New("model returned no parameter changes")
	}
	if len(plan.Changes) > 220 {
		return errors.New("model returned too many parameter changes")
	}
	allowedOps := map[string]bool{"set": true, "add": true, "multiply": true, "toggle": true}
	allowedUnits := map[string]bool{"normalized": true, "raw": true, "percent": true, "hz": true, "db": true, "semitones": true, "bits": true, "enum": true, "boolean": true}
	for i := range plan.Changes {
		change := &plan.Changes[i]
		change.ParameterID = arturia.CanonicalParameterID(change.ParameterID)
		change.Operation = strings.ToLower(strings.TrimSpace(change.Operation))
		change.Unit = strings.ToLower(strings.TrimSpace(change.Unit))
		change.Value = strings.TrimSpace(change.Value)
		change.Reason = strings.TrimSpace(change.Reason)
		if change.ParameterID == "" || change.Value == "" || !allowedOps[change.Operation] || !allowedUnits[change.Unit] {
			return fmt.Errorf("model returned an invalid change at index %d", i)
		}
		if !knowledge.AutomaticEditAllowed(change.ParameterID) {
			policy, _ := knowledge.PolicyForParameter(change.ParameterID)
			return fmt.Errorf("parameter %q is calibration-locked by the Pigments master mapping overlay (controls: %s; status: %s)", change.ParameterID, strings.Join(policy.ControlIDs, ", "), strings.Join(policy.MappingStatuses, ", "))
		}
		if change.AllowAdd && !strings.HasPrefix(change.ParameterID, "Modulations_") {
			change.AllowAdd = false
		}
	}
	return nil
}
