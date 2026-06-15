#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
# SPDX-FileCopyrightText: 2026 Steve Clarke <stephenlclarke@mac.com> - https://xyzzy.tools
"""Regenerate README build examples for the Go implementation."""

from __future__ import annotations

import os
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
LONG_OPTION_RE = re.compile(r"(?<![\w-])--([A-Za-z][A-Za-z0-9-]*)")
MESSAGE_CODE_NAME_RE = re.compile(r"(?m)^\s*([A-Za-z0-9]{1,4})\s*:\s*([A-Za-z][A-Za-z0-9_]*)")
MESSAGE_NAME_CODE_RE = re.compile(r"(?m)^\s*([A-Za-z][A-Za-z0-9_]*)\s+\(([A-Za-z0-9]{1,4})\)")
COMPONENT_NAME_RE = re.compile(r"\b([A-Z][A-Za-z0-9_]*(?:Grp|Data|Instructions|Parties|Instrument|Trailer|Header|Hop)?)\b")
TAG_FIELD_RE = re.compile(r"(?m)^\s*([0-9]+)\s*:\s*([A-Za-z][A-Za-z0-9_]*)")
SOH = "\x01"
GROUP_TAG_CANDIDATES = ("453", "78", "802", "539", "804", "268")
PREFERRED_COMPONENTS = ("PreAllocGrp", "Parties", "Instrument")


@dataclass(frozen=True)
class BuildExample:
    """A deterministic shell transcript for the generated Build it section."""

    display_command: str
    output: str


@dataclass(frozen=True)
class ReadmeExample:
    """A deterministic README command example rendered by the Go app."""

    option: str
    display_command: str
    args: tuple[str, ...]
    stdin: str | None = None
    max_lines: int = 24


@dataclass(frozen=True)
class CapabilitySnapshot:
    """A generated summary of this implementation's discoverable CLI surface."""

    help_text: str
    options: frozenset[str]
    message_code: str
    message_name: str
    component_name: str
    group_tag: str
    group_name: str


def main() -> int:
    """Regenerate generated README sections from the current Go CLI."""
    if not README.exists():
        print(f"README not found: {README}", file=sys.stderr)
        return 1

    ensure_binary()
    capabilities = discover_capabilities()
    original = README.read_text()
    updated = replace_usage_section(original, render_usage_section(capabilities))
    updated = replace_examples_section(updated, render_examples_section(capabilities))
    updated = replace_section(
        updated,
        "capabilities",
        render_capability_section(capabilities),
        "\n<!-- regen-readme:start --section=examples -->",
    )
    updated = remove_unmarked_duplicate_build_section(updated)
    updated = replace_section(updated, "build-examples", render_build_examples_section(), "\n## Development")
    README.write_text(updated)
    print(
        f"Updated {README.relative_to(ROOT)} with generated build, usage, "
        "capability discovery, and CLI examples"
    )
    return 0


def ensure_binary() -> None:
    """Build the local Go binary used by direct and wrapper examples."""
    subprocess.run(["make", "build"], cwd=ROOT, check=True)


def fix_message(fields: list[tuple[str, str]]) -> str:
    """Build a valid single-line FIX.4.4 message for README examples."""
    body = "".join(f"{tag}={value}{SOH}" for tag, value in fields)
    prefix = f"8=FIX.4.4{SOH}9={len(body.encode('ascii'))}{SOH}"
    without_checksum = prefix + body
    checksum = sum(without_checksum.encode("ascii")) % 256
    return f"{without_checksum}10={checksum:03}{SOH}\n"


HEARTBEAT_FIX = fix_message([("35", "0"), ("49", "BUY1"), ("56", "SELL1")])


def discover_capabilities() -> CapabilitySnapshot:
    """Discover supported options and representative dictionary entries from the Go CLI."""
    help_text = run_fixdecoder(("--help",))
    options = frozenset(f"--{option}" for option in sorted(set(LONG_OPTION_RE.findall(help_text))))

    message_output = run_fixdecoder(("--fix=44", "--message", "--column"))
    message_code, message_name = choose_message(parse_messages(message_output))

    component_output = run_fixdecoder(("--fix=44", "--component", "--column"))
    component_name = choose_component(parse_components(component_output))

    group_tag, group_name = choose_group_tag(options)
    return CapabilitySnapshot(
        help_text=sanitise_output(help_text),
        options=options,
        message_code=message_code,
        message_name=message_name,
        component_name=component_name,
        group_tag=group_tag,
        group_name=group_name,
    )


def run_fixdecoder(args: tuple[str, ...], stdin: str | None = None) -> str:
    """Run the Go binary with generation-safe environment defaults."""
    env = os.environ.copy()
    env.pop("FIXDECODER_DEFAULT_ARGS", None)
    env["PAGER"] = "cat"
    result = subprocess.run(
        [str(FIXDECODER), *args],
        cwd=ROOT,
        input=stdin,
        text=True,
        capture_output=True,
        check=False,
        env=env,
    )
    output = f"{result.stdout}{result.stderr}"
    if result.returncode != 0:
        raise RuntimeError(f"Go README discovery failed for {' '.join(args)}\n{output}")
    return output


def parse_messages(output: str) -> list[tuple[str, str]]:
    """Parse message listings regardless of whether they print code-first or name-first."""
    clean = sanitise_output(output)
    found: list[tuple[str, str]] = []
    found.extend((match.group(1), match.group(2)) for match in MESSAGE_CODE_NAME_RE.finditer(clean))
    found.extend((match.group(2), match.group(1)) for match in MESSAGE_NAME_CODE_RE.finditer(clean))
    return dedupe_pairs(found)


def parse_components(output: str) -> list[str]:
    """Parse component listings from column or line-oriented output."""
    clean = sanitise_output(output)
    ignored = {"Session", "Admin", "Business", "Order", "Flow", "Pricing"}
    names = [
        match.group(1)
        for match in COMPONENT_NAME_RE.finditer(clean)
        if match.group(1) not in ignored and len(match.group(1)) > 2
    ]
    return dedupe_names(names)


def choose_message(messages: list[tuple[str, str]]) -> tuple[str, str]:
    """Choose a stable sample message, preferring NewOrderSingle when available."""
    for code, name in messages:
        if code == "D" or name == "NewOrderSingle":
            return code, name
    if messages:
        return messages[0]
    return "D", "NewOrderSingle"


def choose_component(components: list[str]) -> str:
    """Choose a stable sample component, preferring a repeating-group-heavy one."""
    for preferred in PREFERRED_COMPONENTS:
        if preferred in components:
            return preferred
    return components[0] if components else "Instrument"


def choose_group_tag(options: frozenset[str]) -> tuple[str, str]:
    """Find a representative repeating-group NumInGroup tag from the selected dictionary."""
    if "--tag" not in options:
        return "453", "NoPartyIDs"
    for tag in GROUP_TAG_CANDIDATES:
        output = run_fixdecoder(("--fix=44", f"--tag={tag}", "--verbose", "--column"))
        clean = sanitise_output(output)
        match = TAG_FIELD_RE.search(clean)
        if match and ("NUMINGROUP" in clean or match.group(2).startswith("No")):
            return match.group(1), match.group(2)
    return "453", "NoPartyIDs"


def dedupe_pairs(pairs: list[tuple[str, str]]) -> list[tuple[str, str]]:
    """Preserve first-seen message pairs while removing duplicates."""
    seen: set[tuple[str, str]] = set()
    result: list[tuple[str, str]] = []
    for pair in pairs:
        if pair in seen:
            continue
        seen.add(pair)
        result.append(pair)
    return result


def dedupe_names(names: list[str]) -> list[str]:
    """Preserve first-seen component names while removing duplicates."""
    seen: set[str] = set()
    result: list[str] = []
    for name in names:
        if name in seen:
            continue
        seen.add(name)
        result.append(name)
    return result


def build_readme_examples(capabilities: CapabilitySnapshot) -> tuple[ReadmeExample, ...]:
    """Build README examples from the options this binary actually supports."""
    examples: list[ReadmeExample] = [
        ReadmeExample(
            option="stdin",
            display_command="printf '<FIX log>' | scripts/fixdecoder",
            args=(),
            stdin=HEARTBEAT_FIX,
            max_lines=18,
        )
    ]

    def add_if_supported(option: str, example: ReadmeExample) -> None:
        if option in capabilities.options:
            examples.append(example)

    add_if_supported(
        "--info",
        ReadmeExample(
            option="--info",
            display_command="scripts/fixdecoder --info",
            args=("--info",),
            max_lines=16,
        ),
    )
    add_if_supported(
        "--message",
        ReadmeExample(
            option="--message",
            display_command=f"scripts/fixdecoder --fix=44 --message={capabilities.message_code} --column",
            args=("--fix=44", f"--message={capabilities.message_code}", "--column"),
            max_lines=26,
        ),
    )
    add_if_supported(
        "--component",
        ReadmeExample(
            option="--component",
            display_command=f"scripts/fixdecoder --fix=44 --component={capabilities.component_name} --column",
            args=("--fix=44", f"--component={capabilities.component_name}", "--column"),
            max_lines=22,
        ),
    )
    add_if_supported(
        "--tag",
        ReadmeExample(
            option="--tag",
            display_command=f"scripts/fixdecoder --fix=44 --tag={capabilities.group_tag} --verbose --column",
            args=("--fix=44", f"--tag={capabilities.group_tag}", "--verbose", "--column"),
            max_lines=24,
        ),
    )
    sample_xml = ROOT / "resources" / "FIX44.xml"
    if sample_xml.exists():
        add_if_supported(
            "--xml",
            ReadmeExample(
                option="--xml",
                display_command="scripts/fixdecoder --xml resources/FIX44.xml --fix=44 --info",
                args=("--xml", "resources/FIX44.xml", "--fix=44", "--info"),
                max_lines=16,
            ),
        )
    return tuple(examples)


def render_usage_section(capabilities: CapabilitySnapshot) -> str:
    """Render the generated full usage section."""
    return "\n".join(
        [
            "<!-- regen-readme:start --section=usage -->",
            "",
            "## Full Usage",
            "",
            "The text below is generated by running this implementation's `fixdecoder --help`.",
            "",
            "```text",
            capabilities.help_text.rstrip(),
            "```",
            "",
            "<!-- regen-readme:end --section=usage -->",
            "",
            "",
        ]
    )


def render_capability_section(capabilities: CapabilitySnapshot) -> str:
    """Render a generated snapshot of discovered options and dictionary samples."""
    options = ", ".join(f"`{option}`" for option in sorted(capabilities.options))
    message_output = limit_lines(
        sanitise_output(
            run_fixdecoder(("--fix=44", f"--message={capabilities.message_code}", "--column"))
        ),
        18,
    )
    component_output = limit_lines(
        sanitise_output(
            run_fixdecoder(("--fix=44", f"--component={capabilities.component_name}", "--column"))
        ),
        16,
    )
    group_output = limit_lines(
        sanitise_output(
            run_fixdecoder(("--fix=44", f"--tag={capabilities.group_tag}", "--verbose", "--column"))
        ),
        12,
    )
    return "\n".join(
        [
            "<!-- regen-readme:start --section=capabilities -->",
            "",
            "## Generated Capability Snapshot",
            "",
            "This snapshot is generated by `make regen-readme` by running this implementation's binary and reflects the options and dictionary surface currently available in this repository.",
            "",
            f"- Supported long options: {options}",
            f"- Sample message discovered from the dictionary: `{capabilities.message_name} ({capabilities.message_code})`",
            f"- Sample component discovered from the dictionary: `{capabilities.component_name}`",
            f"- Sample repeating group tag discovered from the dictionary: `{capabilities.group_name} ({capabilities.group_tag})`",
            "",
            "```bash",
            f"$ scripts/fixdecoder --fix=44 --message={capabilities.message_code} --column",
            message_output,
            "```",
            "",
            "```bash",
            f"$ scripts/fixdecoder --fix=44 --component={capabilities.component_name} --column",
            component_output,
            "```",
            "",
            "```bash",
            f"$ scripts/fixdecoder --fix=44 --tag={capabilities.group_tag} --verbose --column",
            group_output,
            "```",
            "",
            "<!-- regen-readme:end --section=capabilities -->",
            "",
            "",
        ]
    )


def render_examples_section(capabilities: CapabilitySnapshot) -> str:
    """Render generated command examples for the user-facing Go options."""
    blocks = [
        "<!-- regen-readme:start --section=examples -->",
        "",
        "## Generated CLI Examples",
        "",
        "These examples are generated by `make regen-readme` using the Go command-line application.",
        "",
    ]
    for example in build_readme_examples(capabilities):
        blocks.append(render_example_block(example))
    blocks.extend(["<!-- regen-readme:end --section=examples -->", "", ""])
    return "\n".join(blocks)


def render_example_block(example: ReadmeExample) -> str:
    """Run one README example and format its sanitized output as Markdown."""
    output = render_example_output(example)
    body = f"$ {example.display_command}"
    if output:
        body = f"{body}\n{output}"
    return "\n".join(
        [
            f"### `{example.option}`",
            "",
            "```bash",
            body,
            "```",
            "",
        ]
    )


def render_example_output(example: ReadmeExample) -> str:
    """Run one Go CLI example, returning a short sanitized output block."""
    output = run_fixdecoder(example.args, stdin=example.stdin)
    return limit_lines(sanitise_output(output), example.max_lines)


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
    output = output.replace(SOH, "|")
    output = output.replace(str(ROOT) + "/", "")
    output = output.replace(str(ROOT), ".")
    output = output.replace(str(Path.home()), "~")
    output = output.replace("-dirty", "")
    return "\n".join(line.rstrip() for line in output.rstrip().splitlines())


def limit_lines(output: str, max_lines: int) -> str:
    """Keep generated README examples compact."""
    lines = output.splitlines()
    if len(lines) <= max_lines:
        return "\n".join(lines)
    shown = lines[:max_lines]
    shown.append("...")
    return "\n".join(shown)


def format_prompted_output(example: BuildExample) -> str:
    """Format a shell command with the README prompt marker."""
    body = f"❯ {example.display_command}"
    if example.output:
        body = f"{body}\n{example.output}"
    return body


def replace_usage_section(markdown: str, block: str) -> str:
    """Replace generated usage or migrate the old static usage block on first run."""
    if "<!-- regen-readme:start --section=usage -->" in markdown:
        return replace_section(markdown, "usage", block, "\n## Key options")

    anchor = "\n## Key options"
    anchor_index = markdown.index(anchor)
    fence_start = markdown.rfind("\n```text", 0, anchor_index)
    if fence_start == -1:
        return replace_section(markdown, "usage", block, anchor)
    return f"{markdown[:fence_start].rstrip()}\n\n{block}{markdown[anchor_index:]}"


def replace_examples_section(markdown: str, block: str) -> str:
    """Replace generated examples or migrate the old static Examples section on first run."""
    if "<!-- regen-readme:start --section=examples -->" in markdown:
        return replace_section(markdown, "examples", block, "\n## Build it")

    examples_heading = "\n## Examples"
    build_marker = "\n<!-- regen-readme:start --section=build-examples -->"
    build_heading = "\n## Build it"
    start = markdown.find(examples_heading)
    end = markdown.find(build_marker)
    if end == -1:
        end = markdown.find(build_heading)
    if start != -1 and end != -1 and start < end:
        return f"{markdown[:start].rstrip()}\n\n{block}{markdown[end:]}"
    return replace_section(markdown, "examples", block, build_heading)


def remove_unmarked_duplicate_build_section(markdown: str) -> str:
    """Drop a one-time unmarked Build section left by earlier README migration."""
    build_marker = "\n<!-- regen-readme:start --section=build-examples -->"
    marker_index = markdown.find(build_marker)
    if marker_index == -1:
        return markdown

    build_heading = "\n## Build it"
    heading_index = markdown.rfind(build_heading, 0, marker_index)
    examples_end = markdown.rfind("<!-- regen-readme:end --section=examples -->", 0, marker_index)
    if heading_index == -1 or examples_end == -1 or examples_end > heading_index:
        return markdown
    return f"{markdown[:heading_index].rstrip()}\n\n{markdown[marker_index:].lstrip()}"


def replace_section(markdown: str, section: str, block: str, anchor: str) -> str:
    """Replace a generated README section or insert it before the provided anchor."""
    section_re = re.compile(
        rf"<!-- regen-readme:start --section={re.escape(section)} -->\n.*?"
        rf"<!-- regen-readme:end --section={re.escape(section)} -->\n*",
        re.S,
    )
    if section_re.search(markdown):
        return section_re.sub(block, markdown)

    if anchor in markdown:
        return markdown.replace(anchor, f"\n{block}{anchor.lstrip()}", 1)
    return f"{markdown.rstrip()}\n\n{block}"


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except Exception as exc:
        print(str(exc), file=sys.stderr)
        raise SystemExit(1)
