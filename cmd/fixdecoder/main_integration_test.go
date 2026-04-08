//go:build integration

// SPDX-License-Identifier: AGPL-3.0-only
// SPDX-FileCopyrightText: 2026 Steve Clarke <stephenlclarke@mac.com> - https://xyzzy.tools
//
/// fixdecoder command-line entry point and CLI orchestration.
///
/// The binary ties together the dictionary tooling and the streaming FIX log
/// prettifier.  This file is intentionally light on protocol logic; it wires
/// user input into the focused modules under `src/decoder` and `src/fix`.
/// The comments favour UK English and aim to give future maintainers a quick
/// reminder of why each function exists and how it cooperates with the rest
/// of the app.

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func buildBinary(t *testing.T) string {
	t.Helper()

	name := "fixdecoder"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}

	path := filepath.Join(t.TempDir(), name)
	cmd := exec.Command("go", "build", "-o", path, ".")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build failed: %v\n%s", err, output)
	}

	return path
}

func writeIntegrationTempFile(t *testing.T, name, contents string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) failed: %v", path, err)
	}

	return path
}

func TestBinaryUsesExternalXMLForPrettify(t *testing.T) {
	binary := buildBinary(t)
	xmlPath := writeIntegrationTempFile(t, "schema.xml", `<fix major="4" minor="4"><fields><field number="35" name="ExternalMsgType"><value enum="A" description="ExternalLogon"/></field></fields><messages></messages><components></components><header></header><trailer></trailer></fix>`)
	logPath := writeIntegrationTempFile(t, "fix.log", "8=FIX.4.4\x0135=A\x0110=123\x01\n")

	cmd := exec.Command(binary, "-xml", xmlPath, logPath)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("binary run failed: %v\nstderr=%s", err, stderr.String())
	}

	if !strings.Contains(stdout.String(), "ExternalMsgType") || !strings.Contains(stdout.String(), "ExternalLogon") {
		t.Fatalf("expected binary output to use external XML lookup, stdout=%q", stdout.String())
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestBinaryInfoReportsExternalDictionary(t *testing.T) {
	binary := buildBinary(t)
	xmlPath := writeIntegrationTempFile(t, "schema.xml", `<fix major="4" minor="4"><fields><field number="35" name="MsgType"/></fields><messages></messages><components></components><header></header><trailer></trailer></fix>`)

	cmd := exec.Command(binary, "-xml", xmlPath, "-info")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("binary run failed: %v\nstderr=%s", err, stderr.String())
	}

	if !strings.Contains(stdout.String(), "Dictionary loaded from:") || !strings.Contains(stdout.String(), "Current Schema:") {
		t.Fatalf("expected schema summary output, stdout=%q", stdout.String())
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}
