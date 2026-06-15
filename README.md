![repo logo](docs/repo-logo.png)
![repo title](docs/repo-title.png)

---

[![CI](https://github.com/stephenlclarke/fixdecoder_go/actions/workflows/ci.yml/badge.svg)](https://github.com/stephenlclarke/fixdecoder_go/actions/workflows/ci.yml)
[![Quality Gate Status](https://sonarcloud.io/api/project_badges/measure?project=stephenlclarke_fixdecoder_go&metric=alert_status)](https://sonarcloud.io/summary/new_code?id=stephenlclarke_fixdecoder_go)
[![Bugs](https://sonarcloud.io/api/project_badges/measure?project=stephenlclarke_fixdecoder_go&metric=bugs)](https://sonarcloud.io/summary/new_code?id=stephenlclarke_fixdecoder_go)
[![Code Smells](https://sonarcloud.io/api/project_badges/measure?project=stephenlclarke_fixdecoder_go&metric=code_smells)](https://sonarcloud.io/summary/new_code?id=stephenlclarke_fixdecoder_go)
[![Coverage](https://sonarcloud.io/api/project_badges/measure?project=stephenlclarke_fixdecoder_go&metric=coverage)](https://sonarcloud.io/summary/new_code?id=stephenlclarke_fixdecoder_go)
[![Duplicated Lines (%)](https://sonarcloud.io/api/project_badges/measure?project=stephenlclarke_fixdecoder_go&metric=duplicated_lines_density)](https://sonarcloud.io/summary/new_code?id=stephenlclarke_fixdecoder_go)
[![Lines of Code](https://sonarcloud.io/api/project_badges/measure?project=stephenlclarke_fixdecoder_go&metric=ncloc)](https://sonarcloud.io/summary/new_code?id=stephenlclarke_fixdecoder_go)
[![Reliability Rating](https://sonarcloud.io/api/project_badges/measure?project=stephenlclarke_fixdecoder_go&metric=reliability_rating)](https://sonarcloud.io/summary/new_code?id=stephenlclarke_fixdecoder_go)
[![Security Rating](https://sonarcloud.io/api/project_badges/measure?project=stephenlclarke_fixdecoder_go&metric=security_rating)](https://sonarcloud.io/summary/new_code?id=stephenlclarke_fixdecoder_go)
[![Technical Debt](https://sonarcloud.io/api/project_badges/measure?project=stephenlclarke_fixdecoder_go&metric=sqale_index)](https://sonarcloud.io/summary/new_code?id=stephenlclarke_fixdecoder_go)
[![Maintainability Rating](https://sonarcloud.io/api/project_badges/measure?project=stephenlclarke_fixdecoder_go&metric=sqale_rating)](https://sonarcloud.io/summary/new_code?id=stephenlclarke_fixdecoder_go)
[![Vulnerabilities](https://sonarcloud.io/api/project_badges/measure?project=stephenlclarke_fixdecoder_go&metric=vulnerabilities)](https://sonarcloud.io/summary/new_code?id=stephenlclarke_fixdecoder_go)
![Repo Visitors](https://visitor-badge.laobi.icu/badge?page_id=stephenlclarke.fixdecoder_go)

---

# Steve's FIX Decoder / logfile prettify utility

This is my Go implementation of an "all-singing / all-dancing" utility to pretty-print logfiles containing FIX Protocol messages while experimenting with a compact native Go command-line shape and trying to incorporate SonarQube Code Quality metrics.

I have written utilities like this in past in [Java](https://github.com/stephenlclarke/fixdecoder_java), Python, C, C++, [go](https://github.com/stephenlclarke/fixdecoder_go) and even in Bash/Awk!! Rust remains my favourite, but this Go version is the small native implementation that helped shape the later [Rust](https://github.com/stephenlclarke/fixdecoder_rs) and [Java](https://github.com/stephenlclarke/fixdecoder_java) versions.

![repo title](docs/example.png)

---

<p align="center">
  <a href="https://buy.stripe.com/8x23cvaHjaXzdg30Ni77O00">
    <img src="https://img.shields.io/badge/Buy%20Me%20a%20Coffee-❤️-brightgreen?style=for-the-badge&logo=buymeacoffee&logoColor=white" alt="Buy Me a Coffee">
  </a>
  &nbsp;
  <a href="https://github.com/stephenlclarke/fixdecoder_go/discussions">
    <img src="https://img.shields.io/badge/Leave%20a%20Comment-💬-blue?style=for-the-badge" alt="Leave a Comment">
  </a>
</p>

<p align="center">
  <sub>☕ If you found this project useful, consider buying me a coffee or dropping a comment — it keeps the caffeine and ideas flowing! 😄</sub>
</p>

---

## What is it

fixdecoder is a FIX-aware logfile prettifier and dictionary explorer. It reads stdin or one or more log files, detects FIX messages in each line, prints the original line, and follows it with a colourised tag breakdown using embedded FIX dictionaries or a supplied QuickFIX XML dictionary. For lookup work, `-info`, `-message`, `-component`, and `-tag` inspect the selected FIX version without decoding a log stream.

This Go implementation is intentionally simpler than the [Rust](https://github.com/stephenlclarke/fixdecoder_rs) and [Java](https://github.com/stephenlclarke/fixdecoder_java) repos. It focuses on fast native builds, embedded dictionary lookup, command-line dictionary browsing, and straightforward logfile prettification.

## Quick start

```bash
make build

# Stream and prettify stdin
cat fixlog.txt | ./bin/fixdecoder

# Decode one or more files
./bin/fixdecoder logs/fix.log logs/fix2.log

# Browse dictionary definitions
./bin/fixdecoder -fix=44 -message=D -verbose -column -header -trailer
```

## Running the fixdecoder utility

You can run fixdecoder anywhere you can run a Go binary. The standard build embeds FIX dictionaries for FIX 4.0 through FIX 5.0 SP2 plus FIXT 1.1, and `-xml` can point at an alternative QuickFIX XML file for custom dictionaries.

```text
fixdecoder [[-fix=44] | [-xml FIX44.xml]] [-message[=MSG] [-verbose] [-column] [-header] [-trailer]]
fixdecoder [[-fix=44] | [-xml FIX44.xml]] [-tag[=TAG] [-verbose] [-column]]
fixdecoder [[-fix=44] | [-xml FIX44.xml]] [-component=[NAME] [-verbose]]
fixdecoder [[-fix=44] | [-xml FIX44.xml]] [-info]
fixdecoder [file1.log file2.log ...]
```

## Key options at a glance

- Dictionaries: `-xml`, `-fix`, `-info`, `-message`, `-component`, `-tag`
- Output/layout: `-column`, `-verbose`, `-header`, `-trailer`
- Input: one or more log files, or stdin when no file is supplied

## Examples

### Prettify stdin

The FIX SOH delimiter is shown as `<SOH>` below for readability.

```bash
$ cat fixlog.txt | ./bin/fixdecoder
Processing: (stdin)

8=FIX.4.4<SOH>9=22<SOH>35=0<SOH>49=BUY1<SOH>56=SELL1<SOH>10=168<SOH>
    8 (BeginString): FIX.4.4
    9 (BodyLength): 22
    35 (MsgType): 0 (Heartbeat)
    49 (SenderCompID): BUY1
    56 (TargetCompID): SELL1
    10 (CheckSum): 168
```

### Inspect a message

```bash
$ ./bin/fixdecoder -fix=44 -message=D -header -trailer
Message: NewOrderSingle (D)
    Component: Header
           8: BeginString (STRING) - (Y)
           9: BodyLength (LENGTH) - (Y)
          35: MsgType (STRING) - (Y)
          49: SenderCompID (STRING) - (Y)
          56: TargetCompID (STRING) - (Y)
    Message: Body
          11: ClOrdID (STRING) - (Y)
         526: SecondaryClOrdID (STRING)
         583: ClOrdLinkID (STRING)
   Component: Parties
         453: NoPartyIDs (NUMINGROUP)
...
```

### Inspect a tag

```bash
$ ./bin/fixdecoder -fix=44 -tag=35 -verbose
  35: MsgType (STRING)
       0 : HEARTBEAT
       1 : TEST_REQUEST
       2 : RESEND_REQUEST
       3 : REJECT
       4 : SEQUENCE_RESET
       5 : LOGOUT
```

## Development

The local workflow uses Go and the repo's `ci.sh` wrapper.

```bash
make build
make unit-test
make integration-test
make scan
go test ./... -cover
```

The FIX dictionary Go source is regenerated from the XML resources with:

```bash
./resources/generate_fix_go.sh
```

## Related implementations

- [fixdecoder_rs](https://github.com/stephenlclarke/fixdecoder_rs) is the Rust version and remains my favourite.
- [fixdecoder_java](https://github.com/stephenlclarke/fixdecoder_java) is the Java version with object-oriented internals, Maven, JaCoCo coverage, and Java CI/CD.

## License

Project source is released under the GNU Affero General Public License v3.0 only (`AGPL-3.0-only`). Maintained source files carry SPDX headers:

```text
SPDX-License-Identifier: AGPL-3.0-only
SPDX-FileCopyrightText: 2026 Steve Clarke <stephenlclarke@mac.com> - https://xyzzy.tools
```

The embedded FIX dictionary packages are generated from QuickFIX FIX XML specifications. Those QuickFIX materials remain under the BSD 2-Clause “Simplified” License; see `NOTICE.md` for the retained attribution text and compatibility note.
