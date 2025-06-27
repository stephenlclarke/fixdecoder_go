# Makefile — delegates all real work to ./ci.sh
# --------------------------------------------

CI_SCRIPT := ./ci.sh

# The targets your pipeline (and developers) will call
.PHONY: build unit-test integration-test scan security-scan help

# Straight-through wrappers: “make build” → “./ci.sh build”, etc.
build unit-test integration-test scan:
	$(CI_SCRIPT) $@

# Alias so `make security-scan` feels natural
security-scan:
	$(CI_SCRIPT) scan

# Simple help text
help:
	@echo "Available targets:"
	@echo "  build              → $(CI_SCRIPT) build"
	@echo "  unit-test          → $(CI_SCRIPT) unit-test"
	@echo "  integration-test   → $(CI_SCRIPT) integration-test"
	@echo "  scan.              → $(CI_SCRIPT) scan"