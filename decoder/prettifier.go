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

package decoder

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

var (
	loadDictionary = LoadDictionary
	parseFix       = ParseFix
	streamLogFunc  = streamLog
)

const (
	ColourReset = "\033[0m"
	ColourLine  = "\033[38;5;244m"
	ColourTag   = "\033[38;5;81m"
	ColourName  = "\033[38;5;151m"
	ColourValue = "\033[38;5;228m"
	ColourEnum  = "\033[38;5;214m"
	ColourFile  = "\033[95m"
	ColourError = "\033[31m"

	maxLogLineBytes = 10 * 1024 * 1024
)

func Prettify(msg string) string {
	var sb strings.Builder

	dict := loadDictionary(msg)

	for _, fv := range parseFix(msg) {
		name := dict.GetFieldName(fv.Tag)
		desc := dict.GetEnumDescription(fv.Tag, fv.Value)

		sb.WriteString(fmt.Sprintf("    %s%d%s (%s%s%s): %s%s%s",
			ColourTag, fv.Tag, ColourReset,
			ColourName, name, ColourReset,
			ColourValue, fv.Value, ColourReset,
		))

		if desc != "" {
			sb.WriteString(fmt.Sprintf(" (%s%s%s)", ColourEnum, desc, ColourReset))
		}

		// append newline as a string instead of a rune
		sb.WriteString("\n")
	}

	return sb.String()
}

func PrettifyFiles(paths []string, out io.Writer, errOut io.Writer) int {
	hadError := false

	// 1) If no paths at all, default to stdin (unchanged behaviour)
	if len(paths) == 0 {
		if err := streamLogFunc(os.Stdin, out); err != nil {
			fmt.Fprintln(errOut, ColourError+"Error reading input:"+err.Error()+ColourReset)
			return 1
		}

		return 0
	}

	// 2) Otherwise, iterate over every supplied path.
	//    Treat the single dash "-" as a synonym for stdin.
	for _, path := range paths {
		var (
			r   io.Reader
			c   io.Closer // nil when reading stdin
			err error
		)

		if path == "-" {
			fmt.Fprint(out, "Processing: (stdin)\n\n")
			r = os.Stdin // read from pipe/tty
		} else {
			fmt.Fprint(out, "Processing: ", ColourFile, path, ColourReset, "\n\n")

			var f *os.File
			f, err = os.Open(path)
			if err != nil {
				fmt.Fprintln(errOut, ColourError+"Cannot open file:"+err.Error()+ColourReset)
				hadError = true
				continue
			}

			r, c = f, f // will close after streaming
		}

		if err = streamLogFunc(r, out); err != nil {
			fmt.Fprintln(errOut, ColourError+"Error reading file:"+err.Error()+ColourReset)
			hadError = true
		}

		if c != nil {
			c.Close()
		}
	}

	if hadError {
		return 1
	}

	return 0
}

func streamLog(in io.Reader, out io.Writer) error {
	re := regexp.MustCompile(`8=FIX.*?10=\d{3}`)
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, bufio.MaxScanTokenSize), maxLogLineBytes)

	for scanner.Scan() {
		line := scanner.Text()
		fmt.Fprint(out, ColourLine, line, ColourReset, "\n")

		if m := re.FindString(line); m != "" {
			fmt.Fprint(out, Prettify(m))
		}
	}

	return scanner.Err()
}
