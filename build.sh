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
    local message=$1
    echo -e "\n\033[1;32m${message}\033[0m"
}

function setup_environment {
    local go_path

    log_message "Setting up environment"
    go_path=$(go env GOPATH)
    export GOPATH="${go_path}"
    export PATH="${go_path}/bin:$PATH"

    return 0
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

function latest_tag {
    git tag --sort=-version:refname | head -n 1

    return 0
}
function install_dependencies {
    log_message "Installing test dependencies"
    go install github.com/jstemmer/go-junit-report/v2@latest

    return 0
}

function tidy {
    log_message "Running go mod tidy in all modules"
    go mod tidy
    go mod download

    return 0
}

function generate_fix {
    log_message "Auto-Generating FIX dictionary"
    chmod +x ./resources/generate_fix_go.sh
    ./resources/generate_fix_go.sh

    return 0
}

function run_unit_tests {
    local report_dir
    local abs_report_path
    local report_name

    log_message "Running unit tests"
    mkdir -p reports
    rm -f coverage.out
    report_dir=$(cd reports && pwd)
    for module in "${MODULES[@]}"; do
        echo " - Testing $module"
        abs_report_path="${report_dir}/coverage-$(basename "$module").out"
        cd "$module"
        go test -v -covermode=atomic -coverprofile="${abs_report_path}" .
        cd - >/dev/null
    done

    log_message "Merging coverage reports"
    echo "mode: atomic" > reports/coverage.out
    find . -name 'coverage-*.out' -exec tail -n +2 {} \; >> reports/coverage.out

    log_message "Generating JUnit test reports per module"
    mkdir -p reports
    for module in "${MODULES[@]}"; do
        report_name=$(echo "$module" | tr / -)
        printf " - Creating report for %s\n" "$report_name"
        abs_report_path="${report_dir}/test-report-${report_name}.xml"
        cd "$module"
        go test -json . | go-junit-report > "${abs_report_path}"
        cd - >/dev/null
    done

    log_message "Generating unit test report"
    go test -json ./... | go-junit-report > reports/unit-test-report.xml

    log_message "Generating HTML coverage report"
    go tool cover -html=reports/coverage.out -o reports/coverage.html
    log_message "HTML report available at reports/coverage.html"

    return 0
}

function run_integration_tests {
    log_message "Running integration tests"
    mkdir -p reports
    go test -v -tags=integration -covermode=atomic -coverpkg=./... -coverprofile=reports/coverage.integration.out ./...

    return 0
}

function build_application {
    local remote
    local tag
    local version
    local branch
    local short_sha
    local url

    log_message "Building the application"
    # ensure the bin directory exists
    mkdir -p ./bin

    # ensure your tags are fetched
    remote=$(preferred_remote_name) || true
    if [[ -n "${remote:-}" ]]; then
        git fetch "$remote" --tags --force || true
    fi

    # Grab the highest version tag available, even in shallow CI clones.
    tag=$(latest_tag)
    version=${tag:="v0.0.0"}
    [[ -n "$(git status --porcelain)" ]] && version="${version}-dirty"

    branch=$(git rev-parse --abbrev-ref HEAD)
    short_sha=$(git rev-parse --short HEAD)

    url=$(preferred_remote_url 2>/dev/null || true)

    log_message "Building the application ${version} (branch: ${branch}, commit: ${short_sha})"

    # build with that version baked in
    go build -ldflags="-X main.Version=${version} -X main.Branch=${branch} -X main.Sha=${short_sha} -X main.GitUrl=${url}" -o ./bin/fixdecoder ./cmd/fixdecoder

    return 0
}

setup_environment
install_dependencies
tidy
run_unit_tests
run_integration_tests
build_application

log_message "Build complete!"
