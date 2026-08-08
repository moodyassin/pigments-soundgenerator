package arturia

import (
	"bytes"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

const firstParameterMarker = "36 AfterTouchCurve_LastActivePointIndex"

type parameterBlock struct {
	beforeCount []byte
	afterBlock  []byte
	order       []string
	params      map[string]string
}

func parseParameterBlock(data []byte) (*parameterBlock, error) {
	idxFirst := bytes.Index(data, []byte(firstParameterMarker))
	if idxFirst < 0 {
		return nil, fmt.Errorf("Pigments parameter block marker not found")
	}
	idxSuffix := bytes.LastIndex(data[:idxFirst], []byte(" 0 0 0 "))
	if idxSuffix < 0 {
		return nil, fmt.Errorf("Pigments parameter-count suffix not found")
	}
	idxSpace := bytes.LastIndexByte(data[:idxSuffix], ' ')
	if idxSpace < 0 {
		return nil, fmt.Errorf("Pigments parameter count prefix not found")
	}
	countText := strings.TrimSpace(string(data[idxSpace+1 : idxSuffix]))
	count, err := strconv.Atoi(countText)
	if err != nil || count <= 0 || count > 20000 {
		return nil, fmt.Errorf("invalid Pigments parameter count %q", countText)
	}

	params := make(map[string]string, count)
	order := make([]string, 0, count)
	pos := idxFirst
	for i := 0; i < count; i++ {
		space := bytes.IndexByte(data[pos:], ' ')
		if space < 0 {
			return nil, fmt.Errorf("truncated parameter length at item %d", i)
		}
		space += pos
		idLen, err := strconv.Atoi(string(data[pos:space]))
		if err != nil || idLen <= 0 || idLen > 1024 {
			return nil, fmt.Errorf("invalid parameter ID length at item %d", i)
		}
		idStart := space + 1
		idEnd := idStart + idLen
		if idEnd >= len(data) {
			return nil, fmt.Errorf("truncated parameter ID at item %d", i)
		}
		id := string(data[idStart:idEnd])
		if data[idEnd] != ' ' {
			return nil, fmt.Errorf("malformed parameter separator for %q", id)
		}
		valueStart := idEnd + 1
		valueEndRel := bytes.IndexByte(data[valueStart:], ' ')
		if valueEndRel < 0 {
			return nil, fmt.Errorf("truncated value for parameter %q", id)
		}
		valueEnd := valueStart + valueEndRel
		value := string(data[valueStart:valueEnd])
		if _, duplicate := params[id]; duplicate {
			return nil, fmt.Errorf("duplicate parameter ID %q", id)
		}
		params[id] = value
		order = append(order, id)
		pos = valueEnd + 1
	}

	return &parameterBlock{
		beforeCount: append([]byte(nil), data[:idxSpace+1]...),
		afterBlock:  append([]byte(nil), data[pos:]...),
		order:       order,
		params:      params,
	}, nil
}

func (p *parameterBlock) rebuild() []byte {
	seen := make(map[string]bool, len(p.params))
	keys := make([]string, 0, len(p.params))
	for _, id := range p.order {
		if _, ok := p.params[id]; ok && !seen[id] {
			keys = append(keys, id)
			seen[id] = true
		}
	}
	var added []string
	for id := range p.params {
		if !seen[id] {
			added = append(added, id)
		}
	}
	sort.Strings(added)
	keys = append(keys, added...)

	var out bytes.Buffer
	out.Grow(len(p.beforeCount) + len(p.afterBlock) + len(p.params)*24)
	out.Write(p.beforeCount)
	fmt.Fprintf(&out, "%d 0 0 0 ", len(keys))
	for _, id := range keys {
		fmt.Fprintf(&out, "%d %s %s ", len([]byte(id)), id, p.params[id])
	}
	out.Write(p.afterBlock)
	return out.Bytes()
}

func applyChanges(block *parameterBlock, changes []ParameterChange) ([]AppliedChange, []string, error) {
	applied := make([]AppliedChange, 0, len(changes))
	var warnings []string
	for index, change := range changes {
		canonical := CanonicalParameterID(change.ParameterID)
		if canonical == "" {
			return nil, warnings, fmt.Errorf("change %d has an empty parameter ID", index+1)
		}
		oldRaw, exists := block.params[canonical]
		if !exists {
			if !(change.AllowAdd && (strings.HasPrefix(canonical, "Modulations_") || isKnownParameter(canonical))) {
				return nil, warnings, fmt.Errorf("parameter %q is not present in this preset; set allow_add only for a verified known or Modulations_ parameter", canonical)
			}
			oldRaw = "0"
		}

		newRaw, oldDisplay, newDisplay, approximate, err := calculateNewValue(canonical, oldRaw, change)
		if err != nil {
			return nil, warnings, fmt.Errorf("change %d (%s): %w", index+1, canonical, err)
		}
		block.params[canonical] = newRaw
		applied = append(applied, AppliedChange{
			RequestedID: change.ParameterID,
			ParameterID: canonical,
			Operation:   normalizedOperation(change.Operation),
			Unit:        normalizedUnit(change.Unit),
			OldRaw:      oldRaw,
			NewRaw:      newRaw,
			OldDisplay:  oldDisplay,
			NewDisplay:  newDisplay,
			Reason:      change.Reason,
			Added:       !exists,
			Approximate: approximate,
		})
		if approximate {
			warnings = append(warnings, fmt.Sprintf("%s uses an approximate display-unit conversion; verify the displayed value inside Pigments.", canonical))
		}
	}
	return applied, uniqueStrings(warnings), nil
}

func calculateNewValue(id, oldRaw string, change ParameterChange) (newRaw, oldDisplay, newDisplay string, approximate bool, err error) {
	spec, known := SpecFor(id)
	operation := normalizedOperation(change.Operation)
	unit := normalizedUnit(change.Unit)
	if unit == "" {
		if known {
			unit = spec.Unit
		} else {
			unit = "raw"
		}
	}
	oldValue, parseErr := strconv.ParseFloat(strings.TrimSpace(oldRaw), 64)
	if parseErr != nil {
		return "", "", "", false, fmt.Errorf("current raw value %q is not numeric", oldRaw)
	}

	if operation == "toggle" {
		value := 1.0
		if math.Abs(oldValue) >= 0.5 {
			value = 0
		}
		return formatRaw(value), displayValue(id, oldValue), displayValue(id, value), false, nil
	}

	if unit == "enum" || (known && spec.Unit == "enum") {
		if operation != "set" {
			return "", "", "", false, fmt.Errorf("enum parameters support only the set operation")
		}
		if known {
			if mapped, ok := ResolveEnum(spec, change.Value); ok {
				newValue, _ := strconv.ParseFloat(mapped, 64)
				return mapped, displayValue(id, oldValue), displayValue(id, newValue), false, nil
			}
		}
		value, e := strconv.ParseFloat(strings.TrimSpace(change.Value), 64)
		if e != nil {
			return "", "", "", false, fmt.Errorf("unknown enum value %q", change.Value)
		}
		return formatRaw(value), displayValue(id, oldValue), displayValue(id, value), false, nil
	}

	if unit == "boolean" || (known && spec.Unit == "boolean") {
		if operation != "set" {
			return "", "", "", false, fmt.Errorf("boolean parameters support set or toggle")
		}
		value, e := parseBoolNumber(change.Value)
		if e != nil {
			return "", "", "", false, e
		}
		return formatRaw(value), displayValue(id, oldValue), displayValue(id, value), false, nil
	}

	requested, e := strconv.ParseFloat(strings.TrimSpace(change.Value), 64)
	if e != nil || math.IsNaN(requested) || math.IsInf(requested, 0) {
		return "", "", "", false, fmt.Errorf("invalid numeric value %q", change.Value)
	}

	var newValue float64
	switch unit {
	case "raw", "normalized":
		newValue, err = combine(oldValue, requested, operation)
	case "percent":
		oldPercent := oldValue * 100
		target, combineErr := combine(oldPercent, requested, operation)
		if combineErr != nil {
			err = combineErr
		} else {
			newValue = target / 100
		}
	case "hz", "db", "semitones", "bits":
		if !known || spec.Unit != unit {
			return "", "", "", false, fmt.Errorf("%s conversion is not defined for %s", unit, id)
		}
		oldDisplayNumber := rawToDisplayNumber(oldValue, spec)
		target, combineErr := combine(oldDisplayNumber, requested, operation)
		if combineErr != nil {
			err = combineErr
		} else {
			newValue = displayNumberToRaw(target, spec)
			approximate = true
		}
	default:
		return "", "", "", false, fmt.Errorf("unsupported unit %q", unit)
	}
	if err != nil {
		return "", "", "", false, err
	}

	if known && spec.Unit != "raw" {
		newValue = clamp(newValue, 0, 1)
	} else if strings.HasPrefix(id, "Modulations_") {
		newValue = clamp(newValue, -1, 1)
	}
	return formatRaw(newValue), displayValue(id, oldValue), displayValue(id, newValue), approximate, nil
}

func combine(oldValue, requested float64, operation string) (float64, error) {
	switch operation {
	case "set":
		return requested, nil
	case "add":
		return oldValue + requested, nil
	case "multiply":
		return oldValue * requested, nil
	default:
		return 0, fmt.Errorf("unsupported operation %q", operation)
	}
}

func normalizedOperation(operation string) string {
	switch strings.ToLower(strings.TrimSpace(operation)) {
	case "", "set", "replace":
		return "set"
	case "add", "increase", "decrease":
		return "add"
	case "multiply", "scale":
		return "multiply"
	case "toggle":
		return "toggle"
	default:
		return strings.ToLower(strings.TrimSpace(operation))
	}
}

func normalizedUnit(unit string) string {
	u := strings.ToLower(strings.TrimSpace(unit))
	switch u {
	case "", "raw", "normalized", "percent", "hz", "db", "semitones", "bits", "enum", "boolean":
		return u
	case "%", "percentage":
		return "percent"
	case "decibel", "decibels":
		return "db"
	case "st", "semitone":
		return "semitones"
	case "bit":
		return "bits"
	case "bool":
		return "boolean"
	default:
		return u
	}
}

func parseBoolNumber(value string) (float64, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "on", "yes", "enabled", "enable":
		return 1, nil
	case "0", "false", "off", "no", "disabled", "disable":
		return 0, nil
	default:
		return 0, fmt.Errorf("invalid boolean value %q", value)
	}
}

func rawToDisplayNumber(raw float64, spec ParameterSpec) float64 {
	if spec.Curve == "log" && spec.Min > 0 && spec.Max > spec.Min {
		return spec.Min * math.Pow(spec.Max/spec.Min, raw)
	}
	return spec.Min + raw*(spec.Max-spec.Min)
}

func displayNumberToRaw(value float64, spec ParameterSpec) float64 {
	value = clampRange(value, spec.Min, spec.Max)
	if spec.Curve == "log" && spec.Min > 0 && spec.Max > spec.Min {
		return math.Log(value/spec.Min) / math.Log(spec.Max/spec.Min)
	}
	if spec.Max == spec.Min {
		return 0
	}
	return (value - spec.Min) / (spec.Max - spec.Min)
}

func displayValue(id string, raw float64) string {
	spec, known := SpecFor(id)
	if !known {
		return formatRaw(raw)
	}
	switch spec.Unit {
	case "enum":
		bestDistance := math.MaxFloat64
		var bestNames []string
		for name, rawText := range spec.EnumValues {
			v, e := strconv.ParseFloat(rawText, 64)
			if e != nil {
				continue
			}
			d := math.Abs(v - raw)
			if d+1e-9 < bestDistance {
				bestDistance = d
				bestNames = []string{name}
			} else if math.Abs(d-bestDistance) <= 1e-9 {
				bestNames = append(bestNames, name)
			}
		}
		if len(bestNames) > 0 {
			sort.Slice(bestNames, func(i, j int) bool {
				iHasSpace := strings.Contains(bestNames[i], " ")
				jHasSpace := strings.Contains(bestNames[j], " ")
				if iHasSpace != jHasSpace {
					return iHasSpace
				}
				if len(bestNames[i]) != len(bestNames[j]) {
					return len(bestNames[i]) < len(bestNames[j])
				}
				return bestNames[i] < bestNames[j]
			})
			return fmt.Sprintf("%s (%s)", bestNames[0], formatRaw(raw))
		}
	case "boolean":
		if math.Abs(raw) >= 0.5 {
			return "on"
		}
		return "off"
	case "percent":
		return fmt.Sprintf("%.2f%%", raw*100)
	case "hz":
		return fmt.Sprintf("%.1f Hz (approx.)", rawToDisplayNumber(raw, spec))
	case "db":
		return fmt.Sprintf("%.2f dB (approx.)", rawToDisplayNumber(raw, spec))
	case "semitones":
		return fmt.Sprintf("%.2f st (approx.)", rawToDisplayNumber(raw, spec))
	case "bits":
		return fmt.Sprintf("%.2f bits (approx.)", rawToDisplayNumber(raw, spec))
	}
	return formatRaw(raw)
}

func formatRaw(value float64) string {
	if math.Abs(value) < 0.0000000001 {
		value = 0
	}
	text := strconv.FormatFloat(value, 'f', 8, 64)
	text = strings.TrimRight(strings.TrimRight(text, "0"), ".")
	if text == "" || text == "-0" {
		return "0"
	}
	return text
}

func clampRange(value, a, b float64) float64 {
	if a > b {
		a, b = b, a
	}
	return clamp(value, a, b)
}

func clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func isKnownParameter(id string) bool {
	_, ok := SpecFor(id)
	return ok
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	var result []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}
