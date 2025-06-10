#!/bin/bash
function log_message {
    echo -e "\n\033[1;32m$1\033[0m"
}

log_message "Setting up Go environment"
export PATH="$(go env GOPATH)/bin:$PATH"
go env -w GOPRIVATE=bitbucket.org/edgewater/fixdecoder

go mod tidy
go mod download
go install github.com/jstemmer/go-junit-report/v2@latest

log_message "Generating FIX44 dictionary"
chmod +x ./resources/generate_fix_go.sh
./resources/generate_fix_go.sh

log_message "Running unit tests"
# unit tests
go test -v -covermode=atomic -coverpkg=./... -coverprofile=coverage.out ./...

log_message "Running integration tests"

# integration tests
go test -v -tags=integration -covermode=atomic -coverpkg=./... -coverprofile=coverage.integration.out ./...
go test -tags=integration -timeout=10m -run '^TestMain' ./...

log_message "Generating test unit report"
# produce JUnit XML
go test -json ./... | go-junit-report > test_report.xml

# ensure your tags are fetched
git fetch --tags

# grab the latest tag and construct a version string (e.g. "v1.2.3")
TAG=$(git describe --tags --abbrev=0 2>/dev/null)
VERSION=${TAG:="v0.0.0"}
git status --porcelain >/dev/null 2>&1 && VERSION="${VERSION}-dirty"

BRANCH=$(git rev-parse --abbrev-ref HEAD)
SHORT_SHA=$(git rev-parse --short HEAD)

URL=$(git remote get-url origin)

log_message "Building the application ${VERSION} (branch: ${BRANCH}, commit: ${SHORT_SHA})"

# build with that version baked in
go build -ldflags="-X main.Version=${VERSION} -X main.Branch=${BRANCH} -X main.Sha=${SHORT_SHA} -X main.Url=${URL}" -o fixdecoder

log_message "Build complete!"