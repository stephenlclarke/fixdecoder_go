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
	"path/filepath"
	"strings"
	"testing"
)

const processSuccessFormat = "Process() = %d, want 0; stderr=%q"

func writeTempFile(t *testing.T, name, contents string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) failed: %v", path, err)
	}

	return path
}

func TestParseFlagsArgsSupportsOptionalValueWithoutEquals(t *testing.T) {
	var errOut bytes.Buffer

	opts, err := parseFlagsArgs([]string{"--message", "A"}, &errOut)
	if err != nil {
		t.Fatalf("parseFlagsArgs() returned error: %v", err)
	}

	if !opts.Message.isSet || opts.Message.value != "A" {
		t.Fatalf("expected --message A to set message A, got %+v", opts.Message)
	}

	if len(opts.FileArgs) != 1 || opts.FileArgs[0] != "-" {
		t.Fatalf("expected stdin default file args, got %v", opts.FileArgs)
	}
}

func TestProcessUsesParsedPositionalArgs(t *testing.T) {
	logPath := writeTempFile(t, "fix.log", "8=FIX.4.4\x0135=A\x0110=123\x01\n")

	var out bytes.Buffer
	var errOut bytes.Buffer
	code := Process([]string{"--fix", "44", logPath}, &out, &errOut)

	if code != 0 {
		t.Fatalf(processSuccessFormat, code, errOut.String())
	}

	if strings.Contains(errOut.String(), "open 44") {
		t.Fatalf("expected parsed flag value not to be treated as file, stderr=%q", errOut.String())
	}

	if !strings.Contains(out.String(), "Processing:") {
		t.Fatalf("expected file to be processed, output=%q", out.String())
	}
}

func TestProcessXMLUsesExternalDictionaryForPrettifyFiles(t *testing.T) {
	xmlPath := writeTempFile(t, "schema.xml", `<fix major="4" minor="4"><fields><field number="35" name="ExternalMsgType"><value enum="A" description="ExternalLogon"/></field></fields><messages></messages><components></components><header></header><trailer></trailer></fix>`)
	logPath := writeTempFile(t, "fix.log", "8=FIX.4.4\x0135=A\x0110=123\x01\n")

	var out bytes.Buffer
	var errOut bytes.Buffer
	code := Process([]string{"--xml", xmlPath, logPath}, &out, &errOut)
	if code != 0 {
		t.Fatalf(processSuccessFormat, code, errOut.String())
	}

	if !strings.Contains(out.String(), "ExternalMsgType") || !strings.Contains(out.String(), "ExternalLogon") {
		t.Fatalf("expected prettifier to use external XML lookup, output=%q", out.String())
	}
}

func TestProcessReturnsNonZeroForUnknownFlag(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	code := Process([]string{"--unknown"}, &out, &errOut)
	if code == 0 {
		t.Fatalf("Process() = %d, want non-zero", code)
	}

	if !strings.Contains(errOut.String(), "flag provided but not defined") {
		t.Fatalf("expected parse error on stderr, got %q", errOut.String())
	}
}

func TestProcessRejectsSingleDashLongOptions(t *testing.T) {
	for _, arg := range []string{"-info", "-version", "-help"} {
		t.Run(arg, func(t *testing.T) {
			var out bytes.Buffer
			var errOut bytes.Buffer

			code := Process([]string{arg}, &out, &errOut)
			if code != 2 {
				t.Fatalf("Process() = %d, want 2", code)
			}

			if !strings.Contains(errOut.String(), "must be written as --") {
				t.Fatalf("expected long-option guidance on stderr, got %q", errOut.String())
			}

			if out.String() != "" {
				t.Fatalf("expected no stdout output, got %q", out.String())
			}
		})
	}
}

func TestProcessWarnsOnUnsupportedFixVersion(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	code := Process([]string{"--fix=99", "--message=A"}, &out, &errOut)
	if code != 0 {
		t.Fatalf(processSuccessFormat, code, errOut.String())
	}

	if !strings.Contains(errOut.String(), `Unsupported FIX version "99"`) {
		t.Fatalf("expected unsupported FIX warning, got %q", errOut.String())
	}

	if !strings.Contains(out.String(), "Message:") {
		t.Fatalf("expected handler output to be written to supplied writer, output=%q", out.String())
	}
}

func TestProcessPrintsVersion(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	code := Process([]string{"--version"}, &out, &errOut)
	if code != 0 {
		t.Fatalf(processSuccessFormat, code, errOut.String())
	}

	if !strings.Contains(out.String(), "fixdecoder ") {
		t.Fatalf("expected version output, got %q", out.String())
	}

	if errOut.String() != "" {
		t.Fatalf("expected no stderr output, got %q", errOut.String())
	}
}

func TestProcessPrintsHelp(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"-h"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var out bytes.Buffer
			var errOut bytes.Buffer

			code := Process(args, &out, &errOut)
			if code != 0 {
				t.Fatalf(processSuccessFormat, code, errOut.String())
			}

			help := errOut.String()
			if !strings.Contains(help, "--help") || !strings.Contains(help, "--info") || !strings.Contains(help, "--version") {
				t.Fatalf("expected GNU-style long flags in help output, got %q", help)
			}

			if out.String() != "" {
				t.Fatalf("expected help output on stderr, got stdout=%q", out.String())
			}
		})
	}
}
