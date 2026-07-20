#!/usr/bin/env bash
# scripts/setup-tinygo-bluetooth.sh
#
# Clones tinygo.org/x/bluetooth v0.15.0 into ./tinygo-bluetooth/ and applies
# any patch files that are tracked in that directory by this repository.
#
# Usage:
#   ./scripts/setup-tinygo-bluetooth.sh
#   ./scripts/setup-tinygo-bluetooth.sh --force   # re-clone even if already present
#
# The ./tinygo-bluetooth/ directory is ignored in git except for the patched
# files. This script is called automatically by build scripts.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TARGET_DIR="${REPO_ROOT}/tinygo-bluetooth"
UPSTREAM_URL="https://github.com/tinygo-org/bluetooth.git"
UPSTREAM_TAG="v0.15.0"
TMP_DIR="$(mktemp -d)"

FORCE=false
if [[ "${1:-}" == "--force" ]]; then
  FORCE=true
fi

# ── Collect patch files tracked by git ───────────────────────────────────────
# These are the files inside tinygo-bluetooth/ that are committed in this repo.
# We save them before potentially blowing away the directory.
PATCH_FILES=()
while IFS= read -r f; do
  PATCH_FILES+=("$f")
done < <(git -C "${REPO_ROOT}" ls-files "tinygo-bluetooth/" 2>/dev/null | grep -v '^tinygo-bluetooth/\.gitignore$' || true)

# ── Check if already set up ───────────────────────────────────────────────────
if [[ -f "${TARGET_DIR}/go.mod" ]] && [[ "${FORCE}" == "false" ]]; then
  echo "✓ tinygo-bluetooth already set up (use --force to re-clone)"
  # Still re-apply patches in case they changed
  _apply_patches=true
else
  _apply_patches=false

  echo "▶ Cloning tinygo-org/bluetooth ${UPSTREAM_TAG}..."
  git clone \
    --depth 1 \
    --branch "${UPSTREAM_TAG}" \
    "${UPSTREAM_URL}" \
    "${TMP_DIR}" \
    --quiet

  # Remove the upstream .git so it is a plain directory
  rm -rf "${TMP_DIR}/.git"

  # Swap in the cloned content, preserving our tracked patch files
  echo "▶ Installing upstream source into ${TARGET_DIR}/..."

  # Save our patch files to a temp holding area
  PATCH_HOLD="$(mktemp -d)"
  for pf in "${PATCH_FILES[@]}"; do
    rel="${pf#tinygo-bluetooth/}"   # strip the tinygo-bluetooth/ prefix
    if [[ -f "${TARGET_DIR}/${rel}" ]]; then
      mkdir -p "${PATCH_HOLD}/$(dirname "${rel}")"
      cp "${TARGET_DIR}/${rel}" "${PATCH_HOLD}/${rel}"
    fi
  done

  # Wipe existing content (but keep .gitignore)
  find "${TARGET_DIR}" -mindepth 1 -not -name '.gitignore' -delete 2>/dev/null || true

  # Copy upstream files in
  cp -r "${TMP_DIR}/." "${TARGET_DIR}/"

  # Restore .gitignore (cp above may have overwritten it with upstream's)
  if [[ -f "${REPO_ROOT}/tinygo-bluetooth/.gitignore" ]]; then
    # Our .gitignore is tracked by git — git checkout restores it
    git -C "${REPO_ROOT}" checkout -- tinygo-bluetooth/.gitignore 2>/dev/null || true
  fi

  _apply_patches=true
fi

# ── Apply patch files ─────────────────────────────────────────────────────────
if [[ "${_apply_patches}" == "true" ]] && [[ ${#PATCH_FILES[@]} -gt 0 ]]; then
  echo "▶ Applying ${#PATCH_FILES[@]} patch file(s)..."
  for pf in "${PATCH_FILES[@]}"; do
    rel="${pf#tinygo-bluetooth/}"
    src="${REPO_ROOT}/tinygo-bluetooth/${rel}"
    if [[ -f "${src}" ]]; then
      echo "  ✎ ${rel}"
      # The file is already in place (it's a tracked file in the repo).
      # If the upstream clone overwrote it, restore from the holding area.
      if [[ -d "${PATCH_HOLD:-}" ]] && [[ -f "${PATCH_HOLD}/${rel}" ]]; then
        cp "${PATCH_HOLD}/${rel}" "${TARGET_DIR}/${rel}"
      fi
    fi
  done
fi

# ── Cleanup ───────────────────────────────────────────────────────────────────
rm -rf "${TMP_DIR}" "${PATCH_HOLD:-}"

echo "✅ tinygo-bluetooth is ready (${UPSTREAM_TAG})"
