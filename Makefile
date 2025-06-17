.PHONY: all tidy install-deps generate-fix44 test unit-test integration-test junit-report version build clean coverage-html sonar-scan gitleaks-scan module-reports scan

export PATH := $(shell go env GOPATH)/bin:$(PATH)
export GOPRIVATE=bitbucket.org/edgewater/fixdecoder

# Variables
BINARY_NAME=fixdecoder
OUTPUT_DIR=./bin
OUTPUT_PATH=$(OUTPUT_DIR)/$(BINARY_NAME)
REPORT_DIR=./reports
VERSION ?= $(shell git describe --tags --abbrev=0 2>/dev/null || echo v0.0.0)
BRANCH ?= $(shell git rev-parse --abbrev-ref HEAD)
SHORT_SHA ?= $(shell git rev-parse --short HEAD)
URL ?= $(shell git remote get-url origin)
MODULES = \
	cmd/fixdecoder \
	decoder \
	fix \
	fix/fix40 \
	fix/fix41 \
	fix/fix42 \
	fix/fix43 \
	fix/fix44 \
	fix/fix50 \
	fix/fix50SP1 \
	fix/fix50SP2 \
	fix/fixT11

FIX_SCHEMA_DIRS := \
	fix/fix40 \
	fix/fix41 \
	fix/fix42 \
	fix/fix43 \
	fix/fix44 \
	fix/fix50 \
	fix/fix50SP1 \
	fix/fix50SP2 \
	fix/fixT11

all: generate-fix tidy install-deps test build

tidy:
	@echo "🔧 Running go mod tidy in all modules"
	@for dir in cmd/fixdecoder decoder fix $(FIX_SCHEMA_DIRS); do \
		if [ -d $$dir ]; then \
			echo " - Tidying $$dir"; \
			cd $$dir && go mod tidy && cd - >/dev/null; \
		else \
			echo " - Skipping $$dir (not found)"; \
		fi \
	done
	
install-deps:
	@echo "📦 Installing test dependencies"
	go install github.com/jstemmer/go-junit-report/v2@latest

generate-fix:
	@echo "🛠️  Generating FIX dictionary"
	chmod +x ./resources/generate_fix_go.sh
	./resources/generate_fix_go.sh

test: unit-test integration-test junit-report module-reports

unit-test:
	@echo "🧪 Running unit tests (module-by-module)"
	@mkdir -p reports
	@rm -f coverage.out
	@for module in $(MODULES); do \
		echo " - Testing $$module"; \
		abs_report_path=$$(cd reports && pwd)/coverage-`basename $$module`.out; \
		cd $$module && go test -v -covermode=atomic -coverprofile=$$abs_report_path . || exit 1; \
		cd - >/dev/null; \
	done
	@echo "📦 Merging coverage reports"
	@echo "mode: atomic" > "$$REPORT_DIR/coverage.out"
	@find . -name 'coverage-*.out' -exec tail -n +2 {} \; >> "$$REPORT_DIR/coverage.out"

integration-test:
	@echo "🔁 Running integration tests"
	go test -v -tags=integration -covermode=atomic -coverpkg=./... -coverprofile="$$REPORT_DIR/coverage.integration.out" ./...
	go test -tags=integration -timeout=10m -run '^TestMain' ./...

junit-report:
	@echo "📄 Generating JUnit test report"
	go test -json ./... | go-junit-report > "$$REPORT_DIR/test_report.xml"

module-reports:
	@echo "📄 Generating JUnit test reports per module"
	@mkdir -p reports
	@REPORT_DIR=$$(cd reports && pwd); \
	for module in $(MODULES); do \
		report_name=$$(echo $$module | tr / -); \
		echo " - Creating report for $$report_name"; \
		cd $$module && \
		go test -json . | go-junit-report > "$$REPORT_DIR/test-report-$$report_name.xml" || true; \
		cd - >/dev/null; \
	done

version:
	@echo "🔖 Version: $(VERSION)"
	@echo "🌿 Branch:  $(BRANCH)"
	@echo "🔢 SHA:     $(SHORT_SHA)"
	@echo "🌐 Repo:    $(URL)"

build:
	@echo "🔨 Building application"
	@mkdir -p $(OUTPUT_DIR)
	go build -ldflags="-X main.Version=$(VERSION) -X main.Branch=$(BRANCH) -X main.Sha=$(SHORT_SHA) -X main.Url=$(URL)" \
		-o $(OUTPUT_PATH) ./cmd/fixdecoder
	@echo "✅ Build complete: $(OUTPUT_PATH)"

coverage-html:
	@if [ -n "$$CI" ]; then \
		echo "❌ HTML coverage generation not allowed in CI"; \
		exit 1; \
	else \
		@echo "📊 Generating HTML coverage report"; \
		go tool cover -html=coverage.out -o $(REPORT_DIR)/coverage.html; \
		@echo "📂 HTML report available at $(REPORT_DIR)/coverage.html"; \
	fi

scan:
	@echo "🔍 Running static analysis and upload"
	@if [ -z "$$CI" ]; then \
		echo "🔄 Running SonarQube scan"; \
		docker run --rm -e SONAR_TOKEN="$$SONAR_TOKEN" -v "$(PWD):/usr/src" sonarsource/sonar-scanner-cli; \
	else \
		echo "❌ SonarQube scan not allowed in CI"; \
	fi
	
clean:
	@echo "🧹 Cleaning up..."
	rm -rf $(OUTPUT_DIR) coverage*.out test_report.xml $(REPORT_DIR)/coverage.html $(REPORT_DIR)
	