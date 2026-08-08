package arturia

import (
	"archive/zip"
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

//go:embed Default
var defaultTemplate []byte

//go:embed User.png
var userBankPNG []byte

const (
	maxPGTXSize       = 256 << 20
	maxArchiveEntries = 256
)

type PresetArchive struct {
	entries     []archiveEntry
	presetIndex int
	InnerPath   string
	Metadata    Metadata
	block       *parameterBlock
	presetData  []byte
}

func LoadPGTX(path string) (*PresetArchive, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() <= 0 || info.Size() > maxPGTXSize {
		return nil, fmt.Errorf("invalid .pgtx size: %d bytes", info.Size())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParsePGTX(data)
}

func ParsePGTX(data []byte) (*PresetArchive, error) {
	if len(data) == 0 || len(data) > maxPGTXSize {
		return nil, fmt.Errorf("invalid .pgtx byte size %d", len(data))
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("open .pgtx ZIP: %w", err)
	}
	if len(zr.File) == 0 || len(zr.File) > maxArchiveEntries {
		return nil, fmt.Errorf("invalid .pgtx archive entry count %d", len(zr.File))
	}
	entries := make([]archiveEntry, 0, len(zr.File))
	presetIndex := -1
	var total int64
	for _, file := range zr.File {
		if file.FileInfo().IsDir() {
			continue
		}
		if file.UncompressedSize64 > maxPGTXSize {
			return nil, fmt.Errorf("archive entry %q is too large", file.Name)
		}
		total += int64(file.UncompressedSize64)
		if total > maxPGTXSize {
			return nil, fmt.Errorf("expanded archive is too large")
		}
		rc, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("open archive entry %q: %w", file.Name, err)
		}
		entryData, readErr := io.ReadAll(io.LimitReader(rc, maxPGTXSize+1))
		_ = rc.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read archive entry %q: %w", file.Name, readErr)
		}
		entries = append(entries, archiveEntry{Name: strings.ReplaceAll(file.Name, "\\", "/"), Data: entryData})
		if presetIndex < 0 && bytes.Contains(entryData, []byte("serialization::archive")) && bytes.Contains(entryData, []byte(firstParameterMarker)) {
			presetIndex = len(entries) - 1
		}
	}
	if presetIndex < 0 {
		return nil, fmt.Errorf("no Pigments serialized preset entry found in archive")
	}
	presetData := append([]byte(nil), entries[presetIndex].Data...)
	metadata, err := parseMetadata(presetData)
	if err != nil {
		return nil, err
	}
	block, err := parseParameterBlock(presetData)
	if err != nil {
		return nil, err
	}
	return &PresetArchive{
		entries:     entries,
		presetIndex: presetIndex,
		InnerPath:   entries[presetIndex].Name,
		Metadata:    metadata,
		block:       block,
		presetData:  presetData,
	}, nil
}

func NewFromDefault() (*PresetArchive, error) {
	data := append([]byte(nil), defaultTemplate...)
	metadata, err := parseMetadata(data)
	if err != nil {
		return nil, err
	}
	block, err := parseParameterBlock(data)
	if err != nil {
		return nil, err
	}
	entries := []archiveEntry{
		{Name: "Pigments/User/User/Default Pigments 7", Data: data},
		{Name: "User.png", Data: append([]byte(nil), userBankPNG...)},
	}
	return &PresetArchive{
		entries:     entries,
		presetIndex: 0,
		InnerPath:   entries[0].Name,
		Metadata:    metadata,
		block:       block,
		presetData:  data,
	}, nil
}

func (p *PresetArchive) Clone() (*PresetArchive, error) {
	data, err := p.Build()
	if err != nil {
		return nil, err
	}
	return ParsePGTX(data)
}

func (p *PresetArchive) ApplyPlan(plan PresetPlan, generated bool) ([]AppliedChange, []string, error) {
	changes, warnings, err := applyChanges(p.block, plan.Changes)
	if err != nil {
		return nil, warnings, err
	}
	p.presetData = p.block.rebuild()

	requestedName := strings.TrimSpace(plan.PatchName)
	name := requestedName
	if name == "" {
		name = p.Metadata.Name
	}
	bank := p.Metadata.Bank
	author := p.Metadata.Author
	if generated {
		bank = strings.TrimSpace(plan.BankOverride)
		if bank == "" {
			bank = "User"
		}
		author = strings.TrimSpace(plan.AuthorOverride)
		if author == "" {
			author = "Audio Prompters"
		}
	}
	if strings.TrimSpace(bank) == "" {
		bank = "User"
	}
	if strings.TrimSpace(author) == "" {
		author = "Audio Prompters"
	}

	// A narrow modification must not rewrite metadata-related fields that the
	// user did not ask us to touch. Some Pigments presets intentionally keep an
	// OriginalPresetName that differs from the current display name, and that
	// provenance should survive ordinary parameter edits. New presets and
	// explicit renames do update the metadata block.
	if generated || requestedName != "" {
		updated, err := replaceMetadata(p.presetData, Metadata{Name: name, Bank: bank, Author: author})
		if err != nil {
			return nil, warnings, err
		}
		p.presetData = updated
		p.Metadata = Metadata{Name: name, Bank: bank, Author: author}
	}

	updated, macroWarnings := applyMacroNames(p.presetData, plan.Macros)
	p.presetData = updated
	warnings = append(warnings, macroWarnings...)
	p.block, err = parseParameterBlock(p.presetData)
	if err != nil {
		return nil, warnings, fmt.Errorf("re-parse modified parameter block: %w", err)
	}

	oldPath := p.entries[p.presetIndex].Name
	p.InnerPath = oldPath
	// Existing presets occasionally use an inner filename that differs from their
	// metadata name. Preserve that exact path for a narrow edit unless the user
	// explicitly requested a rename. New generated presets always receive a path
	// matching their generated patch name.
	if generated || requestedName != "" {
		dir := path.Dir(oldPath)
		if dir == "." || dir == "/" {
			dir = "Pigments/User/" + safePathComponent(bank)
		}
		p.InnerPath = path.Join(dir, safePathComponent(name))
		p.entries[p.presetIndex].Name = p.InnerPath
	}
	p.entries[p.presetIndex].Data = p.presetData
	return changes, uniqueStrings(warnings), nil
}

func (p *PresetArchive) Build() ([]byte, error) {
	if p.presetIndex < 0 || p.presetIndex >= len(p.entries) {
		return nil, fmt.Errorf("invalid preset archive state")
	}
	p.entries[p.presetIndex].Data = p.presetData
	data, err := buildStoredZip(p.entries)
	if err != nil {
		return nil, err
	}
	if _, err := ParsePGTX(data); err != nil {
		return nil, fmt.Errorf("generated preset failed validation: %w", err)
	}
	return data, nil
}

func (p *PresetArchive) Save(outputPath string) error {
	data, err := p.Build()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(outputPath, data, 0644)
}

func GeneratePreset(plan PresetPlan, outputDir string) (*ApplyReport, error) {
	preset, err := NewFromDefault()
	if err != nil {
		return nil, err
	}
	changes, warnings, err := preset.ApplyPlan(plan, true)
	if err != nil {
		return nil, err
	}
	warnings = append(warnings, plan.Warnings...)
	outputPath, err := uniqueOutputPath(outputDir, preset.Metadata.Name)
	if err != nil {
		return nil, err
	}
	if err := preset.Save(outputPath); err != nil {
		return nil, err
	}
	return &ApplyReport{OutputPath: outputPath, Metadata: preset.Metadata, Summary: plan.Summary, Changes: changes, Warnings: uniqueStrings(warnings), CreatedAt: time.Now()}, nil
}

func ModifyPreset(sourcePath string, plan PresetPlan, outputDir string) (*ApplyReport, error) {
	preset, err := LoadPGTX(sourcePath)
	if err != nil {
		return nil, err
	}
	changes, warnings, err := preset.ApplyPlan(plan, false)
	if err != nil {
		return nil, err
	}
	warnings = append(warnings, plan.Warnings...)
	outputPath, err := uniqueOutputPath(outputDir, preset.Metadata.Name)
	if err != nil {
		return nil, err
	}
	if sameFile(sourcePath, outputPath) {
		return nil, fmt.Errorf("refusing to overwrite source preset")
	}
	if err := preset.Save(outputPath); err != nil {
		return nil, err
	}
	return &ApplyReport{SourcePath: sourcePath, OutputPath: outputPath, Metadata: preset.Metadata, Summary: plan.Summary, Changes: changes, Warnings: uniqueStrings(warnings), CreatedAt: time.Now()}, nil
}

func InspectPresetFile(filePath, query string, limit int) (*PresetInspection, error) {
	preset, err := LoadPGTX(filePath)
	if err != nil {
		return nil, err
	}
	inspection := preset.Inspect(query, limit)
	inspection.Path = filePath
	return inspection, nil
}

func (p *PresetArchive) Inspect(query string, limit int) *PresetInspection {
	if limit <= 0 {
		limit = 80
	}
	if limit > 500 {
		limit = 500
	}
	parameters := searchParameterViews(p.block.params, query, limit)
	entryNames := make([]string, 0, len(p.entries))
	for _, entry := range p.entries {
		entryNames = append(entryNames, entry.Name)
	}
	return &PresetInspection{
		Metadata:       p.Metadata,
		InnerPath:      p.InnerPath,
		ParameterCount: len(p.block.params),
		Parameters:     parameters,
		ArchiveEntries: entryNames,
	}
}

func searchParameterViews(params map[string]string, query string, limit int) []ParameterView {
	type scored struct {
		view  ParameterView
		score int
	}
	q := strings.ToLower(strings.TrimSpace(query))
	tokens := strings.Fields(q)
	var all []scored
	for id, raw := range params {
		spec, known := SpecFor(id)
		text := strings.ToLower(id)
		view := ParameterView{ID: id, RawValue: raw}
		value, _ := strconv.ParseFloat(raw, 64)
		if known {
			view.FriendlyName = spec.Friendly
			view.Description = spec.Description
			view.Unit = spec.Unit
			view.DisplayValue = displayValue(id, value)
			text += " " + strings.ToLower(spec.Friendly) + " " + strings.ToLower(strings.Join(spec.Aliases, " "))
		}
		score := 0
		if q == "" {
			if known {
				score = 10
			} else {
				score = 1
			}
		} else {
			if strings.EqualFold(q, id) {
				score += 1000
			}
			if strings.Contains(text, q) {
				score += 100
			}
			for _, token := range tokens {
				if strings.Contains(text, token) {
					score += 10
				}
			}
		}
		if score > 0 {
			all = append(all, scored{view: view, score: score})
		}
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].score != all[j].score {
			return all[i].score > all[j].score
		}
		return all[i].view.ID < all[j].view.ID
	})
	if len(all) > limit {
		all = all[:limit]
	}
	views := make([]ParameterView, len(all))
	for i := range all {
		views[i] = all[i].view
	}
	return views
}

// ParameterValues returns a defensive copy of all serialized numeric
// parameters in the preset. The caller cannot mutate the archive through it.
func (p *PresetArchive) ParameterValues() map[string]string {
	values := make(map[string]string, len(p.block.params))
	for id, raw := range p.block.params {
		values[id] = raw
	}
	return values
}

// DiffPresetFiles compares two snapshots without modifying either file. This
// is the most reliable way to discover which serialized ID belongs to a UI
// control: save a baseline, move one control, save again, then diff the pair.
func DiffPresetFiles(beforePath, afterPath string, limit int) (*PresetDiff, error) {
	before, err := LoadPGTX(beforePath)
	if err != nil {
		return nil, fmt.Errorf("load before preset: %w", err)
	}
	after, err := LoadPGTX(afterPath)
	if err != nil {
		return nil, fmt.Errorf("load after preset: %w", err)
	}
	if limit <= 0 {
		limit = 500
	}
	if limit > 5000 {
		limit = 5000
	}

	ids := make(map[string]struct{}, len(before.block.params)+len(after.block.params))
	for id := range before.block.params {
		ids[id] = struct{}{}
	}
	for id := range after.block.params {
		ids[id] = struct{}{}
	}
	ordered := make([]string, 0, len(ids))
	for id := range ids {
		ordered = append(ordered, id)
	}
	sort.Strings(ordered)

	all := make([]ParameterDifference, 0)
	for _, id := range ordered {
		oldRaw, oldOK := before.block.params[id]
		newRaw, newOK := after.block.params[id]
		if oldOK && newOK && oldRaw == newRaw {
			continue
		}
		diff := ParameterDifference{ParameterID: id, BeforeRaw: oldRaw, AfterRaw: newRaw, Status: "changed"}
		if !oldOK {
			diff.Status = "added"
		}
		if !newOK {
			diff.Status = "removed"
		}
		if spec, known := SpecFor(id); known {
			diff.FriendlyName = spec.Friendly
			diff.Unit = spec.Unit
			if value, parseErr := strconv.ParseFloat(oldRaw, 64); oldOK && parseErr == nil {
				diff.BeforeDisplay = displayValue(id, value)
			}
			if value, parseErr := strconv.ParseFloat(newRaw, 64); newOK && parseErr == nil {
				diff.AfterDisplay = displayValue(id, value)
			}
		}
		all = append(all, diff)
	}

	report := &PresetDiff{
		BeforeMetadata: before.Metadata,
		AfterMetadata:  after.Metadata,
		BeforeCount:    len(before.block.params),
		AfterCount:     len(after.block.params),
		ChangeCount:    len(all),
	}
	if len(all) > limit {
		report.Changes = all[:limit]
		report.Truncated = true
		report.Warnings = append(report.Warnings, fmt.Sprintf("diff contains %d changes; only the first %d are shown", len(all), limit))
	} else {
		report.Changes = all
	}
	return report, nil
}

func parseMetadata(data []byte) (Metadata, error) {
	prefix := []byte("22 serialization::archive 10 0 7 0 7 ")
	idx := bytes.Index(data, prefix)
	if idx < 0 {
		return Metadata{}, fmt.Errorf("Pigments metadata header not found")
	}
	pos := idx + len(prefix)
	name, next, err := readLengthString(data, pos)
	if err != nil {
		return Metadata{}, fmt.Errorf("read preset name: %w", err)
	}
	bank, next, err := readLengthString(data, next)
	if err != nil {
		return Metadata{}, fmt.Errorf("read preset bank: %w", err)
	}
	next = skipSpaces(data, next)
	constantEnd := bytes.IndexByte(data[next:], ' ')
	if constantEnd < 0 {
		return Metadata{}, fmt.Errorf("read metadata author marker")
	}
	constant := string(data[next : next+constantEnd])
	if constant != "22" {
		return Metadata{}, fmt.Errorf("unexpected metadata author marker %q", constant)
	}
	author, _, err := readLengthString(data, next+constantEnd+1)
	if err != nil {
		return Metadata{}, fmt.Errorf("read preset author: %w", err)
	}
	return Metadata{Name: name, Bank: bank, Author: author}, nil
}

func replaceMetadata(data []byte, metadata Metadata) ([]byte, error) {
	prefix := []byte("22 serialization::archive 10 0 7 0 7 ")
	idx := bytes.Index(data, prefix)
	if idx < 0 {
		return nil, fmt.Errorf("Pigments metadata header not found")
	}
	start := idx + len(prefix)
	_, next, err := readLengthString(data, start)
	if err != nil {
		return nil, err
	}
	_, next, err = readLengthString(data, next)
	if err != nil {
		return nil, err
	}
	next = skipSpaces(data, next)
	constantEnd := bytes.IndexByte(data[next:], ' ')
	if constantEnd < 0 {
		return nil, fmt.Errorf("metadata author marker not found")
	}
	constant := string(data[next : next+constantEnd])
	_, end, err := readLengthString(data, next+constantEnd+1)
	if err != nil {
		return nil, err
	}
	name := cleanMetadataString(metadata.Name, 96)
	bank := cleanMetadataString(metadata.Bank, 64)
	author := cleanMetadataString(metadata.Author, 96)
	replacement := fmt.Sprintf("%d %s %d %s %s %d %s", len([]byte(name)), name, len([]byte(bank)), bank, constant, len([]byte(author)), author)
	out := make([]byte, 0, len(data)+len(replacement)-(end-start))
	out = append(out, data[:start]...)
	out = append(out, []byte(replacement)...)
	out = append(out, data[end:]...)
	out = replaceLengthField(out, "18 OriginalPresetName ", name)
	return out, nil
}

func replaceLengthField(data []byte, marker, value string) []byte {
	idx := bytes.Index(data, []byte(marker))
	if idx < 0 {
		return data
	}
	start := idx + len(marker)
	_, end, err := readLengthString(data, start)
	if err != nil {
		return data
	}
	replacement := fmt.Sprintf("%d %s", len([]byte(value)), value)
	out := make([]byte, 0, len(data)+len(replacement)-(end-start))
	out = append(out, data[:start]...)
	out = append(out, replacement...)
	out = append(out, data[end:]...)
	return out
}

func readLengthString(data []byte, pos int) (string, int, error) {
	pos = skipSpaces(data, pos)
	if pos >= len(data) {
		return "", pos, io.ErrUnexpectedEOF
	}
	space := bytes.IndexByte(data[pos:], ' ')
	if space < 0 {
		return "", pos, io.ErrUnexpectedEOF
	}
	space += pos
	length, err := strconv.Atoi(string(data[pos:space]))
	if err != nil || length < 0 || length > 1<<20 {
		return "", pos, fmt.Errorf("invalid length prefix %q", string(data[pos:space]))
	}
	start := space + 1
	end := start + length
	if end > len(data) {
		return "", pos, io.ErrUnexpectedEOF
	}
	return string(data[start:end]), end, nil
}

func skipSpaces(data []byte, pos int) int {
	for pos < len(data) && data[pos] == ' ' {
		pos++
	}
	return pos
}

func applyMacroNames(data []byte, macros MacroNames) ([]byte, []string) {
	values := []string{macros.Macro1, macros.Macro2, macros.Macro3, macros.Macro4}
	out := append([]byte(nil), data...)
	var warnings []string
	for i, value := range values {
		if strings.TrimSpace(value) == "" {
			continue
		}
		marker := []byte(fmt.Sprintf("11 Macro%d_Name 16 ", i+1))
		idx := bytes.Index(out, marker)
		if idx < 0 || idx+len(marker)+16 > len(out) {
			warnings = append(warnings, fmt.Sprintf("Macro %d label field was not found and was left unchanged.", i+1))
			continue
		}
		clean := truncateUTF8(cleanMetadataString(value, 16), 16)
		field := make([]byte, 16)
		copy(field, []byte(clean))
		copy(out[idx+len(marker):idx+len(marker)+16], field)
	}
	return out, warnings
}

func cleanMetadataString(value string, maxBytes int) string {
	value = strings.TrimSpace(value)
	var b strings.Builder
	for _, r := range value {
		if r == 0 || unicode.IsControl(r) {
			continue
		}
		b.WriteRune(r)
	}
	value = strings.Join(strings.Fields(b.String()), " ")
	if value == "" {
		value = "Unnamed Patch"
	}
	return truncateUTF8(value, maxBytes)
}

func truncateUTF8(value string, maxBytes int) string {
	if len([]byte(value)) <= maxBytes {
		return value
	}
	data := []byte(value)
	data = data[:maxBytes]
	for len(data) > 0 && !utf8.Valid(data) {
		data = data[:len(data)-1]
	}
	return string(data)
}

func safePathComponent(value string) string {
	value = cleanMetadataString(value, 96)
	value = strings.ReplaceAll(value, "/", "-")
	value = strings.ReplaceAll(value, "\\", "-")
	return value
}

func SafeFilenameName(value string) string {
	value = cleanMetadataString(value, 80)
	var b strings.Builder
	lastUnderscore := false
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastUnderscore = false
		} else if !lastUnderscore {
			b.WriteByte('_')
			lastUnderscore = true
		}
	}
	result := strings.Trim(b.String(), "_")
	if result == "" {
		return "Unnamed_Patch"
	}
	return result
}

func uniqueOutputPath(outputDir, patchName string) (string, error) {
	if strings.TrimSpace(outputDir) == "" {
		outputDir = "."
	}
	abs, err := filepath.Abs(outputDir)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(abs, 0755); err != nil {
		return "", err
	}
	timestamp := time.Now().Format("20060102_1504")
	base := fmt.Sprintf("Pigments_Preset_%s_%s", SafeFilenameName(patchName), timestamp)
	candidate := filepath.Join(abs, base+".pgtx")
	for i := 2; ; i++ {
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate, nil
		} else if err != nil {
			return "", err
		}
		candidate = filepath.Join(abs, fmt.Sprintf("%s_%d.pgtx", base, i))
	}
}

func sameFile(a, b string) bool {
	aa, errA := filepath.Abs(a)
	bb, errB := filepath.Abs(b)
	return errA == nil && errB == nil && filepath.Clean(aa) == filepath.Clean(bb)
}
