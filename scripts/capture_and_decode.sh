#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: capture_and_decode.sh <ssh_user@host> <tcpdump_host> <port> [fixdecoder args...]

The script will:
  - Find local pcap2fix and fixdecoder (prefer local dir, then ./target/release).
  - Ensure pcap2fix and fixdecoder exist on the remote (~/bin), copying if missing.
  - If remote fixdecoder version differs from local, upload the local one.
  - Run tcpdump -> pcap2fix -> fixdecoder entirely on the remote host.

Env overrides:
  REMOTE_TIMEOUT   tcpdump timeout in seconds (default: 120)
  REMOTE_COUNT     tcpdump packet count limit  (default: 5000)
USAGE
}

if [[ $# -lt 3 ]]; then
  usage
  exit 1
fi

SSH_TARGET="$1"
TCP_HOST="$2"
PORT="$3"
shift 3
FIXDECODER_ARGS=("$@")

REMOTE_TIMEOUT="${REMOTE_TIMEOUT:-120}"
REMOTE_COUNT="${REMOTE_COUNT:-5000}"

# Discover remote environment
REMOTE_UNAME_S="$(ssh "${SSH_TARGET}" "uname -s" 2>/dev/null || echo "")"
REMOTE_UNAME_M="$(ssh "${SSH_TARGET}" "uname -m" 2>/dev/null || echo "")"
REMOTE_HOME="$(ssh "${SSH_TARGET}" "printf %s \"\$HOME\"" 2>/dev/null || echo "")"
case "${REMOTE_UNAME_S}_${REMOTE_UNAME_M}" in
  Linux_x86_64) REMOTE_TARGET="x86_64-unknown-linux-gnu" ;;
  Linux_aarch64) REMOTE_TARGET="aarch64-unknown-linux-gnu" ;;
  Linux_arm64) REMOTE_TARGET="aarch64-unknown-linux-gnu" ;;
  Darwin_x86_64) REMOTE_TARGET="x86_64-apple-darwin" ;;
  Darwin_arm64) REMOTE_TARGET="aarch64-apple-darwin" ;;
  Darwin_aarch64) REMOTE_TARGET="aarch64-apple-darwin" ;;
  *) REMOTE_TARGET="" ;;
esac

find_local_bin() {
  local name="$1"
  local target="$2"
  if [[ -n "${target}" && -f "./target/${target}/release/${name}" && -x "./target/${target}/release/${name}" ]]; then
    echo "./target/${target}/release/${name}"
    return 0
  fi
  if [[ -f "./${name}" && -x "./${name}" ]]; then
    echo "./${name}"
    return 0
  fi
  if [[ -f "./target/release/${name}" && -x "./target/release/${name}" ]]; then
    echo "./target/release/${name}"
    return 0
  fi
  if command -v "${name}" >/dev/null 2>&1; then
    command -v "${name}"
    return 0
  fi
  return 1
}

LOCAL_PCAP2FIX="$(find_local_bin pcap2fix "${REMOTE_TARGET}")" || { echo "pcap2fix not found locally"; exit 1; }
LOCAL_FIXDECODER="$(find_local_bin fixdecoder "${REMOTE_TARGET}")" || { echo "fixdecoder not found locally"; exit 1; }
LOCAL_FIXDECODER_VERSION="$(${LOCAL_FIXDECODER} --version | head -1 || true)"

if [[ -z "${REMOTE_HOME}" ]]; then
  echo "error: could not determine remote HOME" >&2
  exit 1
fi

# Ensure ~/bin exists and PATH picks it up for the remote session.
ssh -tt "${SSH_TARGET}" "mkdir -p \"${REMOTE_HOME}/bin\"" >/dev/null

local_bin_compatible() {
  local bin="$1"
  local desc
  desc="$(file -b "${bin}" 2>/dev/null || true)"
  # We only attempt upload if the binary matches remote target/arch.
  if [[ -n "${REMOTE_TARGET}" && "${bin}" == *"/target/${REMOTE_TARGET}/release/"* ]]; then
    return 0
  fi
  [[ "${desc}" == *"ELF"* ]] || [[ "${desc}" == *"Mach-O"* ]] || return 1
  [[ -n "${REMOTE_UNAME_M}" ]] || return 1
  [[ "${desc}" == *"${REMOTE_UNAME_M}"* ]] || return 1
  return 0
}

ensure_remote_bin() {
  local name="$1"
  local local_path="$2"
  local ensure_version="$3" # empty or version string to compare for fixdecoder

  local remote_path
  remote_path="$(ssh "${SSH_TARGET}" "PATH=\"${REMOTE_HOME}/bin:\$PATH\"; command -v ${name} || true")"
  if [[ -z "${remote_path}" ]]; then
    if local_bin_compatible "${local_path}"; then
      scp "${local_path}" "${SSH_TARGET}:${REMOTE_HOME}/bin/${name}" >/dev/null
      ssh "${SSH_TARGET}" "chmod +x \"${REMOTE_HOME}/bin/${name}\"" >/dev/null
      echo "${REMOTE_HOME}/bin/${name}"
      return
    else
      echo "error: no remote ${name} and local binary is not compatible with remote arch (${REMOTE_UNAME_S} ${REMOTE_UNAME_M}). Build ${name} on the remote host (e.g. cargo build --release) or provide a compatible binary." >&2
      exit 1
    fi
  fi

  if [[ -n "${ensure_version}" ]]; then
    local remote_version
    remote_version="$(ssh "${SSH_TARGET}" "PATH=\"${REMOTE_HOME}/bin:\$PATH\"; ${name} --version | head -1 || true")"
    if [[ "${remote_version}" != "${ensure_version}" ]]; then
      if local_bin_compatible "${local_path}"; then
        scp "${local_path}" "${SSH_TARGET}:${REMOTE_HOME}/bin/${name}" >/dev/null
        ssh "${SSH_TARGET}" "chmod +x \"${REMOTE_HOME}/bin/${name}\"" >/dev/null
        echo "${REMOTE_HOME}/bin/${name}"
        return
      else
        echo "warning: remote ${name} (${remote_version}) differs from local (${ensure_version}) but local binary is not compatible with remote arch; using remote version." >&2
      fi
    fi
  fi

  # Ensure the remote binary is executable and runnable; otherwise fail with guidance.
  if ! ssh "${SSH_TARGET}" "PATH=\"${REMOTE_HOME}/bin:\$PATH\"; ${name} --version >/dev/null 2>&1"; then
    if local_bin_compatible "${local_path}"; then
      scp "${local_path}" "${SSH_TARGET}:${REMOTE_HOME}/bin/${name}" >/dev/null
      ssh "${SSH_TARGET}" "chmod +x \"${REMOTE_HOME}/bin/${name}\"" >/dev/null
      echo "${REMOTE_HOME}/bin/${name}"
      return
    else
      echo "error: remote ${name} is not executable and local binary is not compatible with remote arch (${REMOTE_UNAME_S} ${REMOTE_UNAME_M}). Build ${name} on the remote host (e.g. cargo build --release)." >&2
      exit 1
    fi
  fi

  echo "${remote_path}"
}

REMOTE_PCAP2FIX="$(ensure_remote_bin pcap2fix "${LOCAL_PCAP2FIX}" "")"
REMOTE_FIXDECODER="$(ensure_remote_bin fixdecoder "${LOCAL_FIXDECODER}" "${LOCAL_FIXDECODER_VERSION}")"
REMOTE_TIMEOUT_BIN="$(ssh "${SSH_TARGET}" "command -v timeout || command -v gtimeout || true")"
if [[ -z "${REMOTE_TIMEOUT_BIN}" ]]; then
  echo "warning: remote timeout/gtimeout not found; relying on tcpdump packet count limit (${REMOTE_COUNT})" >&2
fi

join_quoted() {
  local out=""
  for arg in "$@"; do
    out+=$(printf " %q" "${arg}")
  done
  echo "${out}"
}

REMOTE_FIX_ARGS="$(join_quoted "${FIXDECODER_ARGS[@]}")"

REMOTE_FILTER="(host ${TCP_HOST} and port ${PORT}) and tcp[((tcp[12] & 0xf0) >> 2):4] = 0x383d4649 and tcp[((tcp[12] & 0xf0) >> 2) + 4] = 0x58"

# Run everything remotely to avoid local pipes hanging.
if [[ -n "${REMOTE_TIMEOUT_BIN}" ]]; then
  ssh -tt "${SSH_TARGET}" "set -euo pipefail; PATH=\"${REMOTE_HOME}/bin:\$PATH\"; sudo ${REMOTE_TIMEOUT_BIN} ${REMOTE_TIMEOUT} tcpdump -U -n -s0 -c ${REMOTE_COUNT} -i any -w - \"${REMOTE_FILTER}\" | ${REMOTE_PCAP2FIX} --port ${PORT} | ${REMOTE_FIXDECODER} --follow${REMOTE_FIX_ARGS}"
else
  ssh -tt "${SSH_TARGET}" "set -euo pipefail; PATH=\"${REMOTE_HOME}/bin:\$PATH\"; sudo tcpdump -U -n -s0 -c ${REMOTE_COUNT} -i any -w - \"${REMOTE_FILTER}\" | ${REMOTE_PCAP2FIX} --port ${PORT} | ${REMOTE_FIXDECODER} --follow${REMOTE_FIX_ARGS}"
fi
