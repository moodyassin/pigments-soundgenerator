#!/usr/bin/env bash
set -euo pipefail

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
VERSION=${VERSION:-0.3.0}
DIST=${DIST:-"$ROOT/dist"}
STAGE=${STAGE:-"${TMPDIR:-/tmp}/audio-prompters-release-$VERSION"}
LDFLAGS='-s -w'

rm -rf "$DIST" "$STAGE"
mkdir -p "$DIST" "$STAGE"
cd "$ROOT"

make_common_files() {
  local dir="$1"
  mkdir -p "$dir/docs" "$dir/knowledge"
  cp README.md LICENSE NOTICE.md CHANGELOG.md VERIFICATION.md SECURITY.md .env.example "$dir/"
  cp docs/KNOWN_LIMITATIONS.md docs/SCREENSHOT_CAPTURE_GUIDE.md docs/PARAMETER_RESEARCH_PROTOCOL.md docs/LEGAL_AND_CONTENT_CHECKLIST.md docs/MASTER_DATABASE_IMPORT_REPORT.md docs/MASTER_DATABASE_AUDIT.md "$dir/docs/"
  cp docs/platform-preview.png "$dir/docs/"
  cp knowledge/parameter_submission_template.csv knowledge/parameter_submission_template.json knowledge/control_parameter_mappings_v1_0.json knowledge/control_parameter_mappings_v1_0.csv "$dir/knowledge/"
}

D="$STAGE/Audio-Prompters-Pigments-Web-MVP-$VERSION-macOS-Apple-Silicon"
make_common_files "$D"
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags "$LDFLAGS" -o "$D/audio-prompters-pigments-web" .
cat > "$D/Run Demo.command" <<'LAUNCH'
#!/bin/bash
set -e
cd "$(dirname "$0")"
mkdir -p data
exec ./audio-prompters-pigments-web serve --mock --open --data-dir ./data
LAUNCH
cat > "$D/Run with OpenAI API.command" <<'LAUNCH'
#!/bin/bash
set -e
cd "$(dirname "$0")"
if [ -f .env.local ]; then
  set -a
  source ./.env.local
  set +a
fi
if [ -z "${OPENAI_API_KEY:-}" ]; then
  echo "OPENAI_API_KEY is not configured."
  echo "Copy .env.example to .env.local, add your server-side API key, then run this file again."
  read -r -p "Press Return to close."
  exit 1
fi
mkdir -p data
exec ./audio-prompters-pigments-web serve --open --data-dir ./data
LAUNCH
chmod +x "$D/audio-prompters-pigments-web" "$D/Run Demo.command" "$D/Run with OpenAI API.command"

D="$STAGE/Audio-Prompters-Pigments-Web-MVP-$VERSION-macOS-Intel"
make_common_files "$D"
GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "$LDFLAGS" -o "$D/audio-prompters-pigments-web" .
cp "$STAGE/Audio-Prompters-Pigments-Web-MVP-$VERSION-macOS-Apple-Silicon/Run Demo.command" "$D/"
cp "$STAGE/Audio-Prompters-Pigments-Web-MVP-$VERSION-macOS-Apple-Silicon/Run with OpenAI API.command" "$D/"
chmod +x "$D/audio-prompters-pigments-web" "$D/Run Demo.command" "$D/Run with OpenAI API.command"

D="$STAGE/Audio-Prompters-Pigments-Web-MVP-$VERSION-Windows-x64"
make_common_files "$D"
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "$LDFLAGS" -o "$D/audio-prompters-pigments-web.exe" .
cat > "$D/Run Demo.bat" <<'LAUNCH'
@echo off
cd /d "%~dp0"
if not exist data mkdir data
start "" http://127.0.0.1:8080
audio-prompters-pigments-web.exe serve --mock --data-dir .\data
if errorlevel 1 pause
LAUNCH
cat > "$D/Run with OpenAI API.bat" <<'LAUNCH'
@echo off
cd /d "%~dp0"
if "%OPENAI_API_KEY%"=="" (
  echo OPENAI_API_KEY is not configured in this Command Prompt.
  echo Run: set OPENAI_API_KEY=your_server_side_api_key
  echo Optional: set OPENAI_MODEL=gpt-5.6-terra
  echo Then launch this file from the same environment or run the EXE directly.
  pause
  exit /b 1
)
if not exist data mkdir data
start "" http://127.0.0.1:8080
audio-prompters-pigments-web.exe serve --data-dir .\data
if errorlevel 1 pause
LAUNCH

D="$STAGE/Audio-Prompters-Pigments-Web-MVP-$VERSION-Linux-x64"
make_common_files "$D"
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "$LDFLAGS" -o "$D/audio-prompters-pigments-web" .
cat > "$D/run-demo.sh" <<'LAUNCH'
#!/usr/bin/env sh
set -eu
cd "$(dirname "$0")"
mkdir -p data
exec ./audio-prompters-pigments-web serve --mock --open --data-dir ./data
LAUNCH
cat > "$D/run-with-openai-api.sh" <<'LAUNCH'
#!/usr/bin/env sh
set -eu
cd "$(dirname "$0")"
if [ -f .env.local ]; then
  set -a
  . ./.env.local
  set +a
fi
if [ -z "${OPENAI_API_KEY:-}" ]; then
  echo "OPENAI_API_KEY is not configured. Copy .env.example to .env.local and add the key." >&2
  exit 1
fi
mkdir -p data
exec ./audio-prompters-pigments-web serve --open --data-dir ./data
LAUNCH
chmod +x "$D/audio-prompters-pigments-web" "$D/run-demo.sh" "$D/run-with-openai-api.sh"

SRC="$STAGE/Audio-Prompters-Pigments-Web-MVP-$VERSION-Source"
mkdir -p "$SRC"
rsync -a --exclude 'dist/' --exclude 'data/' --exclude '.git/' --exclude '*.log' "$ROOT/" "$SRC/"

cd "$STAGE"
for dir in Audio-Prompters-Pigments-Web-MVP-$VERSION-*; do
  zip -q -r "$DIST/${dir}.zip" "$dir"
done

DOC="$STAGE/Audio-Prompters-Pigments-Web-MVP-$VERSION-Parameter-Research-Pack"
mkdir -p "$DOC"
cp "$ROOT/docs/PARAMETER_RESEARCH_PROTOCOL.md" "$ROOT/docs/SCREENSHOT_CAPTURE_GUIDE.md" "$ROOT/docs/MASTER_DATABASE_IMPORT_REPORT.md" "$ROOT/knowledge/parameter_submission_template.csv" "$ROOT/knowledge/parameter_submission_template.json" "$ROOT/knowledge/control_parameter_mappings_v1_0.json" "$ROOT/knowledge/control_parameter_mappings_v1_0.csv" "$DOC/"
zip -q -r "$DIST/Audio-Prompters-Pigments-Web-MVP-$VERSION-Parameter-Research-Pack.zip" "$(basename "$DOC")"

DBPACK="$STAGE/Audio-Prompters-Pigments-Web-MVP-$VERSION-Master-Database-Pack"
mkdir -p "$DBPACK/docs" "$DBPACK/knowledge" "$DBPACK/generated" "$DBPACK/scripts"
cp "$ROOT/knowledge/pigments7_master_database_v1_6.json" "$ROOT/knowledge/control_parameter_mappings_v1_0.json" "$ROOT/knowledge/control_parameter_mappings_v1_0.csv" "$DBPACK/knowledge/"
cp "$ROOT/internal/knowledge/pigments_master_catalog.json" "$ROOT/internal/knowledge/pigments_internal_index.json" "$ROOT/internal/arturia/master_parameter_specs.json" "$DBPACK/generated/"
cp "$ROOT/docs/MASTER_DATABASE_IMPORT_REPORT.md" "$ROOT/docs/MASTER_DATABASE_AUDIT.md" "$DBPACK/docs/"
cp "$ROOT/scripts/import-master-database.py" "$ROOT/scripts/audit-master-database.py" "$DBPACK/scripts/"
zip -q -r "$DIST/Audio-Prompters-Pigments-Web-MVP-$VERSION-Master-Database-Pack.zip" "$(basename "$DBPACK")"

cd "$DIST"
sha256sum *.zip > SHA256SUMS.txt
printf 'Release written to %s\n' "$DIST"
