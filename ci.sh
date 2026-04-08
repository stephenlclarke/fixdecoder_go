#!/usr/bin/env bash

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

set -eo pipefail

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

declare common_preparation=false
declare unit_tests=false
declare integration_tests=false
declare code_scan=false

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
  log_message ">> Auto-Generating FIX dictionary"
  chmod +x ./resources/generate_fix_go.sh
  ./resources/generate_fix_go.sh
}

function unit_tests() {
  if [[ $unit_tests == true ]]; then
    return
  fi

  log_message ">> Running unit tests"
  mkdir -p reports
  rm -f coverage.out
  for module in ${MODULES[@]}; do
      log_message " - Testing $module"
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

  unit_tests=true
}

function compile_binary() {
  # ensure the bin directory exists
  mkdir -p ./bin

  # ensure your tags are fetched
  git fetch --tags

  # grab the latest tag and construct a version string (e.g. "v1.2.3")
  TAG=$(bin/>/dev/null)
  VERSION=${TAG:="v0.0.0"}
  git status --porcelain >/dev/null 2>&1 && VERSION="${VERSION}-dirty"

  BRANCH=$(git rev-parse --abbrev-ref HEAD)
  SHORT_SHA=$(git rev-parse --short HEAD)

  URL=$(git remote get-url origin)

  log_message ">> Building the application ${VERSION} (branch: ${BRANCH}, commit: ${SHORT_SHA})"

  OS=${1:-$(go env GOOS)}
  ARCH=${2:-$(go env GOARCH)}
  BUILD_TAG=""

  if [[ -z "$1" || -z "$2" ]]; then
    log_message ">> Using default OS: ${OS}, ARCH: ${ARCH}"
  else
    log_message ">> Building for OS: ${OS}, ARCH: ${ARCH}"

    BUILD_TAG="-${OS}-${ARCH}-${VERSION//./-}"
  fi


  # build with that version baked in
  env GOOS=${OS} GOARCH=${ARCH} go build -ldflags="-X main.Version=${VERSION} -X main.Branch=${BRANCH} -X main.Sha=${SHORT_SHA} -X main.Url=${URL}" -o ./bin/fixdecoder${BUILD_TAG} ./cmd/fixdecoder
}

function integration_tests() {
  if [[ $integration_tests == true ]]; then
    return
  fi

  log_message ">> Running integration tests"
  # integration tests
  go test -v -tags=integration -covermode=atomic -coverpkg=./... -coverprofile=reports/coverage.integration.out ./...
  go test -tags=integration -timeout=10m -run '^TestMain' ./...

  integration_tests=true
}

function code_scan() {
  if [[ -n "${BITBUCKET_BUILD_NUMBER:-}" ]]; then
    log_message ">> Skipping SonarQube scan in Bitbucket Pipelines"
    code_scan=true
    return
  fi

  if [[ "${code_scan:-false}" == true ]]; then
    return
  fi

  log_message ">> SonarQube Scan"
  docker run --rm -e SONAR_TOKEN="${SONAR_TOKEN}" -v "$(pwd):/usr/src" sonarsource/sonar-scanner-cli

  code_scan=true
}

# Helper that runs the common pre-build steps in order
function common_preparation() {
  if [[ $common_preparation == true ]]; then
    return
  fi

  setup_environment
  install_dependencies
  tidy
  generate_fix

  common_preparation=true
}

# Argument dispatcher
if [[ $# -eq 0 ]]; then
  log_message "usage: $0 {all|build|unit-test|integration-test|scan} [...]"
  exit 1
fi

for target in "$@"; do
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
    build-all)
      common_preparation
      compile_binary
      compile_binary darwin arm64
      compile_binary linux arm64
      compile_binary linux amd64
      compile_binary windows amd64
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
      log_message "Unknown target: $target"
      log_message "usage: $0 {all|build|unit-test|integration-test|scan} [...]"
      exit 1
      ;;
  esac
done
