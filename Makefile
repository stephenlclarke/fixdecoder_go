# SPDX-License-Identifier: AGPL-3.0-only
# SPDX-FileCopyrightText: 2026 Steve Clarke <stephenlclarke@mac.com> - https://xyzzy.tools
#
# fixdecoder command-line entry point and CLI orchestration.
#
# The binary ties together the dictionary tooling and the streaming FIX log
# prettifier.  This file is intentionally light on protocol logic; it wires
# user input into the focused modules under `src/decoder` and `src/fix`.
# The comments favour UK English and aim to give future maintainers a quick
# reminder of why each function exists and how it cooperates with the rest
# of the app.

# Makefile — delegates all real work to ./ci.sh
# --------------------------------------------

CI_SCRIPT := ./ci.sh

# The targets your pipeline (and developers) will call
.PHONY: build build-all unit-test integration-test scan security-scan help

# Straight-through wrappers: “make build” → “./ci.sh build”, etc.
build build-all unit-test integration-test scan:
	$(CI_SCRIPT) $@

# Alias so `make security-scan` feels natural
security-scan:
	$(CI_SCRIPT) scan

# Simple help text
help:
	@echo "Available targets:"
	@echo "  build              → $(CI_SCRIPT) build"
	@echo "  build-all          → $(CI_SCRIPT) build-all"
	@echo "  unit-test          → $(CI_SCRIPT) unit-test"
	@echo "  integration-test   → $(CI_SCRIPT) integration-test"
	@echo "  scan               → $(CI_SCRIPT) scan"
