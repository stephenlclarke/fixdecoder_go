#!/bin/bash

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

function setup_environment {
    log_message "Setting up environment"
    export GOPATH=$(go env GOPATH)
    export PATH="$(go env GOPATH)/bin:$PATH"
}

function preferred_remote_name {
    if git remote get-url github >/dev/null 2>&1; then
        echo github
        return
    fi

    if git remote get-url origin >/dev/null 2>&1; then
        echo origin
        return
    fi

    return 1
}

function preferred_remote_url {
    local remote
    remote=$(preferred_remote_name) || return 1
    git remote get-url "$remote"
}
function install_dependencies {
    log_message "Installing test dependencies"
    go install github.com/jstemmer/go-junit-report/v2@latest
}

function tidy {
    log_message "Running go mod tidy in all modules"
    go mod tidy
    go mod download
}

function generate_fix {
    log_message "Auto-Generating FIX dictionary"
    chmod +x ./resources/generate_fix_go.sh
    ./resources/generate_fix_go.sh
}   

function run_unit_tests {
    log_message "Running unit tests"
    mkdir -p reports
    rm -f coverage.out
    for module in ${MODULES[@]}; do
        echo " - Testing $module"
        abs_report_path=$(cd reports && pwd)/coverage-`basename $module`.out
        cd $module
        go test -v -covermode=atomic -coverprofile=$abs_report_path .
        cd - >/dev/null
    done

    log_message "Merging coverage reports"
    echo "mode: atomic" > reports/coverage.out
    find . -name 'coverage-*.out' -exec tail -n +2 {} \; >> reports/coverage.out

    log_message "Generating JUnit test reports per module"
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

    log_message "Generating unit test report"
    go test -json ./... | go-junit-report > reports/unit-test-report.xml

    log_message "Generating HTML coverage report"
    go tool cover -html=reports/coverage.out -o reports/coverage.html
    log_message "HTML report available at reports/coverage.html"
}

function run_integration_tests {
    log_message "Running integration tests"
    mkdir -p reports
    go test -v -tags=integration -covermode=atomic -coverpkg=./... -coverprofile=reports/coverage.integration.out ./...
}

function build_application {
    local remote

    log_message "Building the application"
    # ensure the bin directory exists
    mkdir -p ./bin

    # ensure your tags are fetched
    remote=$(preferred_remote_name) || true
    if [[ -n "${remote:-}" ]]; then
        git fetch "$remote" --tags --force
    fi

    # grab the latest tag and construct a version string (e.g. "v1.2.3")
    TAG=$(git describe --tags --abbrev=0 2>/dev/null)
    VERSION=${TAG:="v0.0.0"}
    [[ -n "$(git status --porcelain)" ]] && VERSION="${VERSION}-dirty"

    BRANCH=$(git rev-parse --abbrev-ref HEAD)
    SHORT_SHA=$(git rev-parse --short HEAD)

    URL=$(preferred_remote_url 2>/dev/null || true)

    log_message "Building the application ${VERSION} (branch: ${BRANCH}, commit: ${SHORT_SHA})"

    # build with that version baked in
    go build -ldflags="-X main.Version=${VERSION} -X main.Branch=${BRANCH} -X main.Sha=${SHORT_SHA} -X main.GitUrl=${URL}" -o ./bin/fixdecoder ./cmd/fixdecoder
}

setup_environment
install_dependencies
tidy
run_unit_tests
run_integration_tests
build_application

log_message "Build complete!"
