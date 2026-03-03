# Makefile — invokes helper functions from ./ci/ci_helper.sh for common tasks

SHELL := /bin/bash
CI_SCRIPT := ./ci/ci_helper.sh
ZIG := $(shell command -v zig 2>/dev/null)

.PHONY: setup-environment prepare build build-release build-release-all scan coverage sonar release clean help
.PHONY: test-unit test-integration test-all

# Common cross targets (override ALL_TARGETS to customize). Canonical release set:
#  - Linux: x86_64-gnu, x86_64-musl, aarch64-gnu
#  - macOS: x86_64, aarch64
#  - Windows: x86_64-gnu, x86_64-msvc, aarch64-msvc
ALL_TARGETS ?= x86_64-unknown-linux-gnu x86_64-unknown-linux-musl aarch64-unknown-linux-gnu x86_64-apple-darwin aarch64-apple-darwin x86_64-pc-windows-gnu x86_64-pc-windows-msvc aarch64-pc-windows-msvc

setup-environment:
	@bash -lc 'source $(CI_SCRIPT) && cmd_setup_environment'

prepare:
	@bash -lc 'source $(CI_SCRIPT) && cmd_setup_environment && ensure_build_metadata && download_fix_specs'
	@cargo run --quiet --bin generate_sensitive_tags >/dev/null

build: prepare
	@bash -lc 'source $(CI_SCRIPT) && ensure_build_metadata && cargo fmt --all && cargo build --workspace'

build-release: prepare
	@bash -lc 'source $(CI_SCRIPT) && ensure_build_metadata && cargo fmt --all && cargo build --workspace --release'

build-release-all: prepare
	@bash -lc '\
		source $(CI_SCRIPT) && ensure_build_metadata && cargo fmt --all; \
		if [[ -n "$(ZIG)" ]]; then \
			echo ">> Using zig for cross-linking where applicable"; \
			mkdir -p target/zig-linkers; \
			make_wrap() { \
				local tgt="$$1"; local kind="$$2"; local file="target/zig-linkers/zig-$$tgt-$$kind"; \
				echo "#!/usr/bin/env bash" > $$file; \
				if [[ $$kind == "cc" ]]; then \
					echo "exec \"$(ZIG)\" cc -target $$tgt \"\$$@\"" >> $$file; \
				else \
					echo "exec \"$(ZIG)\" ar \"\$$@\"" >> $$file; \
				fi; \
				chmod +x $$file; \
				echo $$file; \
			}; \
			export CARGO_TARGET_X86_64_UNKNOWN_LINUX_GNU_LINKER=$$(make_wrap x86_64-linux-gnu cc); \
			export AR_x86_64_unknown_linux_gnu=$$(make_wrap x86_64-linux-gnu ar); \
			export CARGO_TARGET_AARCH64_UNKNOWN_LINUX_GNU_LINKER=$$(make_wrap aarch64-linux-gnu cc); \
			export AR_aarch64_unknown_linux_gnu=$$(make_wrap aarch64-linux-gnu ar); \
			export CARGO_TARGET_X86_64_PC_WINDOWS_GNU_LINKER=$$(make_wrap x86_64-windows-gnu cc); \
			export AR_x86_64_pc_windows_gnu=$$(make_wrap x86_64-windows-gnu ar); \
		else \
			echo ">> zig not found; cross targets may require platform toolchains in PATH"; \
		fi; \
		for tgt in $(ALL_TARGETS); do \
			echo ">> Building --release for target $$tgt"; \
			rustup target add $$tgt >/dev/null 2>&1 || true; \
			cargo build --workspace --release --target $$tgt || exit $$?; \
		done \
	'

scan: prepare
	@bash -lc '\
		source $(CI_SCRIPT) && \
		ensure_build_metadata && \
		cargo fmt --all --check && \
		cargo clippy --workspace --all-targets -- -D warnings && \
		if command -v yamllint >/dev/null 2>&1; then \
			yamllint .github/workflows || true; \
		else \
			echo "yamllint not installed; skipping YAML lint"; \
		fi; \
		mkdir -p target/coverage && \
		if command -v cargo-audit >/dev/null 2>&1; then \
			echo "Running cargo-audit (text output)"; \
			if [ -d "$${HOME}/.cargo/advisory-db" ]; then \
				cargo audit --no-fetch || true; \
			else \
				cargo audit || true; \
			fi; \
			echo "Running cargo-audit (JSON) → target/coverage/rustsec.json"; \
			if [ -d "$${HOME}/.cargo/advisory-db" ]; then \
				cargo audit --no-fetch --json > target/coverage/rustsec.json || true; \
			else \
				cargo audit --json > target/coverage/rustsec.json || true; \
			fi; \
			echo "Converting RustSec report to Sonar generic issues (target/coverage/sonar-generic-issues.json)"; \
			python3 ci/convert_rustsec_to_sonar.py target/coverage/rustsec.json target/coverage/sonar-generic-issues.json || true; \
		else \
			echo "cargo-audit not installed; skipping security scan"; \
		fi \
	'

coverage: build
	@bash -lc '\
		source $(CI_SCRIPT) && \
		ensure_build_metadata && \
		mkdir -p target/coverage && \
		cargo llvm-cov clean --workspace >/dev/null 2>&1 || true; \
		CPU_CORES=$$(getconf _NPROCESSORS_ONLN 2>/dev/null || sysctl -n hw.ncpu 2>/dev/null || printf "4"); \
		RUST_TEST_THREADS=$$CPU_CORES \
		cargo llvm-cov \
		  --package fixdecoder \
		  --package pcap2fix \
		  --cobertura \
		  --ignore-filename-regex "src/fix/sensitive.rs|src/bin/generate_sensitive_tags.rs" \
		  --output-path target/coverage/coverage.xml \
	'

test-unit:
	@bash -lc '\
		source $(CI_SCRIPT) && \
		ensure_build_metadata && \
		cargo test --bins \
	'

test-integration:
	@bash -lc '\
		source $(CI_SCRIPT) && \
		ensure_build_metadata && \
		cargo test --tests \
	'

test-all:
	@$(MAKE) test-unit
	@$(MAKE) test-integration

sonar:
	@bash -lc '\
		source $(CI_SCRIPT) && \
		if [[ -z "$$(echo "$(MAKECMDGOALS)" | grep -E "(^| )scan( |$$)|(^| )coverage( |$$)")" ]]; then \
			$(MAKE) scan coverage; \
		fi; \
		ensure_sonar_scanner && \
		sonar-scanner -Dsonar.externalIssuesReportPaths=target/coverage/sonar-generic-issues.json \
	'

release:
	@py=$$(command -v python3 || command -v python || true); \
	if [ -z "$$py" ]; then \
		echo "python3 (or python) is required for release bumping." >&2; \
		exit 1; \
	fi; \
	ver=$$(grep -m1 '^version' Cargo.toml | sed -E 's/.*"([^"]+)".*/\1/'); \
	if [ -z "$$ver" ]; then echo "Could not read version from Cargo.toml" >&2; exit 1; fi; \
	next="$$ver"; \
	while git rev-parse "v$${next}" >/dev/null 2>&1; do \
		next=$$($$py ci/next_patch.py "$${next}"); \
	done; \
	if ! git diff --quiet || ! git diff --cached --quiet; then \
		echo "Working tree is not clean; commit or stash changes before tagging." >&2; \
		exit 1; \
	fi; \
	cleanup() { \
		rc=$$?; \
		if [ $$rc -ne 0 ]; then \
			echo "Release failed; restoring Cargo.toml/Cargo.lock" >&2; \
			git restore --staged Cargo.toml Cargo.lock >/dev/null 2>&1 || true; \
			git restore Cargo.toml Cargo.lock >/dev/null 2>&1 || true; \
		fi; \
		exit $$rc; \
	}; \
	trap 'cleanup' EXIT; \
	$$py ci/bump_version.py "$$ver" "$$next" || exit 1; \
	echo "Bumped version: $$ver -> $$next"; \
	if [ -f Cargo.lock ]; then git add Cargo.toml Cargo.lock; else git add Cargo.toml; fi; \
	git commit -m "chore(release): v$$next"; \
	git tag -a "v$$next" -m "Release v$$next"; \
	git push origin HEAD; \
	git push origin "v$$next"; \
	echo "Created and pushed tag v$$next"; \
	trap - EXIT

clean:
	@cargo clean

help:
	@echo "Available targets:"
	@echo "  setup-environment  → ensure toolchain + coverage tools"
	@echo "  prepare            → setup + build metadata + download FIX specs + regenerate generators"
	@echo "  build              → fmt + cargo build (debug)"
	@echo "  build-release      → fmt + cargo build --release"
	@echo "  build-release-all  → fmt + cargo build --release for ALL_TARGETS ($(ALL_TARGETS))"
	@echo "  scan               → fmt --check + clippy (+ cargo-audit when available)"
	@echo "  coverage           → cargo llvm-cov --cobertura"
	@echo "  sonar              → sonar-scanner (requires coverage.xml)"
	@echo "  release            → bump patch version, commit, and tag v<version>"
	@echo "  clean              → cargo clean"
	@echo "  help               → this help text"
