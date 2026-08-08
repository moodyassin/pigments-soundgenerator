# Pigments parameter research protocol

Visible UI information is essential, but it does not prove how Pigments serializes a control. The most reliable research unit is a **controlled before/after preset pair**.

## One-control rule

For each test, change exactly one visible control. Do not play notes, move macros, change the mod wheel, switch tabs in a way that changes state, or alter metadata between saves unless that action is the subject of the test.

## Basic procedure

1. Record the exact Pigments version and operating system.
2. Load a known starting preset; preferably a clean research preset.
3. Confirm that no modulation source is moving the target control.
4. Save `Before_[control]_[value].pgtx`.
5. Capture a full-panel screenshot and a close-up showing the target value.
6. Change only the target control.
7. Save `After_[control]_[value].pgtx`.
8. Capture the after screenshot.
9. Upload the pair to Parameter Lab or run:

```bash
pigments-web diff --before Before.pgtx --after After.pgtx
```

10. Record the serialized ID, old value, new value, visible values, and confidence level.

## Knob and slider curves

A two-point test discovers an ID but usually does not prove a curve. For a continuous control, save controlled values at:

- minimum;
- 25%;
- center/default where applicable;
- 75%;
- maximum.

For frequency, time, gain, or rate controls, also capture meaningful displayed values. These controls are often logarithmic, piecewise, bipolar, or otherwise non-linear.

## Enum and switch controls

For a switch, save off and on.

For an enum, save one file for every option. Keep the surrounding preset identical. Record the visible order because serialized direction may be reversed.

Examples:

- Coarse Mod Quantize: C, C#, D, D#, E, F, F#, G, G#, A, A#, B;
- Bit Crush mode: Classic, Smooth;
- Sample Shaper mode: Unison, Chord, Super, Resonator, Bit Crush, Modulation.

## Modulation routing tests

Use a dedicated baseline with no existing route from the selected source to the selected destination.

1. Save baseline.
2. Create exactly one routing.
3. Set a clearly recorded positive amount.
4. Save.
5. Repeat for a negative amount if the destination supports bipolar modulation.
6. Test source mode and retrigger settings separately.

Do not combine route creation, amount, smoothing, polarity, and source-mode changes in one pair.

## Sample Engine tests

Capture these separately for Engine 1 and Engine 2:

- keyboard tracking;
- coarse-mod quantize;
- coarse tune;
- fine tune;
- sample filter at LP, No Filter, and HP;
- shaper enable;
- every shaper mode;
- Bit Crush decimate at several points;
- Classic/Smooth;
- bit depth at 16, 12, 8, 4, and 1.5 bits where selectable;
- pitch follow;
- sample start at several positions;
- A–F slot selection, trim start, trim end, gain, tune, direction, loop, and any other visible per-slot control.

The time ruler depends on the loaded sample. Record both the displayed seconds and the sample duration. Do not assume seven seconds is a global range.

## Sample and wavetable asset research

Do not redistribute Arturia factory assets. For serialization research, use an original test sample or wavetable for which you own all rights. Record:

- source filename and checksum;
- format, sample rate, channels, duration, and bit depth;
- whether the `.pgtx` embeds the asset or references a path;
- archive entries added or removed;
- behavior after moving the original source file.

## Confidence levels

- `ui_only`: visible control documented; no serialized evidence.
- `id_discovered`: controlled pair identifies a serialized ID.
- `direction_verified`: enum/switch direction confirmed.
- `curve_approximate`: multiple points fit a likely curve but not exact.
- `curve_verified`: enough points and import tests establish conversion.
- `roundtrip_verified`: generated value imports and displays correctly in Pigments.
- `audition_verified`: sound behavior matches the expected control.

Only `roundtrip_verified` controls should be considered safe for a commercial generation allowlist.
