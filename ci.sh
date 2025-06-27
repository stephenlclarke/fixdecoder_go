#!/usr/bin/env bash
# ---------------------------------------------------------------------------
# Unified CI helper for the fixdecoder project
#
#   scripts/ci.sh build             – compile binary (was build.sh → build)
#   scripts/ci.sh unit-test         – unit tests + coverage  (was build.sh → unit)
#   scripts/ci.sh integration-test  – integration tests      (was build.sh → integration)
#   scripts/ci.sh scan              – gitleaks secret scan   (was scan.sh)
#
# Every build-related target runs the common preparation steps that lived
# in build.sh:  setup_environment → install_dependencies → tidy → generate_fix
# ---------------------------------------------------------------------------

set -euo pipefail

# ──────────────────────────────────────────────────────────────
#  Constants
# ──────────────────────────────────────────────────────────────
MODULES=(
	"cmd/fixdecoder"
	"decoder"
	"fix"
	"fix/fix40"
	"fix/fix41"
	"fix/fix42"
	"fix/fix43"
	"fix/fix44"
	"fix/fix50"
	"fix/fix50SP1"
	"fix/fix50SP2"
	"fix/fixT11"
)

function log_message {
  echo -e "\n\033[1;32m$1\033[0m"
}

function setup_environment() {
  log_message ">> Setting up environment"
  export GOPATH=$(go env GOPATH)
  export PATH="$(go env GOPATH)/bin:$PATH"
  go env -w GOPRIVATE=bitbucket.org/edgewater/fixdecoder
}

function install_dependencies() {
  log_message ">> Installing test dependencies"
  go install github.com/jstemmer/go-junit-report/v2@latest
}

function tidy() {
  log_message ">> Running go mod tidy in all modules"
  go mod tidy
  go mod download
}

function generate_fix() {
  echo ">> Auto-Generating FIX dictionary"
  chmod +x ./resources/generate_fix_go.sh
  ./resources/generate_fix_go.sh
}

function unit_tests() {
  log_message ">>Running unit tests"
  mkdir -p reports
  rm -f coverage.out
  for module in ${MODULES[@]}; do
      echo " - Testing $module"
      abs_report_path=$(cd reports && pwd)/coverage-`basename $module`.out
      cd $module
      go test -v -covermode=atomic -coverprofile=$abs_report_path .
      cd - >/dev/null
  done

  log_message ">> Merging coverage reports"
  echo "mode: atomic" > reports/coverage.out
  find . -name 'coverage-*.out' -exec tail -n +2 {} \; >> reports/coverage.out

  log_message ">> Generating JUnit test reports per module"
  mkdir -p reports
  REPORT_DIR=$(cd reports && pwd)
  for module in ${MODULES[@]}; do
      report_name=$(echo $module | tr / -)
      printf " - Creating report for %s\n" $report_name
      abs_report_path=$(cd reports && pwd)/test-report-$report_name.xml
      cd $module
      go test -json . | go-junit-report > $abs_report_path
      cd - >/dev/null
  done

  log_message ">> Generating unit test report"
  go test -json ./... | go-junit-report > reports/unit-test-report.xml

  log_message ">> Generating HTML coverage report"
  go tool cover -html=reports/coverage.out -o reports/coverage.html
  log_message "HTML report available at reports/coverage.html"
}

function compile_binary() {
    # ensure the bin directory exists
    mkdir -p ./bin

    # ensure your tags are fetched
    git fetch --tags

    # grab the latest tag and construct a version string (e.g. "v1.2.3")
    TAG=$(git describe --tags --abbrev=0 2>/dev/null)
    VERSION=${TAG:="v0.0.0"}
    git status --porcelain >/dev/null 2>&1 && VERSION="${VERSION}-dirty"

    BRANCH=$(git rev-parse --abbrev-ref HEAD)
    SHORT_SHA=$(git rev-parse --short HEAD)

    URL=$(git remote get-url origin)

    log_message ">> Building the application ${VERSION} (branch: ${BRANCH}, commit: ${SHORT_SHA})"

    # build with that version baked in
    go build -ldflags="-X main.Version=${VERSION} -X main.Branch=${BRANCH} -X main.Sha=${SHORT_SHA} -X main.Url=${URL}" -o ./bin/fixdecoder ./cmd/fixdecoder
}

function integration_tests() {
  log_message ">> Running integration tests"
  # integration tests
  go test -v -tags=integration -covermode=atomic -coverpkg=./... -coverprofile=reports/coverage.integration.out ./...
  go test -tags=integration -timeout=10m -run '^TestMain' ./...
}

function code_scan() {
  echo ">> SonarQube Scan"
  docker run --rm -e SONAR_TOKEN="${SONAR_TOKEN}" -v "$(pwd):/usr/src" sonarsource/sonar-scanner-cli
}

# Helper that runs the common pre-build steps in order
function common_preparation() {
  setup_environment
  install_dependencies
  tidy
  generate_fix
}

# Target dispatcher
target=${1:-""}
case "$target" in
  all)
    common_preparation
    compile_binary
    unit_tests
    integration_tests
    code_scan
    ;;

  build)
    common_preparation
    compile_binary
    ;;

  unit-test)
    common_preparation
    unit_tests
    ;;

  integration-test)
    common_preparation
    integration_tests
    ;;

  scan)
    code_scan
    ;;

  *)
    echo "usage: $0 {all|build|unit-test|integration-test|scan}"
    exit 1
    ;;
esac
