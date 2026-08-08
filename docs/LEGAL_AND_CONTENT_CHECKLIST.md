# Legal and content checklist

This is a product-development checklist, not legal advice.

## Before public launch

- Obtain legal review of the intended `.pgtx` generation and modification workflow.
- Ask Arturia for written guidance or permission covering:
  - compatible third-party preset generation;
  - use of a clean template or documented preset format;
  - automated installation in a future companion app;
  - use of Pigments names, screenshots, and compatibility statements;
  - handling of factory preset structures and embedded assets.
- Replace or clear every inherited template/resource whose commercial redistribution status is uncertain.
- Maintain an independent-product disclaimer.
- Do not imply certification, endorsement, or partnership unless documented.
- Review OpenAI branding requirements before using ChatGPT/GPT/OpenAI names in product branding.

## User-uploaded preset policy

- Require the uploader to confirm ownership or permission to modify.
- Keep modifications private by default.
- Do not add modified uploads to a public library or marketplace automatically.
- Detect and handle embedded samples, wavetables, images, or other assets.
- Publish retention and deletion periods.
- Provide a rights-holder reporting and takedown process.
- Do not use customer files to train a public model or create public examples without separate consent.

## Generated content policy

- Use original preset names and descriptions.
- Avoid requests that explicitly copy a living artist's proprietary preset or a commercially sold patch one-to-one.
- Allow high-level sonic characteristics without promising exact replication.
- Record provenance: template version, compiler version, model, prompt, and changes.
- Make clear that generated presets are third-party and not validated by Arturia.

## Samples and wavetables

- Do not bundle Arturia factory samples or wavetables unless licensed for redistribution.
- Do not copy user source material into a generated preset unless the user owns the relevant rights and explicitly requests embedding.
- Prefer user-created or properly licensed assets with stored license metadata.
- Add malware scanning and format validation for all uploaded assets.

## Screenshots and manuals

- Use screenshots supplied for private research only unless publication rights are clear.
- Avoid copying manual pages into the product.
- Create original diagrams and descriptions.
- Link users to official documentation for operation instructions.

## Future desktop companion

- Do not inject code into Pigments, bypass licensing, modify Arturia binaries, or automate activation.
- Use documented operating-system and application interfaces.
- Require clear user consent before writing to preset directories.
- Back up files and provide rollback.
- Sign and notarize releases where supported.
- Complete Arturia technical/legal review before implementing automatic import or control.
