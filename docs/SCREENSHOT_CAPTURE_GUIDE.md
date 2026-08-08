# Pigments screenshot capture guide

Screenshots are valuable for labels, hierarchy, ranges, enums, defaults, dependencies, and version changes. They should accompany controlled `.pgtx` pairs rather than replace them.

## General capture rules

- Use screenshots from your installed, licensed copy of Pigments.
- Record the exact Pigments version and operating system.
- Use 100% or a documented UI scaling value.
- Capture PNG when possible.
- Keep the entire Pigments window visible in one image for context.
- Add a second close-up for small labels and readouts.
- Open dropdowns when documenting enums.
- Hover or focus the control so the exact displayed value is visible.
- Do not apply annotations over labels or values; place notes in the submission sheet.
- Avoid including account email, serial numbers, purchase data, or other private information.

## Required Sample Engine screenshot set

1. Full Pigments window with Engine 1 set to Sample.
2. Full Sample Engine panel with Tune, sample viewer, A–F slots, and Shaper visible.
3. Tune close-up:
   - Keyboard Tracking
   - Coarse Mod Quantize dropdown
   - Coarse
   - Fine
   - Sample Filter
4. Sample browser with:
   - Categories list
   - Folders/User content area
   - search field
   - selected item name
5. Sample-viewer close-up:
   - complete time ruler
   - loaded sample duration
   - Sample Start
   - selected A–F slot
   - trim/loop markers
6. Shaper-mode dropdown showing all modes.
7. Bit Crush close-up:
   - Decimate
   - Classic/Smooth mode
   - Bit Depth
   - Pitch Follow
8. One screenshot for each state that changes the visible control layout.
9. Repeat the relevant set for Engine 2 if its serialized behavior differs.

## File naming

```text
Pigments_[version]_[engine]_[section]_[control]_[state]_[sequence].png
```

Example:

```text
Pigments_7.0.1_Engine1_Sample_BitDepth_8bit_01.png
```

## What screenshots cannot establish alone

A screenshot cannot prove:

- the internal parameter ID;
- the normalized value stored in `.pgtx`;
- whether the curve is linear or logarithmic;
- whether a dropdown order is serialized forward or backward;
- whether a sample is embedded or path-referenced;
- whether a generated value round-trips correctly.

Pair screenshots with baseline/changed presets using `PARAMETER_RESEARCH_PROTOCOL.md`.

## Redistribution caution

Treat screenshots as internal research evidence unless you have permission to publish them. For a public website, use original Audio Prompters graphics and diagrams rather than reproducing Arturia manuals or application screenshots as marketing assets without review.
