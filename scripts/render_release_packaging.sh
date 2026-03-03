#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: scripts/render_release_packaging.sh <version> <sha256sums_file> [output_dir] [release_date]

Examples:
  scripts/render_release_packaging.sh 1.0.0 dist/SHA256SUMS.txt
  scripts/render_release_packaging.sh 1.0.0 dist/SHA256SUMS.txt packaging/rendered 2026-03-03
USAGE
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

if [[ $# -lt 2 || $# -gt 4 ]]; then
  usage >&2
  exit 1
fi

VERSION="$1"
SHAFILE="$2"
OUTDIR="${3:-packaging/rendered}"
RELEASE_DATE="${4:-$(date +%F)}"

if [[ ! -f "$SHAFILE" ]]; then
  echo "SHA file not found: $SHAFILE" >&2
  exit 1
fi

get_sha() {
  local name="$1"
  local sha
  sha="$(awk -v n="$name" '$2 == n {print $1}' "$SHAFILE" | tail -n1)"
  if [[ -z "$sha" ]]; then
    echo "missing checksum for $name in $SHAFILE" >&2
    exit 1
  fi
  printf '%s' "$sha"
}

declare -A TOKENS=(
  ["__VERSION__"]="$VERSION"
  ["__RELEASE_DATE__"]="$RELEASE_DATE"
  ["__SHA256_FIXDECODER_DARWIN_ARM64__"]="$(get_sha "fixdecoder-${VERSION}.darwin-arm64")"
  ["__SHA256_FIXDECODER_DARWIN_X86_64__"]="$(get_sha "fixdecoder-${VERSION}.darwin-x86_64")"
  ["__SHA256_FIXDECODER_LINUX_GNU_ARM64__"]="$(get_sha "fixdecoder-${VERSION}.linux-gnu-arm64")"
  ["__SHA256_FIXDECODER_LINUX_GNU_X86_64__"]="$(get_sha "fixdecoder-${VERSION}.linux-gnu-x86_64")"
  ["__SHA256_PCAP2FIX_DARWIN_ARM64__"]="$(get_sha "pcap2fix-${VERSION}.darwin-arm64")"
  ["__SHA256_PCAP2FIX_DARWIN_X86_64__"]="$(get_sha "pcap2fix-${VERSION}.darwin-x86_64")"
  ["__SHA256_PCAP2FIX_LINUX_GNU_ARM64__"]="$(get_sha "pcap2fix-${VERSION}.linux-gnu-arm64")"
  ["__SHA256_PCAP2FIX_LINUX_GNU_X86_64__"]="$(get_sha "pcap2fix-${VERSION}.linux-gnu-x86_64")"
  ["__SHA256_FIXDECODER_WINDOWS_X86_64__"]="$(get_sha "fixdecoder-${VERSION}.windows-x86_64.exe")"
  ["__SHA256_FIXDECODER_WINDOWS_ARM64__"]="$(get_sha "fixdecoder-${VERSION}.windows-arm64.exe")"
)

render() {
  local src="$1"
  local dst="$2"
  local content
  content="$(cat "$src")"
  for key in "${!TOKENS[@]}"; do
    content="${content//$key/${TOKENS[$key]}}"
  done
  printf '%s\n' "$content" > "$dst"
}

mkdir -p "$OUTDIR/homebrew" "$OUTDIR/winget"

render "packaging/homebrew/fixdecoder.rb.tmpl" "$OUTDIR/homebrew/fixdecoder.rb"
render "packaging/winget/StephenLClarke.Fixdecoder.yaml.tmpl" \
  "$OUTDIR/winget/StephenLClarke.Fixdecoder.yaml"
render "packaging/winget/StephenLClarke.Fixdecoder.locale.en-US.yaml.tmpl" \
  "$OUTDIR/winget/StephenLClarke.Fixdecoder.locale.en-US.yaml"
render "packaging/winget/StephenLClarke.Fixdecoder.installer.yaml.tmpl" \
  "$OUTDIR/winget/StephenLClarke.Fixdecoder.installer.yaml"

echo "Rendered templates into: $OUTDIR"
