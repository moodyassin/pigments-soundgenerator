# Known limitations — v0.3.0

- The release is a functional development MVP, not a complete hosted SaaS product.
- Jobs are indexed in server memory; restarting the server invalidates download/report IDs even when files remain until cleanup.
- Local storage is used instead of a database and private object storage.
- The built-in rate limiter is process-local and not suitable for multi-instance deployment.
- Real OpenAI API behavior has not been tested with the owner's production key in this build environment.
- Generated presets have not been imported and auditioned in the user's installed Pigments 7.0.1 from this build environment.
- The development template originated in an external proof of concept and requires rights and compatibility review before commercial distribution.
- The master database contains 3,525 observed internal IDs, but an observed ID alone does not prove its visible UI control, displayed unit, enum order, or conversion curve.
- The mapping overlay currently permits 79 documented controls for conservative automatic editing. Combined with the earlier curated catalog, 245 parameter specifications are planner-visible after calibration locks; this is still not complete Pigments coverage.
- Fourteen continuous controls still lack documented ranges, eight selectors lack option lists, and all 125 documented controls currently omit explicit defaults.
- Some Hz, dB, semitone, time, enum, ratio, and bit-depth conversions remain approximate until controlled round-trip calibration is complete.
- Sample and wavetable asset selection requires serialized-object support and is not treated as a simple numeric parameter change.
- Sample A–F asset replacement is disabled.
- Sample browser categories and names vary by installed version/content; the included reference subset is not authoritative or exhaustive.
- Source screenshot records are research evidence. Arturia screenshots are not redistributed in the public runtime package.
- The source database revision `pgtx_import_001` uses its revision ID as a `source_id`, but that ID is not represented in the source array. The importer records this as a provenance warning and leaves the source file unchanged.
- No accounts, payments, subscriptions, email, analytics, admin console, cloud queue, antivirus service, or customer-support system are included.
- No direct/live Pigments connection is included in Phase 1.
