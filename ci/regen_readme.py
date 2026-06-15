#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
# SPDX-FileCopyrightText: 2026 Steve Clarke <stephenlclarke@mac.com> - https://xyzzy.tools
"""Regenerate README build examples for the Go implementation."""

from __future__ import annotations

import re
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
README = ROOT / "README.md"
FIXDECODER = ROOT / "bin" / "fixdecoder"
LAUNCHER = ROOT / "scripts" / "fixdecoder"
ANSI_RE = re.compile(r"\x1b\[[0-9;?]*[ -/]*[@-~]")


@dataclass(frozen=True)
class BuildExample:
    """A deterministic shell transcript for the generated Build it section."""

    display_command: str
    output: str


def main() -> int:
    """Regenerate the generated README Build it section."""
    if not README.exists():
        print(f"README not found: {README}", file=sys.stderr)
        return 1

    ensure_binary()
    original = README.read_text()
    updated = replace_section(original, "build-examples", render_build_examples_section())
    README.write_text(updated)
    print(f"Updated {README.relative_to(ROOT)} with generated build examples")
    return 0


def ensure_binary() -> None:
    """Build the local Go binary used by direct and wrapper examples."""
    subprocess.run(["make", "build"], cwd=ROOT, check=True)


def render_build_examples_section() -> str:
    """Render generated build examples using the Rust README structure."""
    examples = [
        BuildExample(
            "bash --version",
            "\n".join(render_shell_command(("bash", "--version")).splitlines()[:3]),
        ),
        BuildExample(
            "go version",
            render_shell_command(("go", "version")),
        ),
        BuildExample(
            "git clone git@github.com:stephenlclarke/fixdecoder_go.git",
            "Cloning into 'fixdecoder_go'...\n...\n❯ cd fixdecoder_go",
        ),
        BuildExample(
            "make build unit-test integration-test",
            "\n".join(
                [
                    "",
                    ">> Setting up environment",
                    ">> Running go mod tidy in all modules",
                    ">> Building the application",
                    ">> Running unit tests",
                    ">> Running integration tests",
                ]
            ),
        ),
        BuildExample(
            "make build-all",
            "\n".join(
                [
                    "",
                    ">> Building for OS: darwin, ARCH: arm64",
                    ">> Building for OS: linux, ARCH: arm64",
                    ">> Building for OS: linux, ARCH: amd64",
                    ">> Building for OS: windows, ARCH: amd64",
                ]
            ),
        ),
        BuildExample(
            "./bin/fixdecoder --version",
            render_shell_command((str(FIXDECODER), "--version")),
        ),
        BuildExample(
            "scripts/fixdecoder --version",
            render_shell_command((str(LAUNCHER), "--version")),
        ),
    ]

    return "\n".join(
        [
            "<!-- regen-readme:start --section=build-examples -->",
            "",
            "## Build it",
            "",
            "Build it from source. This requires `bash` and a recent Go toolchain.",
            "",
            "```bash",
            format_prompted_output(examples[0]),
            "```",
            "",
            "```bash",
            format_prompted_output(examples[1]),
            "```",
            "",
            "Clone the git repo.",
            "",
            "```bash",
            format_prompted_output(examples[2]),
            "```",
            "",
            "Then build it. Local builds compile the binary and run the test suites used by CI.",
            "",
            "```bash",
            format_prompted_output(examples[3]),
            "```",
            "",
            "Build all release-style fixdecoder binaries.",
            "",
            "```bash",
            format_prompted_output(examples[4]),
            "```",
            "",
            "Run it (from the release build) and check the version details:",
            "",
            "```bash",
            format_prompted_output(examples[5]),
            "```",
            "",
            "Run the same build through the source-checkout wrapper:",
            "",
            "```bash",
            format_prompted_output(examples[6]),
            "```",
            "",
            "<!-- regen-readme:end --section=build-examples -->",
            "",
            "",
        ]
    )


def render_shell_command(command: tuple[str, ...]) -> str:
    """Run a local command and return sanitized stdout plus stderr."""
    result = subprocess.run(
        list(command),
        cwd=ROOT,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        check=True,
    )
    return sanitise_output(result.stdout)


def sanitise_output(output: str) -> str:
    """Normalize process output so README diffs are stable across machines."""
    output = ANSI_RE.sub("", output)
    output = output.replace(str(ROOT) + "/", "")
    output = output.replace(str(ROOT), ".")
    output = output.replace(str(Path.home()), "~")
    output = output.replace("-dirty", "")
    return "\n".join(line.rstrip() for line in output.rstrip().splitlines())


def format_prompted_output(example: BuildExample) -> str:
    """Format a shell command with the README prompt marker."""
    body = f"❯ {example.display_command}"
    if example.output:
        body = f"{body}\n{example.output}"
    return body


def replace_section(markdown: str, section: str, block: str) -> str:
    """Replace a generated README section or insert it before Development."""
    section_re = re.compile(
        rf"<!-- regen-readme:start --section={re.escape(section)} -->\n.*?"
        rf"<!-- regen-readme:end --section={re.escape(section)} -->\n*",
        re.S,
    )
    if section_re.search(markdown):
        return section_re.sub(block, markdown)

    anchor = "\n## Development"
    if anchor in markdown:
        return markdown.replace(anchor, f"\n{block}## Development", 1)
    return f"{markdown.rstrip()}\n\n{block}"


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except Exception as exc:
        print(str(exc), file=sys.stderr)
        raise SystemExit(1)
