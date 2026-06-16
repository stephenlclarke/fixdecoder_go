// SPDX-License-Identifier: AGPL-3.0-only
// SPDX-FileCopyrightText: 2026 Steve Clarke <stephenlclarke@mac.com> - https://xyzzy.tools

package resources

import _ "embed"

// UsageText contains the user-facing command-line help for the Go CLI.
//
//go:embed messages/usage_en.txt
var UsageText string
