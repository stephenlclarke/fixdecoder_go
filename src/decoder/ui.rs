// SPDX-License-Identifier: AGPL-3.0-only
// SPDX-FileCopyrightText: 2026 Steve Clarke <stephenlclarke@mac.com> - https://xyzzy.tools

use crate::decoder::fixparser::parse_fix;
use crate::decoder::prettifier::{MsgTypeCount, PrettifyContext, prettify_files, prettify_with_report};
use crate::decoder::summary::OrderSummary;
use crate::decoder::tag_lookup;
use crate::decoder::validator;
use crate::fix;
use anyhow::{Context, Result, anyhow};
use bubbletea_rs::{Cmd, KeyMsg, Model, Msg, Program, WindowSizeMsg, quit, window_size};
use bubbletea_widgets::help;
use bubbletea_widgets::key::Binding;
use bubbletea_widgets::viewport;
use crossterm::event::{KeyCode, KeyModifiers};
use once_cell::sync::Lazy;
use regex::Regex;
use std::collections::{BTreeMap, HashMap};
use std::fs;
use std::io::{IsTerminal, Read};
use std::path::{Path, PathBuf};
use std::sync::Arc;
use std::sync::Mutex;
use std::time::{SystemTime, UNIX_EPOCH};

const RESET: &str = "\x1b[0m";
const BOLD: &str = "\x1b[1m";
const FG_TEXT: &str = "\x1b[38;2;230;233;239m";
const FG_DIM: &str = "\x1b[38;2;95;104;120m";
const FG_ACCENT: &str = "\x1b[38;2;139;233;253m";
const FG_RULE: &str = "\x1b[38;2;68;71;90m";
const FG_WARN: &str = "\x1b[38;2;255;184;108m";
const FG_GUTTER: &str = "\x1b[38;2;98;114;164m";
const BG_HEADER: &str = "\x1b[48;2;40;42;54m";
const BG_STATUS: &str = "\x1b[48;2;24;26;32m";
const BG_DIALOG: &str = "\x1b[48;2;33;35;43m";
const BG_DIALOG_SELECTED: &str = "\x1b[48;2;236;239;244m";
const FG_DIALOG_SELECTED: &str = "\x1b[38;2;20;23;28m";
const MATCH_START: &str = "\x1b[38;2;255;255;255m\x1b[48;2;255;149;0m";
const MATCH_END: &str = "\x1b[39m\x1b[49m";
const SOH_CHAR: char = '\u{0001}';
const SOH_VISIBLE_CHAR: char = '␁';
const SOH_START: &str = "\x1b[47m\x1b[30m";
const SOH_END: &str = "\x1b[39;49m";

static UI_FIX_REGEX: Lazy<Regex> =
    Lazy::new(|| Regex::new(r"8=FIX.*?10=\d{3}\u{0001}").expect("valid FIX extraction regex"));
static UI_FIX_LINE_REGEX: Lazy<Regex> =
    Lazy::new(|| Regex::new(r"8=FIX.*10=\d{3}").expect("valid FIX line regex"));

#[derive(Clone)]
struct UiBootstrap {
    raw_lines: Vec<String>,
    decoded_plain_lines: Vec<String>,
    decoded_secret_lines: Vec<String>,
    fix_messages: Vec<FixMessageRecord>,
    initial_secret_enabled: bool,
    source_label: String,
    warning_count: usize,
}

static UI_BOOTSTRAP: Lazy<Mutex<Option<UiBootstrap>>> = Lazy::new(|| Mutex::new(None));

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
enum ViewMode {
    Raw,
    Decoded,
}

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
enum SearchDirection {
    Forward,
    Backward,
}

impl SearchDirection {
    fn symbol(self) -> char {
        match self {
            Self::Forward => '/',
            Self::Backward => '?',
        }
    }

    fn opposite(self) -> Self {
        match self {
            Self::Forward => Self::Backward,
            Self::Backward => Self::Forward,
        }
    }
}

#[derive(Clone, Debug)]
struct SearchPrompt {
    direction: SearchDirection,
    query: String,
}

#[derive(Clone)]
struct FixMessageRecord {
    raw_message: String,
    decoded_plain_lines: Vec<String>,
    decoded_secret_lines: Vec<String>,
    fields: Vec<crate::decoder::fixparser::FieldValue>,
    msg_type: Option<String>,
    dict: Arc<tag_lookup::FixTagLookup>,
}

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
enum FilterOperator {
    Present,
    Equals,
    Regex,
}

impl FilterOperator {
    fn label(self) -> &'static str {
        match self {
            Self::Present => "present",
            Self::Equals => "equals",
            Self::Regex => "regex",
        }
    }
}

#[derive(Clone)]
enum FilterPredicate {
    Present,
    Equals(String),
    Regex { pattern: String, regex: Regex },
}

#[derive(Clone)]
enum FilterClause {
    MsgTypes(Vec<String>),
    TagPredicate {
        msg_types: Vec<String>,
        tag: u32,
        tag_name: String,
        predicate: FilterPredicate,
    },
}

impl FilterClause {
    fn scope_label(msg_types: &[String]) -> String {
        if msg_types.is_empty() {
            return "any msg".to_string();
        }
        if msg_types.len() == 1 {
            return format!("35={}", msg_types[0]);
        }
        format!("35 in [{}]", msg_types.join(","))
    }

    fn msg_matches_scope(message: &FixMessageRecord, msg_types: &[String]) -> bool {
        if msg_types.is_empty() {
            return true;
        }
        let Some(current) = message.msg_type.as_deref() else {
            return false;
        };
        msg_types.iter().any(|entry| entry == current)
    }

    fn summary(&self) -> String {
        match self {
            Self::MsgTypes(msg_types) => {
                format!("{}: message type filter", Self::scope_label(msg_types))
            }
            Self::TagPredicate {
                msg_types,
                tag,
                tag_name,
                predicate,
            } => {
                let predicate_text = match predicate {
                    FilterPredicate::Present => "present".to_string(),
                    FilterPredicate::Equals(value) => format!("== {value}"),
                    FilterPredicate::Regex { pattern, .. } => format!("~ /{pattern}/"),
                };
                format!(
                    "{}: {} ({}) {}",
                    Self::scope_label(msg_types),
                    tag,
                    tag_name,
                    predicate_text
                )
            }
        }
    }

    fn matches(&self, message: &FixMessageRecord) -> bool {
        match self {
            Self::MsgTypes(msg_types) => Self::msg_matches_scope(message, msg_types),
            Self::TagPredicate {
                msg_types,
                tag,
                predicate,
                ..
            } => {
                if !Self::msg_matches_scope(message, msg_types) {
                    return false;
                }
                let values: Vec<&str> = message
                    .fields
                    .iter()
                    .filter(|field| field.tag == *tag)
                    .map(|field| field.value.as_str())
                    .collect();

                match predicate {
                    FilterPredicate::Present => !values.is_empty(),
                    FilterPredicate::Equals(expected) => {
                        values.iter().any(|value| *value == expected)
                    }
                    FilterPredicate::Regex { regex, .. } => {
                        values.iter().any(|value| regex.is_match(value))
                    }
                }
            }
        }
    }
}

#[derive(Clone)]
struct TagOption {
    tag: u32,
    name: String,
}

impl TagOption {
    fn label(&self) -> String {
        format!("{} ({})", self.tag, self.name)
    }
}

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
enum FilterDialogStage {
    Main,
    SelectMsgType,
    SelectTag,
    SelectOperator,
    InputValue,
}

#[derive(Clone, Default)]
struct PendingFilterClause {
    msg_types: Vec<String>,
    tag: Option<u32>,
    tag_name: Option<String>,
    operator: Option<FilterOperator>,
    value: String,
}

#[derive(Clone)]
struct FilterDialogState {
    stage: FilterDialogStage,
    cursor: usize,
    selected_clause: Option<usize>,
    pending: PendingFilterClause,
}

impl Default for FilterDialogState {
    fn default() -> Self {
        Self {
            stage: FilterDialogStage::Main,
            cursor: 0,
            selected_clause: None,
            pending: PendingFilterClause::default(),
        }
    }
}

#[derive(Clone)]
struct UiBindings {
    quit: Binding,
    toggle_help: Binding,
    toggle_secret: Binding,
    toggle_mode: Binding,
    toggle_wrap: Binding,
    search_forward: Binding,
    search_backward: Binding,
    search_next: Binding,
    search_prev: Binding,
    open_filter: Binding,
    scroll_left: Binding,
    scroll_right: Binding,
}

impl Default for UiBindings {
    fn default() -> Self {
        Self {
            quit: Binding::new(vec!["q", "esc", "ctrl+c"]).with_help("q/esc", "quit"),
            toggle_help: Binding::new(vec![KeyCode::Char('h')]).with_help("h", "toggle help"),
            toggle_secret: Binding::new(vec!["s"]).with_help("s", "toggle secrets"),
            toggle_mode: Binding::new(vec!["r"]).with_help("r", "toggle raw/decoded"),
            toggle_wrap: Binding::new(vec!["w"]).with_help("w", "toggle wrap"),
            search_forward: Binding::new(vec!["/"]).with_help("/", "search fwd"),
            search_backward: Binding::new(vec![KeyCode::Char('?')]).with_help("?", "search back"),
            search_next: Binding::new(vec!["n"]).with_help("n", "next match"),
            search_prev: Binding::new(vec!["p"]).with_help("p", "prev match"),
            open_filter: Binding::new(vec!["f"]).with_help("f", "filter dialog"),
            scroll_left: Binding::new(vec![KeyCode::Left]).with_help("\u{2190}", "scroll left"),
            scroll_right: Binding::new(vec![KeyCode::Right]).with_help("\u{2192}", "scroll right"),
        }
    }
}

struct FixUiModel {
    viewport: viewport::Model,
    help: help::Model,
    bindings: UiBindings,
    raw_lines: Vec<String>,
    decoded_plain_lines: Vec<String>,
    decoded_secret_lines: Vec<String>,
    fix_messages: Vec<FixMessageRecord>,
    filtered_raw_lines: Vec<String>,
    filtered_plain_lines: Vec<String>,
    filtered_secret_lines: Vec<String>,
    filtered_message_count: usize,
    filter_clauses: Vec<FilterClause>,
    filter_dialog: Option<FilterDialogState>,
    fix_spec_conformance: bool,
    display_lines: Vec<String>,
    display_line_numbers: Vec<Option<usize>>,
    secret_enabled: bool,
    view_mode: ViewMode,
    wrap_enabled: bool,
    source_label: String,
    warning_count: usize,
    term_width: usize,
    term_height: usize,
    search_prompt: Option<SearchPrompt>,
    search_regex: Option<Regex>,
    search_pattern: Option<String>,
    search_direction: SearchDirection,
    status_message: Option<String>,
}

impl FixUiModel {
    fn from_bootstrap(bootstrap: UiBootstrap) -> Self {
        let mut viewport = viewport::new(80, 20);
        viewport.keymap.up = Binding::new(vec![KeyCode::Up]).with_help("\u{2191}", "up");
        viewport.keymap.down = Binding::new(vec![KeyCode::Down]).with_help("\u{2193}", "down");
        viewport.keymap.page_up = Binding::new(vec![KeyCode::PageUp]).with_help("pgup", "page up");
        viewport.keymap.page_down =
            Binding::new(vec![KeyCode::PageDown]).with_help("pgdn", "page down");
        viewport.keymap.half_page_up = Binding::new(Vec::<KeyCode>::new());
        viewport.keymap.half_page_down = Binding::new(Vec::<KeyCode>::new());
        viewport.keymap.left = Binding::new(Vec::<KeyCode>::new());
        viewport.keymap.right = Binding::new(Vec::<KeyCode>::new());

        let mut model = Self {
            viewport,
            help: help::Model::new(),
            bindings: UiBindings::default(),
            raw_lines: bootstrap.raw_lines,
            decoded_plain_lines: bootstrap.decoded_plain_lines,
            decoded_secret_lines: bootstrap.decoded_secret_lines,
            fix_messages: bootstrap.fix_messages,
            filtered_raw_lines: Vec::new(),
            filtered_plain_lines: Vec::new(),
            filtered_secret_lines: Vec::new(),
            filtered_message_count: 0,
            filter_clauses: Vec::new(),
            filter_dialog: None,
            fix_spec_conformance: true,
            display_lines: Vec::new(),
            display_line_numbers: Vec::new(),
            secret_enabled: bootstrap.initial_secret_enabled,
            view_mode: ViewMode::Decoded,
            wrap_enabled: false,
            source_label: bootstrap.source_label,
            warning_count: bootstrap.warning_count,
            term_width: 100,
            term_height: 30,
            search_prompt: None,
            search_regex: None,
            search_pattern: None,
            search_direction: SearchDirection::Forward,
            status_message: None,
        };
        model.recompute_filtered_lines();
        model.refresh_layout();
        model
    }

    fn filters_active(&self) -> bool {
        !self.filter_clauses.is_empty()
    }

    fn active_decoded_lines(&self) -> &[String] {
        if self.filters_active() {
            if self.secret_enabled {
                &self.filtered_secret_lines
            } else {
                &self.filtered_plain_lines
            }
        } else if self.secret_enabled {
            &self.decoded_secret_lines
        } else {
            &self.decoded_plain_lines
        }
    }

    fn active_lines(&self) -> &[String] {
        match self.view_mode {
            ViewMode::Raw => {
                if self.filters_active() {
                    &self.filtered_raw_lines
                } else {
                    &self.raw_lines
                }
            }
            ViewMode::Decoded => self.active_decoded_lines(),
        }
    }

    fn set_view_mode(&mut self, mode: ViewMode) {
        if self.view_mode == mode {
            return;
        }
        self.view_mode = mode;
        self.viewport.x_offset = 0;
        self.sync_layout_content(false);
    }

    fn toggle_view_mode(&mut self) {
        let next = match self.view_mode {
            ViewMode::Raw => ViewMode::Decoded,
            ViewMode::Decoded => ViewMode::Raw,
        };
        self.set_view_mode(next);
    }

    fn toggle_secret_mode(&mut self) {
        self.secret_enabled = !self.secret_enabled;
        if self.view_mode == ViewMode::Decoded {
            self.viewport.x_offset = 0;
            self.sync_layout_content(true);
            return;
        }
        self.sync_layout_content(false);
    }

    fn refresh_layout(&mut self) {
        self.sync_layout_content(false);
    }

    fn compute_viewport_dimensions(&self, line_count: usize) -> (usize, usize) {
        let help_rows = if self.help.show_all { 12 } else { 6 };
        let reserved_rows = help_rows + self.filter_dialog_height();
        let line_number_width = line_count.max(1).to_string().len().max(3) + 3;
        let width = self.term_width.saturating_sub(line_number_width).max(20);
        let height = self.term_height.saturating_sub(reserved_rows).max(3);
        (width, height)
    }

    fn display_lines_for_width(&self, width: usize) -> Vec<String> {
        self.build_display_lines_and_numbers(width).0
    }

    fn is_fix_message_line(line: &str) -> bool {
        let plain = strip_ansi(line);
        UI_FIX_LINE_REGEX.is_match(&plain)
    }

    fn build_display_lines_and_numbers(&self, width: usize) -> (Vec<String>, Vec<Option<usize>>) {
        let mut lines = Vec::<String>::new();
        let mut numbers = Vec::<Option<usize>>::new();
        let mut fix_counter = 0usize;

        for line in self.active_lines() {
            let is_fix = Self::is_fix_message_line(line);
            let display_line = viewport_display_line(line);
            if self.wrap_enabled {
                let segments = wrap_single_line(&display_line, width);
                if segments.is_empty() {
                    lines.push(String::new());
                    numbers.push(None);
                    continue;
                }
                for (seg_idx, segment) in segments.into_iter().enumerate() {
                    lines.push(segment);
                    if is_fix && seg_idx == 0 {
                        fix_counter += 1;
                        numbers.push(Some(fix_counter));
                    } else {
                        numbers.push(None);
                    }
                }
            } else {
                lines.push(display_line);
                if is_fix {
                    fix_counter += 1;
                    numbers.push(Some(fix_counter));
                } else {
                    numbers.push(None);
                }
            }
        }

        if lines.is_empty() {
            lines.push(String::new());
            numbers.push(None);
        }

        (lines, numbers)
    }

    fn sync_layout_content(&mut self, reset_vertical: bool) {
        let mut line_count = self.active_lines().len().max(1);
        let mut viewport_width = 20;
        let mut viewport_height = 3;
        let mut display_lines = self.active_lines().to_vec();
        let mut display_numbers = vec![None];

        for _ in 0..3 {
            let (width, height) = self.compute_viewport_dimensions(line_count);
            viewport_width = width;
            viewport_height = height;
            let (lines, numbers) = self.build_display_lines_and_numbers(width);
            display_lines = lines;
            display_numbers = numbers;
            let next_count = display_lines.len().max(1);
            if next_count == line_count {
                break;
            }
            line_count = next_count;
        }

        self.viewport.width = viewport_width;
        self.viewport.height = viewport_height;
        self.help.width = self.term_width.saturating_sub(2);
        self.viewport.set_content_lines(display_lines.clone());
        self.display_lines = display_lines;
        self.display_line_numbers = display_numbers;
        if self.wrap_enabled {
            self.viewport.x_offset = 0;
        }
        if reset_vertical {
            self.viewport.goto_top();
        }
    }

    fn toggle_wrap_mode(&mut self) {
        self.wrap_enabled = !self.wrap_enabled;
        if self.wrap_enabled {
            self.viewport.x_offset = 0;
        }
        self.sync_layout_content(false);
    }

    fn recompute_filtered_lines(&mut self) {
        self.filtered_raw_lines.clear();
        self.filtered_plain_lines.clear();
        self.filtered_secret_lines.clear();
        self.filtered_message_count = 0;

        if self.filter_clauses.is_empty() {
            return;
        }

        let mut selected: Vec<&FixMessageRecord> = self
            .fix_messages
            .iter()
            .filter(|msg| self.filter_clauses.iter().all(|clause| clause.matches(msg)))
            .collect();

        self.filtered_message_count = selected.len();
        if selected.is_empty() {
            let empty = "No FIX messages matched active filters.".to_string();
            self.filtered_raw_lines.push(empty.clone());
            self.filtered_plain_lines.push(empty.clone());
            self.filtered_secret_lines.push(empty);
            return;
        }

        for msg in selected.drain(..) {
            self.filtered_raw_lines.push(msg.raw_message.clone());
            self.filtered_plain_lines
                .extend(msg.decoded_plain_lines.iter().cloned());
            self.filtered_secret_lines
                .extend(msg.decoded_secret_lines.iter().cloned());
        }
    }

    fn open_filter_dialog(&mut self) {
        self.filter_dialog = Some(FilterDialogState::default());
        self.status_message = Some(
            "filter: configure clauses (space toggles FIX Spec conformance checkbox)".to_string(),
        );
        self.refresh_layout();
    }

    fn close_filter_dialog(&mut self) {
        self.filter_dialog = None;
        self.refresh_layout();
    }

    fn set_fix_spec_conformance(&mut self, enabled: bool) {
        if self.fix_spec_conformance == enabled {
            return;
        }
        self.fix_spec_conformance = enabled;
        let mode = if enabled { "on" } else { "off" };
        self.status_message = Some(format!("filter: FIX Spec conformance {mode}"));
    }

    fn toggle_fix_spec_conformance(&mut self) {
        let enabled = !self.fix_spec_conformance;
        self.set_fix_spec_conformance(enabled);
    }

    fn filter_dialog_max_rows(&self) -> usize {
        self.term_height.saturating_sub(14).clamp(6, 16)
    }

    fn filter_dialog_height(&self) -> usize {
        let Some(dialog) = self.filter_dialog.as_ref() else {
            return 0;
        };
        let rows = match dialog.stage {
            FilterDialogStage::Main => 8 + self.filter_clauses.len().min(5),
            FilterDialogStage::SelectMsgType => 6 + self.available_msg_types().len().min(6),
            FilterDialogStage::SelectTag => {
                6 + self
                    .available_tags_for_msg_types(&dialog.pending.msg_types)
                    .len()
                    .min(6)
            }
            FilterDialogStage::SelectOperator => 9,
            FilterDialogStage::InputValue => 8,
        };
        rows.clamp(7, self.filter_dialog_max_rows())
    }

    fn available_msg_types(&self) -> Vec<String> {
        let mut msg_types: Vec<String> = self
            .fix_messages
            .iter()
            .filter_map(|msg| msg.msg_type.clone())
            .collect();
        msg_types.sort();
        msg_types.dedup();
        msg_types
    }

    fn message_in_selected_types(message: &FixMessageRecord, msg_types: &[String]) -> bool {
        if msg_types.is_empty() {
            return true;
        }
        let Some(mt) = message.msg_type.as_deref() else {
            return false;
        };
        msg_types.iter().any(|entry| entry == mt)
    }

    fn collect_observed_tags(
        &self,
        msg_types: &[String],
        tags: &mut BTreeMap<u32, String>,
    ) {
        for msg in &self.fix_messages {
            if !Self::message_in_selected_types(msg, msg_types) {
                continue;
            }
            for field in &msg.fields {
                tags.entry(field.tag)
                    .or_insert_with(|| msg.dict.field_name(field.tag));
            }
        }
    }

    fn conformance_tags_for_single_type(&self, msg_type: &str) -> BTreeMap<u32, String> {
        let mut tags = BTreeMap::<u32, String>::new();
        for msg in &self.fix_messages {
            if msg.msg_type.as_deref() != Some(msg_type) {
                continue;
            }
            if let Some(def) = msg.dict.message_def(msg_type) {
                for tag in &def.field_order {
                    tags.entry(*tag).or_insert_with(|| msg.dict.field_name(*tag));
                }
            }
        }
        tags
    }

    fn available_tags_for_msg_types(&self, msg_types: &[String]) -> Vec<TagOption> {
        let mut tags = BTreeMap::<u32, String>::new();

        if self.fix_spec_conformance {
            if msg_types.is_empty() {
                for msg_type in self.available_msg_types() {
                    let msg_tags = self.conformance_tags_for_single_type(&msg_type);
                    for (tag, name) in msg_tags {
                        tags.entry(tag).or_insert(name);
                    }
                }
            } else {
                let mut iter = msg_types.iter();
                if let Some(first) = iter.next() {
                    tags = self.conformance_tags_for_single_type(first);
                    for msg_type in iter {
                        let rhs = self.conformance_tags_for_single_type(msg_type);
                        tags.retain(|tag, _| rhs.contains_key(tag));
                    }
                }
            }
        } else {
            self.collect_observed_tags(msg_types, &mut tags);
            if tags.is_empty() {
                self.collect_observed_tags(&[], &mut tags);
            }
        }

        tags.into_iter()
            .map(|(tag, name)| TagOption { tag, name })
            .collect()
    }

    fn create_clause_from_pending(&mut self, pending: &PendingFilterClause) -> bool {
        if pending.tag.is_none() {
            if pending.msg_types.is_empty() {
                self.status_message =
                    Some("filter: choose at least one MsgType or select a tag".to_string());
                return false;
            }
            self.filter_clauses
                .push(FilterClause::MsgTypes(pending.msg_types.clone()));
            self.recompute_filtered_lines();
            self.sync_layout_content(true);
            self.status_message = Some(format!(
                "filter: {} clause(s) active",
                self.filter_clauses.len()
            ));
            return true;
        }

        let (Some(tag), Some(tag_name), Some(operator)) =
            (pending.tag, pending.tag_name.clone(), pending.operator)
        else {
            self.status_message = Some("filter: incomplete clause".to_string());
            return false;
        };

        let predicate = match operator {
            FilterOperator::Present => FilterPredicate::Present,
            FilterOperator::Equals => {
                if pending.value.is_empty() {
                    self.status_message = Some("filter: value is required for equals".to_string());
                    return false;
                }
                FilterPredicate::Equals(pending.value.clone())
            }
            FilterOperator::Regex => {
                if pending.value.is_empty() {
                    self.status_message = Some("filter: regex is required".to_string());
                    return false;
                }
                let regex = match Regex::new(&pending.value) {
                    Ok(compiled) => compiled,
                    Err(err) => {
                        self.status_message = Some(format!("filter: invalid regex ({err})"));
                        return false;
                    }
                };
                FilterPredicate::Regex {
                    pattern: pending.value.clone(),
                    regex,
                }
            }
        };

        let clause = FilterClause::TagPredicate {
            msg_types: pending.msg_types.clone(),
            tag,
            tag_name,
            predicate,
        };
        self.filter_clauses.push(clause);
        self.recompute_filtered_lines();
        self.sync_layout_content(true);
        self.status_message = Some(format!("filter: {} clause(s) active", self.filter_clauses.len()));
        true
    }

    fn clear_filter_clauses(&mut self) {
        self.filter_clauses.clear();
        self.recompute_filtered_lines();
        self.sync_layout_content(true);
        self.status_message = Some("filter: cleared".to_string());
    }

    fn remove_filter_clause(&mut self, idx: usize) {
        if idx >= self.filter_clauses.len() {
            self.status_message = Some("filter: no clause selected".to_string());
            return;
        }
        self.filter_clauses.remove(idx);
        self.recompute_filtered_lines();
        self.sync_layout_content(true);
        self.status_message = Some(format!("filter: {} clause(s) active", self.filter_clauses.len()));
    }

    fn main_dialog_row_count(&self) -> usize {
        5 + self.filter_clauses.len()
    }

    fn handle_filter_dialog_main(&mut self, key_msg: &KeyMsg) {
        enum MainAction {
            None,
            StartAdd,
            ToggleConformance,
            Clear,
            DeleteSelected(Option<usize>),
            Close,
            SelectClause(usize),
        }

        let row_count = self.main_dialog_row_count();
        let mut action = MainAction::None;

        if let Some(dialog) = self.filter_dialog.as_mut() {
            match key_msg.key {
                KeyCode::Up | KeyCode::Char('k') => {
                    dialog.cursor = dialog.cursor.saturating_sub(1);
                }
                KeyCode::Down | KeyCode::Char('j') => {
                    if dialog.cursor + 1 < row_count {
                        dialog.cursor += 1;
                    }
                }
                KeyCode::Char('a') => action = MainAction::StartAdd,
                KeyCode::Char('x') => action = MainAction::Clear,
                KeyCode::Char('d') => action = MainAction::DeleteSelected(dialog.selected_clause),
                KeyCode::Char('c') => action = MainAction::ToggleConformance,
                KeyCode::Char(' ') | KeyCode::Enter => {
                    let clause_start = 1;
                    let clause_end = clause_start + self.filter_clauses.len();
                    let add_row = clause_end;
                    let delete_row = clause_end + 1;
                    let clear_row = clause_end + 2;
                    let close_row = clause_end + 3;
                    action = match dialog.cursor {
                        0 => MainAction::ToggleConformance,
                        row if row >= clause_start && row < clause_end => {
                            MainAction::SelectClause(row - clause_start)
                        }
                        row if row == add_row => MainAction::StartAdd,
                        row if row == delete_row => MainAction::DeleteSelected(dialog.selected_clause),
                        row if row == clear_row => MainAction::Clear,
                        row if row == close_row => MainAction::Close,
                        _ => MainAction::None,
                    };
                }
                _ => {}
            }

            if dialog.cursor > 0 && dialog.cursor <= self.filter_clauses.len() {
                dialog.selected_clause = Some(dialog.cursor - 1);
            } else if !matches!(action, MainAction::SelectClause(_)) {
                dialog.selected_clause = None;
            }
        }

        match action {
            MainAction::None => {}
            MainAction::StartAdd => {
                if let Some(dialog) = self.filter_dialog.as_mut() {
                    dialog.stage = FilterDialogStage::SelectMsgType;
                    dialog.cursor = 0;
                    dialog.pending = PendingFilterClause::default();
                }
            }
            MainAction::ToggleConformance => self.toggle_fix_spec_conformance(),
            MainAction::Clear => {
                self.clear_filter_clauses();
                let cap = self.main_dialog_row_count().saturating_sub(1);
                if let Some(dialog) = self.filter_dialog.as_mut() {
                    dialog.cursor = dialog.cursor.min(cap);
                    dialog.selected_clause = None;
                }
            }
            MainAction::DeleteSelected(selected) => {
                if let Some(idx) = selected {
                    self.remove_filter_clause(idx);
                    let cap = self.main_dialog_row_count().saturating_sub(1);
                    if let Some(dialog) = self.filter_dialog.as_mut() {
                        dialog.cursor = dialog.cursor.min(cap);
                        dialog.selected_clause = None;
                    }
                } else {
                    self.status_message = Some("filter: select a clause row first".to_string());
                }
            }
            MainAction::Close => self.close_filter_dialog(),
            MainAction::SelectClause(idx) => {
                if let Some(dialog) = self.filter_dialog.as_mut() {
                    dialog.selected_clause = Some(idx);
                }
            }
        }
    }

    fn handle_filter_dialog_select_msg_type(&mut self, key_msg: &KeyMsg) {
        let msg_types = self.available_msg_types();
        let row_count = msg_types.len() + 1;
        if let Some(dialog) = self.filter_dialog.as_mut() {
            match key_msg.key {
                KeyCode::Up | KeyCode::Char('k') => {
                    dialog.cursor = dialog.cursor.saturating_sub(1);
                }
                KeyCode::Down | KeyCode::Char('j') => {
                    if dialog.cursor + 1 < row_count {
                        dialog.cursor += 1;
                    }
                }
                KeyCode::Char(' ') => {
                    if dialog.cursor == 0 {
                        dialog.pending.msg_types.clear();
                    } else if let Some(choice) = msg_types.get(dialog.cursor - 1) {
                        if let Some(pos) = dialog.pending.msg_types.iter().position(|v| v == choice) {
                            dialog.pending.msg_types.remove(pos);
                        } else {
                            dialog.pending.msg_types.push(choice.clone());
                            dialog.pending.msg_types.sort();
                            dialog.pending.msg_types.dedup();
                        }
                    }
                }
                KeyCode::Enter => {
                    if dialog.cursor > 0
                        && let Some(choice) = msg_types.get(dialog.cursor - 1)
                        && !dialog.pending.msg_types.iter().any(|v| v == choice)
                    {
                        dialog.pending.msg_types.push(choice.clone());
                        dialog.pending.msg_types.sort();
                        dialog.pending.msg_types.dedup();
                    }
                    if dialog.pending.msg_types.is_empty() {
                        dialog.stage = FilterDialogStage::SelectTag;
                        dialog.cursor = 0;
                        return;
                    }
                    dialog.pending.tag = None;
                    dialog.pending.tag_name = None;
                    dialog.pending.operator = None;
                    let pending = dialog.pending.clone();
                    dialog.stage = FilterDialogStage::Main;
                    dialog.cursor = 0;
                    let _ = self.create_clause_from_pending(&pending);
                }
                KeyCode::Char('t') => {
                    if dialog.cursor > 0
                        && let Some(choice) = msg_types.get(dialog.cursor - 1)
                        && !dialog.pending.msg_types.iter().any(|v| v == choice)
                    {
                        dialog.pending.msg_types.push(choice.clone());
                        dialog.pending.msg_types.sort();
                        dialog.pending.msg_types.dedup();
                    }
                    dialog.stage = FilterDialogStage::SelectTag;
                    dialog.cursor = 0;
                }
                KeyCode::Esc => {
                    dialog.stage = FilterDialogStage::Main;
                    dialog.cursor = 0;
                }
                _ => {}
            }
        }
    }

    fn handle_filter_dialog_select_tag(&mut self, key_msg: &KeyMsg) {
        let options = self
            .filter_dialog
            .as_ref()
            .map(|dialog| self.available_tags_for_msg_types(&dialog.pending.msg_types));
        let options = options.unwrap_or_default();
        let row_count = options.len().max(1);
        if let Some(dialog) = self.filter_dialog.as_mut() {
            match key_msg.key {
                KeyCode::Up | KeyCode::Char('k') => {
                    dialog.cursor = dialog.cursor.saturating_sub(1);
                }
                KeyCode::Down | KeyCode::Char('j') => {
                    if dialog.cursor + 1 < row_count {
                        dialog.cursor += 1;
                    }
                }
                KeyCode::Enter => {
                    if let Some(choice) = options.get(dialog.cursor) {
                        dialog.pending.tag = Some(choice.tag);
                        dialog.pending.tag_name = Some(choice.name.clone());
                        dialog.pending.operator = None;
                        dialog.pending.value.clear();
                        dialog.stage = FilterDialogStage::SelectOperator;
                        dialog.cursor = 0;
                    } else {
                        self.status_message =
                            Some("filter: no selectable tags for current scope".to_string());
                        dialog.stage = FilterDialogStage::Main;
                        dialog.cursor = 0;
                    }
                }
                KeyCode::Esc => {
                    dialog.stage = FilterDialogStage::SelectMsgType;
                    dialog.cursor = 0;
                }
                _ => {}
            }
        }
    }

    fn handle_filter_dialog_select_operator(&mut self, key_msg: &KeyMsg) {
        let operators = [
            FilterOperator::Present,
            FilterOperator::Equals,
            FilterOperator::Regex,
        ];
        let mut pending_for_create: Option<PendingFilterClause> = None;

        if let Some(dialog) = self.filter_dialog.as_mut() {
            match key_msg.key {
                KeyCode::Up | KeyCode::Char('k') => {
                    dialog.cursor = dialog.cursor.saturating_sub(1);
                }
                KeyCode::Down | KeyCode::Char('j') => {
                    if dialog.cursor + 1 < operators.len() {
                        dialog.cursor += 1;
                    }
                }
                KeyCode::Enter => {
                    let choice = operators[dialog.cursor];
                    dialog.pending.operator = Some(choice);
                    match choice {
                        FilterOperator::Present => {
                            pending_for_create = Some(dialog.pending.clone());
                            dialog.stage = FilterDialogStage::Main;
                            dialog.cursor = 0;
                        }
                        FilterOperator::Equals | FilterOperator::Regex => {
                            dialog.pending.value.clear();
                            dialog.stage = FilterDialogStage::InputValue;
                        }
                    }
                }
                KeyCode::Esc => {
                    dialog.stage = FilterDialogStage::SelectTag;
                    dialog.cursor = 0;
                }
                _ => {}
            }
        }

        if let Some(pending) = pending_for_create
            && self.create_clause_from_pending(&pending)
            && let Some(open) = self.filter_dialog.as_mut()
        {
            open.selected_clause = Some(self.filter_clauses.len().saturating_sub(1));
        }
    }

    fn handle_filter_dialog_input_value(&mut self, key_msg: &KeyMsg) {
        let mut pending_for_create: Option<PendingFilterClause> = None;
        if let Some(dialog) = self.filter_dialog.as_mut() {
            match key_msg.key {
                KeyCode::Esc => {
                    dialog.stage = FilterDialogStage::SelectOperator;
                    dialog.cursor = 0;
                }
                KeyCode::Enter => {
                    pending_for_create = Some(dialog.pending.clone());
                    dialog.stage = FilterDialogStage::Main;
                    dialog.cursor = 0;
                }
                KeyCode::Backspace => {
                    dialog.pending.value.pop();
                }
                KeyCode::Char(ch)
                    if !key_msg.modifiers.contains(KeyModifiers::CONTROL)
                        && !key_msg.modifiers.contains(KeyModifiers::ALT) =>
                {
                    dialog.pending.value.push(ch);
                }
                _ => {}
            }
        }

        if let Some(pending) = pending_for_create
            && self.create_clause_from_pending(&pending)
            && let Some(open) = self.filter_dialog.as_mut()
        {
            open.selected_clause = Some(self.filter_clauses.len().saturating_sub(1));
        }
    }

    fn handle_filter_input(&mut self, key_msg: &KeyMsg) -> Option<Cmd> {
        if key_msg.key == KeyCode::Char('c') && key_msg.modifiers.contains(KeyModifiers::CONTROL) {
            return Some(quit());
        }

        if matches!(key_msg.key, KeyCode::Esc)
            && self
                .filter_dialog
                .as_ref()
                .is_some_and(|dialog| dialog.stage == FilterDialogStage::Main)
        {
            self.close_filter_dialog();
            return None;
        }

        let stage = self
            .filter_dialog
            .as_ref()
            .map(|dialog| dialog.stage)
            .unwrap_or(FilterDialogStage::Main);

        match stage {
            FilterDialogStage::Main => self.handle_filter_dialog_main(key_msg),
            FilterDialogStage::SelectMsgType => self.handle_filter_dialog_select_msg_type(key_msg),
            FilterDialogStage::SelectTag => self.handle_filter_dialog_select_tag(key_msg),
            FilterDialogStage::SelectOperator => self.handle_filter_dialog_select_operator(key_msg),
            FilterDialogStage::InputValue => self.handle_filter_dialog_input_value(key_msg),
        }
        None
    }

    fn begin_search(&mut self, direction: SearchDirection) {
        self.status_message = None;
        self.search_prompt = Some(SearchPrompt {
            direction,
            query: String::new(),
        });
    }

    fn commit_search_prompt(&mut self) {
        let Some(prompt) = self.search_prompt.take() else {
            return;
        };
        let query = if prompt.query.is_empty() {
            self.search_pattern.clone().unwrap_or_default()
        } else {
            prompt.query
        };

        if query.is_empty() {
            self.status_message = Some("search: empty pattern".to_string());
            return;
        }
        self.search_for_pattern(&query, prompt.direction);
    }

    fn repeat_search(&mut self, reverse: bool) {
        let (Some(pattern), Some(regex)) = (self.search_pattern.clone(), self.search_regex.clone())
        else {
            self.status_message = Some("search: no previous pattern".to_string());
            return;
        };
        let direction = if reverse {
            self.search_direction.opposite()
        } else {
            self.search_direction
        };
        match self.find_match_line(&regex, direction) {
            Some((line, wrapped)) => {
                self.viewport.set_y_offset(line);
                if wrapped {
                    self.status_message = Some(format!(
                        "search: wrapped {} for /{}/",
                        if direction == SearchDirection::Forward {
                            "to top"
                        } else {
                            "to bottom"
                        },
                        pattern
                    ));
                } else {
                    self.status_message = Some(format!("search: /{}/", pattern));
                }
            }
            None => {
                self.status_message = Some(format!("search: no match for /{}/", pattern));
            }
        }
    }

    fn search_for_pattern(&mut self, pattern: &str, direction: SearchDirection) {
        let regex = match Regex::new(pattern) {
            Ok(compiled) => compiled,
            Err(err) => {
                self.status_message = Some(format!("search: invalid regex ({err})"));
                return;
            }
        };

        self.search_pattern = Some(pattern.to_string());
        self.search_regex = Some(regex.clone());
        self.search_direction = direction;

        match self.find_match_line(&regex, direction) {
            Some((line, wrapped)) => {
                self.viewport.set_y_offset(line);
                if wrapped {
                    self.status_message = Some(format!(
                        "search: wrapped {} for /{}/",
                        if direction == SearchDirection::Forward {
                            "to top"
                        } else {
                            "to bottom"
                        },
                        pattern
                    ));
                } else {
                    self.status_message = Some(format!("search: /{}/", pattern));
                }
            }
            None => {
                self.status_message = Some(format!("search: no match for /{}/", pattern));
            }
        }
    }

    fn find_match_line(&self, regex: &Regex, direction: SearchDirection) -> Option<(usize, bool)> {
        let lines = self.display_lines_for_width(self.viewport.width);
        if lines.is_empty() {
            return None;
        }

        let current = self.viewport.y_offset.min(lines.len().saturating_sub(1));
        let matches = |idx: usize| regex.is_match(&strip_ansi(&lines[idx]));

        match direction {
            SearchDirection::Forward => {
                for idx in (current + 1)..lines.len() {
                    if matches(idx) {
                        return Some((idx, false));
                    }
                }
                for idx in 0..=current {
                    if matches(idx) {
                        return Some((idx, true));
                    }
                }
            }
            SearchDirection::Backward => {
                for idx in (0..current).rev() {
                    if matches(idx) {
                        return Some((idx, false));
                    }
                }
                for idx in (current..lines.len()).rev() {
                    if matches(idx) {
                        return Some((idx, true));
                    }
                }
            }
        }

        None
    }

    fn handle_search_input(&mut self, key_msg: &KeyMsg) -> Option<Cmd> {
        if key_msg.key == KeyCode::Char('c') && key_msg.modifiers.contains(KeyModifiers::CONTROL) {
            return Some(quit());
        }

        let Some(prompt) = self.search_prompt.as_mut() else {
            return None;
        };

        match key_msg.key {
            KeyCode::Esc => {
                self.search_prompt = None;
                self.status_message = Some("search: cancelled".to_string());
            }
            KeyCode::Enter => {
                self.commit_search_prompt();
            }
            KeyCode::Backspace => {
                prompt.query.pop();
            }
            KeyCode::Char(ch)
                if !key_msg.modifiers.contains(KeyModifiers::CONTROL)
                    && !key_msg.modifiers.contains(KeyModifiers::ALT) =>
            {
                prompt.query.push(ch);
            }
            _ => {}
        }
        None
    }

    fn line_number_width(&self) -> usize {
        let max_number = self
            .display_line_numbers
            .iter()
            .copied()
            .flatten()
            .max()
            .unwrap_or(0);
        max_number.to_string().len().max(3)
    }

    fn render_header(&self) -> String {
        let filter_state = if self.filters_active() {
            format!("filter:{} active", self.filter_clauses.len())
        } else {
            "filter:off".to_string()
        };
        let spec_state = if self.fix_spec_conformance {
            "spec:on"
        } else {
            "spec:off"
        };
        let title = format!(
            " fixdecoder  {}  {}  {} ",
            self.source_label, filter_state, spec_state
        );
        format!(
            "{}{}{}{}{}",
            BG_HEADER,
            FG_ACCENT,
            BOLD,
            pad_or_trim(&title, self.term_width.max(20)),
            RESET
        )
    }

    fn render_body(&self) -> String {
        let number_width = self.line_number_width();
        let start_line = self.viewport.y_offset;
        let mut rendered = String::new();
        let body_rows = self.viewport.height.max(1);

        for idx in 0..body_rows {
            if idx > 0 {
                rendered.push('\n');
            }
            if let Some(line) = self.display_lines.get(start_line + idx) {
                let line_number_text = self
                    .display_line_numbers
                    .get(start_line + idx)
                    .and_then(|n| *n)
                    .map(|n| n.to_string())
                    .unwrap_or_default();
                let line_prefix = if line.contains("\x1b[") { "" } else { FG_TEXT };
                let line_rendered = render_visible_line(
                    line,
                    line_prefix,
                    self.search_regex.as_ref(),
                    self.viewport.x_offset,
                    self.viewport.width,
                );
                rendered.push_str(&format!(
                    "{}{:>width$}{} {}│{} {}{}{}",
                    FG_GUTTER,
                    line_number_text,
                    RESET,
                    FG_RULE,
                    RESET,
                    line_prefix,
                    line_rendered,
                    RESET,
                    width = number_width
                ));
            } else {
                rendered.push_str(&format!(
                    "{}{:>width$}{} {}│{} {}~{}",
                    FG_GUTTER,
                    "",
                    RESET,
                    FG_RULE,
                    RESET,
                    FG_DIM,
                    RESET,
                    width = number_width
                ));
            }
        }
        rendered
    }

    fn render_status(&self) -> String {
        if let Some(prompt) = self.search_prompt.as_ref() {
            let mut status = format!(
                " {}{}{}{}  Enter:search  Esc:cancel  Backspace:delete ",
                FG_ACCENT,
                prompt.direction.symbol(),
                FG_TEXT,
                prompt.query
            );
            status.push(' ');
            return format!(
                "{}{}{}{}{}",
                BG_STATUS,
                FG_TEXT,
                pad_or_trim(&status, self.term_width.max(20)),
                RESET,
                FG_DIM
            );
        }

        let total_lines = self.viewport.line_count().max(1);
        let line = self.viewport.y_offset + 1;
        let col = self.viewport.x_offset + 1;
        let scroll = (self.viewport.scroll_percent() * 100.0).round() as i32;
        let secrets_mode = if self.secret_enabled {
            format!("{}[s:secret on]{}", FG_ACCENT, FG_TEXT)
        } else {
            format!("{}[s:secret off]{}", FG_DIM, FG_TEXT)
        };
        let mode_state = if self.view_mode == ViewMode::Raw {
            format!("{}[r:raw on]{}", FG_ACCENT, FG_TEXT)
        } else {
            format!("{}[r:raw off]{}", FG_DIM, FG_TEXT)
        };
        let wrap_state = if self.wrap_enabled {
            format!("{}[w:wrap on]{}", FG_ACCENT, FG_TEXT)
        } else {
            format!("{}[w:wrap off]{}", FG_DIM, FG_TEXT)
        };
        let filter_state = if self.filters_active() {
            format!(
                "{}[f:{} match/{} total]{}",
                FG_ACCENT,
                self.filtered_message_count,
                self.fix_messages.len(),
                FG_TEXT
            )
        } else {
            format!("{}[f:filter off]{}", FG_DIM, FG_TEXT)
        };
        let conformance_state = if self.fix_spec_conformance {
            format!("{}[spec:on]{}", FG_ACCENT, FG_TEXT)
        } else {
            format!("{}[spec:off]{}", FG_DIM, FG_TEXT)
        };
        let horizontal_hint = if self.wrap_enabled {
            "wrap active"
        } else {
            "\u{2190}/\u{2192} scroll"
        };
        let col_display = if self.wrap_enabled {
            "-".to_string()
        } else {
            col.to_string()
        };

        let mut status = format!(
            " Ln {}/{}  Col {}  Scroll {:>3}%  {} {} {} {} {}  \u{2191}/\u{2193}/pgup/pgdn move  {}  / ? search  n/p next/prev  f filters ",
            line.min(total_lines),
            total_lines,
            col_display,
            scroll,
            mode_state,
            wrap_state,
            secrets_mode,
            filter_state,
            conformance_state,
            horizontal_hint
        );
        if let Some(msg) = self.status_message.as_ref() {
            status.push_str(&format!(" {}{}{} ", FG_WARN, msg, FG_TEXT));
        }
        if self.warning_count > 0 {
            status.push_str(&format!(
                " {}{} warning(s){} ",
                FG_WARN, self.warning_count, FG_TEXT
            ));
        }
        status.push_str(" h help  q quit ");
        format!(
            "{}{}{}{}{}",
            BG_STATUS,
            FG_TEXT,
            pad_or_trim(&status, self.term_width.max(20)),
            RESET,
            FG_DIM
        )
    }

    fn render_filter_dialog(&self) -> Option<String> {
        let dialog = self.filter_dialog.as_ref()?;
        let width = self.term_width.saturating_sub(6).max(30);
        let mut lines = Vec::<String>::new();
        let title = " FIX Filters ";
        let border = format!("+{}+", "-".repeat(width.saturating_sub(2)));
        lines.push(format!("{}{}{}{}", BG_DIALOG, FG_ACCENT, border, RESET));
        lines.push(format!(
            "{}{}|{}{}{}|{}",
            BG_DIALOG,
            FG_ACCENT,
            BOLD,
            pad_or_trim(title, width.saturating_sub(2)),
            FG_ACCENT,
            RESET
        ));

        match dialog.stage {
            FilterDialogStage::Main => {
                let checkbox = if self.fix_spec_conformance { "[x]" } else { "[ ]" };
                let rows = vec![
                    format!("{checkbox} FIX Spec conformance (restrict tag list by FIX version + MsgType)"),
                    "Add clause".to_string(),
                    "Delete selected clause".to_string(),
                    "Clear all clauses".to_string(),
                    "Close dialog".to_string(),
                ];
                let row_count = self.main_dialog_row_count();
                let selected = dialog.cursor.min(row_count.saturating_sub(1));

                lines.push(self.render_dialog_line(
                    &rows[0],
                    selected == 0,
                    width,
                ));
                if self.filter_clauses.is_empty() {
                    lines.push(self.render_dialog_line("No active clauses", false, width));
                } else {
                    for (idx, clause) in self.filter_clauses.iter().enumerate().take(5) {
                        let text = format!("{}. {}", idx + 1, clause.summary());
                        lines.push(self.render_dialog_line(
                            &text,
                            selected == idx + 1,
                            width,
                        ));
                    }
                    if self.filter_clauses.len() > 5 {
                        lines.push(self.render_dialog_line(
                            &format!("... {} more clause(s)", self.filter_clauses.len() - 5),
                            false,
                            width,
                        ));
                    }
                }
                let actions_start = 1 + self.filter_clauses.len();
                lines.push(self.render_dialog_line(
                    &rows[1],
                    selected == actions_start,
                    width,
                ));
                lines.push(self.render_dialog_line(
                    &rows[2],
                    selected == actions_start + 1,
                    width,
                ));
                lines.push(self.render_dialog_line(
                    &rows[3],
                    selected == actions_start + 2,
                    width,
                ));
                lines.push(self.render_dialog_line(
                    &rows[4],
                    selected == actions_start + 3,
                    width,
                ));
                lines.push(self.render_dialog_line(
                    "Use up/down + Enter. Also: a add, d delete, x clear, c toggle checkbox, Esc close",
                    false,
                    width,
                ));
            }
            FilterDialogStage::SelectMsgType => {
                lines.push(self.render_dialog_line("Select MsgType scope:", false, width));
                let mut options = vec!["Any message type".to_string()];
                options.extend(self.available_msg_types());
                let (start, end) = dialog_window(options.len(), dialog.cursor, 6);
                for (idx, option) in options[start..end].iter().enumerate() {
                    let absolute = start + idx;
                    let row_text = if absolute == 0 {
                        let any = if dialog.pending.msg_types.is_empty() {
                            "[x]"
                        } else {
                            "[ ]"
                        };
                        format!("{any} {option}")
                    } else {
                        let checked = dialog
                            .pending
                            .msg_types
                            .iter()
                            .any(|selected| selected == option);
                        let mark = if checked { "[x]" } else { "[ ]" };
                        format!("{mark} {option}")
                    };
                    lines.push(self.render_dialog_line(
                        &row_text,
                        absolute == dialog.cursor,
                        width,
                    ));
                }
                lines.push(self.render_dialog_line(
                    "Space: toggle MsgType  Enter: apply MsgType-only  t: continue to tag filter  Esc: back",
                    false,
                    width,
                ));
            }
            FilterDialogStage::SelectTag => {
                lines.push(self.render_dialog_line("Select FIX tag:", false, width));
                let options = self.available_tags_for_msg_types(&dialog.pending.msg_types);
                if options.is_empty() {
                    lines.push(self.render_dialog_line(
                        "No tags available in current scope",
                        false,
                        width,
                    ));
                } else {
                    let max_rows = 6usize;
                    let cursor = dialog.cursor.min(options.len().saturating_sub(1));
                    let (start, end) = dialog_window(options.len(), cursor, max_rows);
                    for (idx, option) in options[start..end].iter().enumerate() {
                        let absolute = start + idx;
                        lines.push(self.render_dialog_line(
                            &option.label(),
                            absolute == cursor,
                            width,
                        ));
                    }
                }
                let mode = if self.fix_spec_conformance { "on" } else { "off" };
                let scope = if dialog.pending.msg_types.is_empty() {
                    "any MsgType".to_string()
                } else {
                    format!("MsgType(s): {}", dialog.pending.msg_types.join(","))
                };
                lines.push(self.render_dialog_line(
                    &format!(
                        "{scope}  FIX Spec conformance: {mode}. Enter: choose  Esc: back"
                    ),
                    false,
                    width,
                ));
            }
            FilterDialogStage::SelectOperator => {
                let options = [
                    FilterOperator::Present,
                    FilterOperator::Equals,
                    FilterOperator::Regex,
                ];
                let tag = dialog.pending.tag.unwrap_or_default();
                let name = dialog.pending.tag_name.as_deref().unwrap_or("unknown");
                lines.push(self.render_dialog_line(
                    &format!("Select operator for {tag} ({name}):"),
                    false,
                    width,
                ));
                for (idx, op) in options.iter().enumerate() {
                    lines.push(self.render_dialog_line(
                        op.label(),
                        idx == dialog.cursor,
                        width,
                    ));
                }
                lines.push(self.render_dialog_line(
                    "Enter: choose  Esc: back",
                    false,
                    width,
                ));
            }
            FilterDialogStage::InputValue => {
                let op = dialog
                    .pending
                    .operator
                    .map(FilterOperator::label)
                    .unwrap_or("value");
                lines.push(self.render_dialog_line(
                    &format!("Enter {op} value/pattern:"),
                    false,
                    width,
                ));
                lines.push(self.render_dialog_line(
                    &format!("> {}", dialog.pending.value),
                    true,
                    width,
                ));
                lines.push(self.render_dialog_line(
                    "Enter: add clause  Esc: back  Backspace: delete",
                    false,
                    width,
                ));
            }
        }

        lines.push(format!("{}{}{}{}", BG_DIALOG, FG_ACCENT, border, RESET));
        Some(lines.join("\n"))
    }

    fn render_dialog_line(&self, text: &str, selected: bool, width: usize) -> String {
        let inner = width.saturating_sub(4);
        let content = pad_or_trim(text, inner);
        if selected {
            return format!(
                "{}{}| {}{}{} {}{}{} |{}{}",
                BG_DIALOG,
                FG_TEXT,
                BG_DIALOG_SELECTED,
                FG_DIALOG_SELECTED,
                BOLD,
                content,
                RESET,
                BG_DIALOG,
                FG_TEXT,
                RESET
            );
        }
        format!("{}{}| {} |{}", BG_DIALOG, FG_TEXT, content, RESET)
    }
}

impl help::KeyMap for FixUiModel {
    fn short_help(&self) -> Vec<&Binding> {
        vec![
            &self.viewport.keymap.up,
            &self.viewport.keymap.down,
            &self.bindings.search_forward,
            &self.bindings.search_backward,
            &self.bindings.open_filter,
            &self.bindings.toggle_wrap,
            &self.bindings.scroll_left,
            &self.bindings.scroll_right,
            &self.bindings.toggle_mode,
            &self.bindings.toggle_secret,
            &self.bindings.quit,
        ]
    }

    fn full_help(&self) -> Vec<Vec<&Binding>> {
        vec![
            vec![
                &self.viewport.keymap.up,
                &self.viewport.keymap.down,
                &self.viewport.keymap.page_up,
                &self.viewport.keymap.page_down,
            ],
            vec![
                &self.bindings.toggle_wrap,
                &self.bindings.scroll_left,
                &self.bindings.scroll_right,
            ],
            vec![
                &self.bindings.search_forward,
                &self.bindings.search_backward,
                &self.bindings.search_next,
                &self.bindings.search_prev,
                &self.bindings.open_filter,
            ],
            vec![
                &self.bindings.toggle_mode,
                &self.bindings.toggle_secret,
                &self.bindings.toggle_help,
                &self.bindings.quit,
            ],
        ]
    }
}

impl Model for FixUiModel {
    fn init() -> (Self, Option<Cmd>) {
        let bootstrap = UI_BOOTSTRAP
            .lock()
            .ok()
            .and_then(|mut slot| slot.take())
            .unwrap_or_else(|| UiBootstrap {
                raw_lines: vec!["No raw content loaded".to_string()],
                decoded_plain_lines: vec!["No decoded content loaded".to_string()],
                decoded_secret_lines: vec!["No decoded content loaded".to_string()],
                fix_messages: Vec::new(),
                initial_secret_enabled: false,
                source_label: "stdin".to_string(),
                warning_count: 0,
            });

        let model = Self::from_bootstrap(bootstrap);
        (model, Some(window_size()))
    }

    fn update(&mut self, msg: Msg) -> Option<Cmd> {
        if let Some(size_msg) = msg.downcast_ref::<WindowSizeMsg>() {
            self.term_width = usize::from(size_msg.width).max(20);
            self.term_height = usize::from(size_msg.height).max(10);
            self.refresh_layout();
            return None;
        }

        if let Some(key_msg) = msg.downcast_ref::<KeyMsg>() {
            if self.search_prompt.is_some() {
                return self.handle_search_input(key_msg);
            }
            if self.filter_dialog.is_some() {
                return self.handle_filter_input(key_msg);
            }
            if self.bindings.quit.matches(key_msg) {
                return Some(quit());
            }
            if self.bindings.toggle_help.matches(key_msg) {
                self.help.show_all = !self.help.show_all;
                self.refresh_layout();
                return None;
            }
            if self.bindings.toggle_mode.matches(key_msg) {
                self.toggle_view_mode();
                return None;
            }
            if self.bindings.toggle_wrap.matches(key_msg) {
                self.toggle_wrap_mode();
                return None;
            }
            if self.bindings.search_forward.matches(key_msg) {
                self.begin_search(SearchDirection::Forward);
                return None;
            }
            if self.bindings.search_backward.matches(key_msg) {
                self.begin_search(SearchDirection::Backward);
                return None;
            }
            if self.bindings.search_next.matches(key_msg) {
                self.repeat_search(false);
                return None;
            }
            if self.bindings.search_prev.matches(key_msg) {
                self.repeat_search(true);
                return None;
            }
            if self.bindings.open_filter.matches(key_msg) {
                self.open_filter_dialog();
                return None;
            }
            if self.bindings.toggle_secret.matches(key_msg) {
                self.toggle_secret_mode();
                return None;
            }
            if self.bindings.scroll_left.matches(key_msg) {
                if self.wrap_enabled {
                    return None;
                }
                self.viewport.scroll_left();
                return None;
            }
            if self.bindings.scroll_right.matches(key_msg) {
                if self.wrap_enabled {
                    return None;
                }
                self.viewport.scroll_right();
                return None;
            }
            let forwarded: Msg = Box::new(key_msg.clone());
            let _ = bubbletea_rs::Model::update(&mut self.viewport, forwarded);
        }
        None
    }

    fn view(&self) -> String {
        let mut output = String::new();
        output.push_str(&self.render_header());
        output.push('\n');
        output.push_str(&self.render_body());
        output.push('\n');
        if let Some(dialog) = self.render_filter_dialog() {
            output.push_str(&dialog);
            output.push('\n');
        }
        output.push_str(&self.render_status());
        output.push_str(RESET);
        output.push('\n');
        output.push_str(&self.help.view(self));
        output.push_str(RESET);
        output
    }
}

#[derive(Clone)]
struct LoadedInput {
    label: String,
    content: String,
}

fn render_visible_line(
    line: &str,
    default_restore_colour: &str,
    search_regex: Option<&Regex>,
    x_offset: usize,
    width: usize,
) -> String {
    if width == 0 {
        return String::new();
    }

    let mut rendered = String::new();
    let mut active_sgr = String::new();
    let match_ranges = search_regex
        .map(|regex| find_match_ranges(line, regex))
        .unwrap_or_default();
    let mut range_idx = 0usize;
    let mut in_match = false;
    let mut output_in_match = false;
    let mut output_started = false;
    let mut plain_offset = 0usize;
    let mut visual_col = 0usize;
    let mut output_col = 0usize;
    let bytes = line.as_bytes();
    let mut idx = 0;

    while idx < line.len() {
        if bytes[idx] == 0x1b && idx + 1 < line.len() && bytes[idx + 1] == b'[' {
            let mut end = idx + 2;
            while end < line.len() {
                let b = bytes[end];
                if (0x40..=0x7e).contains(&b) {
                    end += 1;
                    break;
                }
                end += 1;
            }
            let seq = &line[idx..end.min(line.len())];
            if seq.ends_with('m') {
                active_sgr.clear();
                active_sgr.push_str(seq);
            }
            if output_started && output_col < width {
                rendered.push_str(seq);
            }
            idx = end.min(line.len());
            continue;
        }

        let ch = line[idx..].chars().next().unwrap_or_default();
        let plain_start = plain_offset;
        plain_offset += ch.len_utf8();

        while range_idx < match_ranges.len() && match_ranges[range_idx].1 <= plain_start {
            range_idx += 1;
        }
        let matched = range_idx < match_ranges.len()
            && match_ranges[range_idx].0 < plain_offset
            && match_ranges[range_idx].1 > plain_start;
        if matched && !in_match {
            in_match = true;
        }

        let should_output = visual_col >= x_offset && output_col < width;
        if should_output && !output_started {
            output_started = true;
            if !active_sgr.is_empty() {
                rendered.push_str(&active_sgr);
            }
        }

        if should_output {
            if in_match && !output_in_match {
                rendered.push_str(MATCH_START);
                output_in_match = true;
            }
            if !in_match && output_in_match {
                rendered.push_str(MATCH_END);
                restore_active_colour(&mut rendered, &active_sgr, default_restore_colour);
                output_in_match = false;
            }
        }

        if should_output {
            if ch == SOH_CHAR || ch == SOH_VISIBLE_CHAR {
                rendered.push_str(SOH_START);
                rendered.push(SOH_VISIBLE_CHAR);
                rendered.push_str(SOH_END);
                if active_sgr.is_empty() {
                    rendered.push_str(default_restore_colour);
                } else {
                    rendered.push_str(&active_sgr);
                }
                if output_in_match {
                    rendered.push_str(MATCH_START);
                }
            } else {
                rendered.push(ch);
            }
            output_col += 1;
            if output_col >= width {
                break;
            }
        }

        if ch == SOH_CHAR {
            idx += 1;
        } else {
            idx += ch.len_utf8();
        }
        visual_col += 1;

        while range_idx < match_ranges.len() && match_ranges[range_idx].1 <= plain_offset {
            range_idx += 1;
        }
        let still_matched = range_idx < match_ranges.len()
            && match_ranges[range_idx].0 < plain_offset
            && match_ranges[range_idx].1 > plain_offset;
        if in_match && !still_matched {
            in_match = false;
            if output_in_match {
                rendered.push_str(MATCH_END);
                restore_active_colour(&mut rendered, &active_sgr, default_restore_colour);
                output_in_match = false;
            }
        }
    }

    if output_in_match {
        rendered.push_str(MATCH_END);
        restore_active_colour(&mut rendered, &active_sgr, default_restore_colour);
    }

    rendered
}

fn restore_active_colour(rendered: &mut String, active_sgr: &str, default_restore_colour: &str) {
    if active_sgr.is_empty() {
        rendered.push_str(default_restore_colour);
    } else {
        rendered.push_str(active_sgr);
    }
}

fn find_match_ranges(line: &str, regex: &Regex) -> Vec<(usize, usize)> {
    let plain = strip_ansi(line);
    regex
        .find_iter(&plain)
        .filter_map(|m| {
            if m.start() < m.end() {
                Some((m.start(), m.end()))
            } else {
                None
            }
        })
        .collect()
}

fn strip_ansi(line: &str) -> String {
    let mut plain = String::new();
    let bytes = line.as_bytes();
    let mut idx = 0usize;

    while idx < line.len() {
        if bytes[idx] == 0x1b && idx + 1 < line.len() && bytes[idx + 1] == b'[' {
            let mut end = idx + 2;
            while end < line.len() {
                let b = bytes[end];
                end += 1;
                if (0x40..=0x7e).contains(&b) {
                    break;
                }
            }
            idx = end.min(line.len());
            continue;
        }

        let ch = line[idx..].chars().next().unwrap_or_default();
        plain.push(ch);
        idx += ch.len_utf8();
    }

    plain
}

fn viewport_display_line(line: &str) -> String {
    line.chars()
        .map(|ch| if ch == SOH_CHAR { SOH_VISIBLE_CHAR } else { ch })
        .collect()
}

#[cfg(test)]
fn wrap_lines(lines: &[String], width: usize) -> Vec<String> {
    if width == 0 {
        return vec![String::new()];
    }

    let mut wrapped = Vec::new();
    for line in lines {
        wrapped.extend(wrap_single_line(line, width));
    }

    if wrapped.is_empty() {
        wrapped.push(String::new());
    }
    wrapped
}

fn wrap_single_line(line: &str, width: usize) -> Vec<String> {
    if line.is_empty() {
        return vec![String::new()];
    }

    let mut wrapped = Vec::new();
    let mut segment = String::new();
    let mut segment_width = 0usize;
    let mut active_sgr = String::new();
    let bytes = line.as_bytes();
    let mut idx = 0usize;

    while idx < line.len() {
        if bytes[idx] == 0x1b && idx + 1 < line.len() && bytes[idx + 1] == b'[' {
            let mut end = idx + 2;
            while end < line.len() {
                let b = bytes[end];
                end += 1;
                if (0x40..=0x7e).contains(&b) {
                    break;
                }
            }
            let seq = &line[idx..end.min(line.len())];
            segment.push_str(seq);
            if seq.ends_with('m') {
                if seq == "\x1b[0m" {
                    active_sgr.clear();
                } else {
                    active_sgr.clear();
                    active_sgr.push_str(seq);
                }
            }
            idx = end.min(line.len());
            continue;
        }

        if segment_width >= width {
            wrapped.push(segment);
            segment = String::new();
            if !active_sgr.is_empty() {
                segment.push_str(&active_sgr);
            }
            segment_width = 0;
        }

        let ch = line[idx..].chars().next().unwrap_or_default();
        segment.push(ch);
        segment_width += 1;
        idx += ch.len_utf8();
    }

    if !segment.is_empty() {
        wrapped.push(segment);
    } else if wrapped.is_empty() {
        wrapped.push(String::new());
    }

    wrapped
}

fn pad_or_trim(input: &str, width: usize) -> String {
    let mut out: String = input.chars().take(width).collect();
    let len = out.chars().count();
    if len < width {
        out.push_str(&" ".repeat(width - len));
    }
    out
}

fn dialog_window(total: usize, cursor: usize, max_rows: usize) -> (usize, usize) {
    if total <= max_rows {
        return (0, total);
    }
    let half = max_rows / 2;
    let mut start = cursor.saturating_sub(half);
    if start + max_rows > total {
        start = total.saturating_sub(max_rows);
    }
    (start, (start + max_rows).min(total))
}

fn source_label(files: &[String]) -> String {
    if files.is_empty() {
        return "stdin".to_string();
    }
    if files.len() == 1 {
        return if files[0] == "-" {
            "stdin".to_string()
        } else {
            files[0].clone()
        };
    }
    format!("{} inputs", files.len())
}

fn read_file_lossy(path: &Path) -> Result<String> {
    let bytes = fs::read(path).with_context(|| format!("failed to read {}", path.display()))?;
    Ok(String::from_utf8_lossy(&bytes).into_owned())
}

fn read_stdin_lossy() -> Result<String> {
    let mut bytes = Vec::new();
    std::io::stdin()
        .read_to_end(&mut bytes)
        .context("failed to read stdin")?;
    Ok(String::from_utf8_lossy(&bytes).into_owned())
}

fn load_inputs(files: &[String]) -> Result<Vec<LoadedInput>> {
    let mut loaded = Vec::new();
    let mut consumed_stdin = false;

    for file in files {
        if file == "-" {
            if consumed_stdin {
                return Err(anyhow!("stdin can only be consumed once in --ui mode"));
            }
            consumed_stdin = true;
            loaded.push(LoadedInput {
                label: "stdin".to_string(),
                content: read_stdin_lossy()?,
            });
        } else {
            loaded.push(LoadedInput {
                label: file.clone(),
                content: read_file_lossy(Path::new(file))?,
            });
        }
    }

    if loaded.is_empty() {
        loaded.push(LoadedInput {
            label: "stdin".to_string(),
            content: read_stdin_lossy()?,
        });
    }

    Ok(loaded)
}

fn build_raw_lines(inputs: &[LoadedInput]) -> Vec<String> {
    let mut lines = Vec::new();

    for (idx, input) in inputs.iter().enumerate() {
        if inputs.len() > 1 {
            if !lines.is_empty() {
                lines.push(String::new());
            }
            lines.push(format!("── {} ──", input.label));
        }
        for line in input.content.lines() {
            lines.push(line.to_string());
        }
        if input.content.is_empty() {
            lines.push(String::new());
        }
        if input.content.ends_with('\n') {
            let is_last = idx + 1 == inputs.len();
            if !is_last {
                lines.push(String::new());
            }
        }
    }

    if lines.is_empty() {
        lines.push("No raw input content available.".to_string());
    }

    lines
}

fn create_decode_temp_files(inputs: &[LoadedInput]) -> Result<(PathBuf, Vec<String>)> {
    let timestamp = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .as_nanos();
    let temp_dir = std::env::temp_dir().join(format!(
        "fixdecoder-ui-{}-{}",
        std::process::id(),
        timestamp
    ));

    fs::create_dir_all(&temp_dir)
        .with_context(|| format!("failed to create {}", temp_dir.display()))?;

    let mut paths = Vec::new();
    for (idx, input) in inputs.iter().enumerate() {
        let file_path = temp_dir.join(format!("input_{idx}.log"));
        fs::write(&file_path, input.content.as_bytes())
            .with_context(|| format!("failed to write {}", file_path.display()))?;
        paths.push(file_path.display().to_string());
    }

    Ok((temp_dir, paths))
}

fn collect_decoded_lines_from_inputs(
    inputs: &[LoadedInput],
    obfuscator: &fix::Obfuscator,
    display_delimiter: char,
    validate: bool,
    summary: bool,
    fix_override: Option<&str>,
) -> Result<(Vec<String>, usize, i32)> {
    let (temp_dir, decode_files) = create_decode_temp_files(inputs)?;

    let mut out_buf: Vec<u8> = Vec::new();
    let mut err_buf: Vec<u8> = Vec::new();
    let mut summary_state = summary.then(|| OrderSummary::new(display_delimiter));

    let mut ctx = PrettifyContext {
        out: &mut out_buf,
        err_out: &mut err_buf,
        obfuscator,
        display_delimiter,
        summary: &mut summary_state,
        fix_override,
        follow: false,
        live_status_enabled: false,
        validation_enabled: validate,
        message_counts: HashMap::<String, MsgTypeCount>::new(),
        counts_dirty: false,
        counts_height: 0,
        interrupted: crate::decoder::prettifier::interrupt_flag(),
    };

    let exit_code = prettify_files(&decode_files, &mut ctx);
    let _ = fs::remove_dir_all(&temp_dir);

    let mut lines: Vec<String> = String::from_utf8_lossy(&out_buf)
        .lines()
        .map(|line| line.to_string())
        .collect();

    if lines.is_empty() {
        lines.push("No FIX messages decoded for the selected input.".to_string());
    }

    let warning_count = String::from_utf8_lossy(&err_buf)
        .lines()
        .filter(|line| !line.trim().is_empty())
        .count();

    Ok((lines, warning_count, exit_code))
}

fn split_pretty_lines(pretty: &str) -> Vec<String> {
    let mut lines: Vec<String> = pretty.lines().map(|line| line.to_string()).collect();
    if lines.is_empty() {
        lines.push(String::new());
    }
    lines
}

fn apply_display_delimiter(text: &str, delimiter: char) -> String {
    if delimiter == SOH_CHAR {
        return text.to_string();
    }
    text.chars()
        .map(|ch| if ch == SOH_CHAR { delimiter } else { ch })
        .collect()
}

fn collect_fix_message_records(
    inputs: &[LoadedInput],
    plain_obfuscator: &fix::Obfuscator,
    secret_obfuscator: &fix::Obfuscator,
    display_delimiter: char,
    validate: bool,
    fix_override: Option<&str>,
) -> Vec<FixMessageRecord> {
    let mut records = Vec::new();

    for input in inputs {
        for capture in UI_FIX_REGEX.find_iter(&input.content) {
            let raw_message = capture.as_str().to_string();
            let fields = parse_fix(&raw_message);
            let msg_type = fields
                .iter()
                .find(|field| field.tag == 35)
                .map(|field| field.value.clone());

            let dict = tag_lookup::load_dictionary_with_override(&raw_message, fix_override);
            let report = validate.then(|| validator::validate_fix_message(&raw_message, &dict));

            let plain_message = plain_obfuscator.enabled_line(&raw_message);
            let secret_message = secret_obfuscator.enabled_line(&raw_message);
            let plain_line = apply_display_delimiter(&plain_message, display_delimiter);
            let secret_line = apply_display_delimiter(&secret_message, display_delimiter);

            let decoded_plain = prettify_with_report(&plain_message, &dict, report.as_ref());
            let decoded_secret = prettify_with_report(&secret_message, &dict, report.as_ref());
            let mut decoded_plain_lines = vec![plain_line];
            decoded_plain_lines.extend(split_pretty_lines(&decoded_plain));
            let mut decoded_secret_lines = vec![secret_line];
            decoded_secret_lines.extend(split_pretty_lines(&decoded_secret));

            records.push(FixMessageRecord {
                raw_message,
                decoded_plain_lines,
                decoded_secret_lines,
                fields,
                msg_type,
                dict,
            });
        }
    }

    records
}

pub fn run_ui(
    files: &[String],
    initial_secret_enabled: bool,
    display_delimiter: char,
    validate: bool,
    summary: bool,
    follow: bool,
    fix_override: Option<&str>,
) -> Result<i32> {
    if follow {
        return Err(anyhow!("--ui does not currently support --follow"));
    }
    if files.len() == 1 && files[0] == "-" && std::io::stdin().is_terminal() {
        return Err(anyhow!("--ui needs file input or piped stdin"));
    }

    let inputs = load_inputs(files)?;
    let raw_lines = build_raw_lines(&inputs);
    let plain_obfuscator = fix::create_obfuscator(false);
    let secret_obfuscator = fix::create_obfuscator(true);
    let filter_plain_obfuscator = fix::create_obfuscator(false);
    let filter_secret_obfuscator = fix::create_obfuscator(true);

    let (decoded_plain_lines, warning_plain, decode_plain) = collect_decoded_lines_from_inputs(
        &inputs,
        &plain_obfuscator,
        display_delimiter,
        validate,
        summary,
        fix_override,
    )?;
    let (decoded_secret_lines, warning_secret, decode_secret) = collect_decoded_lines_from_inputs(
        &inputs,
        &secret_obfuscator,
        display_delimiter,
        validate,
        summary,
        fix_override,
    )?;

    let warning_count = warning_plain.max(warning_secret);
    let decode_code = if initial_secret_enabled {
        decode_secret
    } else {
        decode_plain
    };
    let fix_messages = collect_fix_message_records(
        &inputs,
        &filter_plain_obfuscator,
        &filter_secret_obfuscator,
        display_delimiter,
        validate,
        fix_override,
    );

    let bootstrap = UiBootstrap {
        raw_lines,
        decoded_plain_lines,
        decoded_secret_lines,
        fix_messages,
        initial_secret_enabled,
        source_label: source_label(files),
        warning_count,
    };

    {
        let mut slot = UI_BOOTSTRAP
            .lock()
            .map_err(|_| anyhow!("failed to prepare UI state"))?;
        *slot = Some(bootstrap);
    }

    let program = Program::<FixUiModel>::builder()
        .alt_screen(true)
        .report_focus(true)
        .build()
        .context("failed to create UI program")?;

    let runtime = tokio::runtime::Builder::new_multi_thread()
        .enable_all()
        .build()
        .context("failed to create tokio runtime for UI")?;

    runtime
        .block_on(program.run())
        .map_err(|err| anyhow!("UI execution failed: {err}"))?;

    Ok(decode_code)
}

#[cfg(test)]
mod tests {
    use super::{
        FG_TEXT, FixUiModel, LoadedInput, MATCH_END, MATCH_START, SOH_END, SOH_START,
        SOH_VISIBLE_CHAR,
        SearchDirection, UiBootstrap, ViewMode, build_raw_lines, collect_fix_message_records,
        pad_or_trim, render_visible_line, source_label, strip_ansi, wrap_lines,
    };
    use crate::fix;
    use regex::Regex;
    use std::collections::HashSet;

    #[test]
    fn pad_or_trim_enforces_width() {
        assert_eq!(pad_or_trim("abc", 5), "abc  ");
        assert_eq!(pad_or_trim("abcdef", 3), "abc");
    }

    #[test]
    fn source_label_is_human_friendly() {
        assert_eq!(source_label(&[]), "stdin");
        assert_eq!(source_label(&["-".to_string()]), "stdin");
        assert_eq!(source_label(&["log.fix".to_string()]), "log.fix");
        assert_eq!(
            source_label(&["a.fix".to_string(), "b.fix".to_string()]),
            "2 inputs"
        );
    }

    #[test]
    fn build_raw_lines_adds_headers_for_multiple_inputs() {
        let inputs = vec![
            LoadedInput {
                label: "one.log".to_string(),
                content: "a\nb".to_string(),
            },
            LoadedInput {
                label: "two.log".to_string(),
                content: "c".to_string(),
            },
        ];

        let lines = build_raw_lines(&inputs);
        assert_eq!(lines[0], "── one.log ──");
        assert!(lines.contains(&"── two.log ──".to_string()));
    }

    #[test]
    fn decoded_view_switches_between_secret_modes() {
        let bootstrap = UiBootstrap {
            raw_lines: vec!["raw".to_string()],
            decoded_plain_lines: vec!["plain".to_string()],
            decoded_secret_lines: vec!["secret".to_string()],
            fix_messages: Vec::new(),
            initial_secret_enabled: false,
            source_label: "stdin".to_string(),
            warning_count: 0,
        };
        let mut model = FixUiModel::from_bootstrap(bootstrap);
        assert_eq!(
            model.active_decoded_lines().first().map(String::as_str),
            Some("plain")
        );
        model.secret_enabled = true;
        assert_eq!(
            model.active_decoded_lines().first().map(String::as_str),
            Some("secret")
        );
    }

    #[test]
    fn secret_toggle_in_raw_mode_applies_when_switching_back_to_decoded() {
        let bootstrap = UiBootstrap {
            raw_lines: vec!["raw".to_string()],
            decoded_plain_lines: vec!["plain".to_string()],
            decoded_secret_lines: vec!["secret".to_string()],
            fix_messages: Vec::new(),
            initial_secret_enabled: false,
            source_label: "stdin".to_string(),
            warning_count: 0,
        };
        let mut model = FixUiModel::from_bootstrap(bootstrap);

        model.set_view_mode(ViewMode::Raw);
        model.toggle_secret_mode();
        assert!(model.secret_enabled);

        model.toggle_view_mode();
        assert_eq!(model.view_mode, ViewMode::Decoded);
        assert_eq!(
            model.active_lines().first().map(String::as_str),
            Some("secret")
        );
    }

    #[test]
    fn render_visible_line_highlights_soh_in_plain_text() {
        let rendered = render_visible_line("8=FIX\u{0001}9=123", FG_TEXT, None, 0, 80);
        assert_eq!(
            rendered,
            format!("8=FIX{}␁{}{}9=123", SOH_START, SOH_END, FG_TEXT)
        );
    }

    #[test]
    fn render_visible_line_restores_active_ansi_after_soh() {
        let rendered = render_visible_line("\x1b[31mA\u{0001}B\x1b[0m", FG_TEXT, None, 0, 80);
        assert!(rendered.contains(&format!("{}␁{}\x1b[31m", SOH_START, SOH_END)));
    }

    #[test]
    fn render_visible_line_styles_preconverted_soh_marker() {
        let input = format!("8=FIX{SOH_VISIBLE_CHAR}9=123");
        let rendered = render_visible_line(&input, FG_TEXT, None, 0, 80);
        assert_eq!(
            rendered,
            format!("8=FIX{}␁{}{}9=123", SOH_START, SOH_END, FG_TEXT)
        );
    }

    #[test]
    fn render_visible_line_highlights_search_matches() {
        let regex = Regex::new("alpha").expect("regex compiles");
        let rendered = render_visible_line("prefix alpha suffix", FG_TEXT, Some(&regex), 0, 80);
        assert!(rendered.contains(&format!("{MATCH_START}alpha{MATCH_END}")));
    }

    #[test]
    fn render_visible_line_horizontal_slice_preserves_ansi_prefix() {
        let line = "\x1b[38;5;244mIN 2026-03-03 12:34:56.789\x1b[0m";
        let rendered = render_visible_line(line, FG_TEXT, None, 1, 20);
        assert!(
            rendered.contains("\x1b[38;5;244m"),
            "active colour prefix should remain valid after horizontal clipping"
        );
        assert!(
            !rendered.starts_with("38;5;244m"),
            "must not emit broken SGR fragments"
        );
    }

    #[test]
    fn horizontal_scroll_moves_viewport_offset() {
        let bootstrap = UiBootstrap {
            raw_lines: vec!["abcdefghijklmnopqrstuvwxyz".to_string()],
            decoded_plain_lines: vec!["plain".to_string()],
            decoded_secret_lines: vec!["secret".to_string()],
            fix_messages: Vec::new(),
            initial_secret_enabled: false,
            source_label: "stdin".to_string(),
            warning_count: 0,
        };
        let mut model = FixUiModel::from_bootstrap(bootstrap);
        model.term_width = 20;
        model.term_height = 10;
        model.set_view_mode(ViewMode::Raw);
        model.refresh_layout();

        assert_eq!(model.viewport.x_offset, 0);
        model.viewport.scroll_right();
        assert!(model.viewport.x_offset > 0);
    }

    #[test]
    fn wrap_mode_reflows_long_lines_and_resets_horizontal_offset() {
        let bootstrap = UiBootstrap {
            raw_lines: vec!["abcdefghijklmnopqrstuvwxyz".to_string()],
            decoded_plain_lines: vec!["plain".to_string()],
            decoded_secret_lines: vec!["secret".to_string()],
            fix_messages: Vec::new(),
            initial_secret_enabled: false,
            source_label: "stdin".to_string(),
            warning_count: 0,
        };
        let mut model = FixUiModel::from_bootstrap(bootstrap);
        model.term_width = 20;
        model.term_height = 10;
        model.set_view_mode(ViewMode::Raw);
        model.refresh_layout();
        let unwrapped_lines = model.viewport.line_count();

        model.viewport.scroll_right();
        assert!(model.viewport.x_offset > 0);

        model.toggle_wrap_mode();
        assert!(model.wrap_enabled);
        assert_eq!(model.viewport.x_offset, 0);
        assert!(model.viewport.line_count() > unwrapped_lines);
    }

    #[test]
    fn wrap_lines_preserves_active_ansi_style() {
        let lines = vec!["\x1b[31mabcdefghij\x1b[0m".to_string()];
        let wrapped = wrap_lines(&lines, 4);
        assert_eq!(wrapped.len(), 3);
        assert!(wrapped[1].starts_with("\x1b[31m"));
    }

    #[test]
    fn strip_ansi_removes_escape_sequences() {
        assert_eq!(strip_ansi("\x1b[31mERROR\x1b[0m"), "ERROR");
    }

    #[test]
    fn search_supports_forward_backward_and_repeat() {
        let bootstrap = UiBootstrap {
            raw_lines: vec![
                "line-1".to_string(),
                "line-2".to_string(),
                "line-3".to_string(),
                "line-4".to_string(),
                "alpha".to_string(),
                "line-6".to_string(),
                "line-7".to_string(),
                "middle".to_string(),
                "line-9".to_string(),
                "alpha".to_string(),
                "line-11".to_string(),
                "line-12".to_string(),
            ],
            decoded_plain_lines: vec!["plain".to_string()],
            decoded_secret_lines: vec!["secret".to_string()],
            fix_messages: Vec::new(),
            initial_secret_enabled: false,
            source_label: "stdin".to_string(),
            warning_count: 0,
        };
        let mut model = FixUiModel::from_bootstrap(bootstrap);
        model.set_view_mode(ViewMode::Raw);
        model.term_width = 80;
        model.term_height = 10;
        model.refresh_layout();

        model.search_for_pattern("alpha", SearchDirection::Forward);
        assert_eq!(model.viewport.y_offset, 4);

        model.repeat_search(false);
        assert_eq!(model.viewport.y_offset, 9);

        model.repeat_search(true);
        assert_eq!(model.viewport.y_offset, 4);

        model.search_for_pattern("middle", SearchDirection::Backward);
        assert_eq!(model.viewport.y_offset, 7);
    }

    #[test]
    fn search_matches_ansi_coloured_lines() {
        let bootstrap = UiBootstrap {
            raw_lines: vec![
                "line-1".to_string(),
                "line-2".to_string(),
                "line-3".to_string(),
                "line-4".to_string(),
                "line-5".to_string(),
                "\x1b[31mERROR\x1b[0m details".to_string(),
                "line-7".to_string(),
                "line-8".to_string(),
            ],
            decoded_plain_lines: vec!["plain".to_string()],
            decoded_secret_lines: vec!["secret".to_string()],
            fix_messages: Vec::new(),
            initial_secret_enabled: false,
            source_label: "stdin".to_string(),
            warning_count: 0,
        };
        let mut model = FixUiModel::from_bootstrap(bootstrap);
        model.set_view_mode(ViewMode::Raw);
        model.term_width = 80;
        model.term_height = 10;
        model.refresh_layout();

        model.search_for_pattern("ERROR", SearchDirection::Forward);
        assert_eq!(model.viewport.y_offset, 5);
    }

    #[test]
    fn fix_spec_conformance_checkbox_controls_tag_restriction() {
        let message = "8=FIX.4.4\u{0001}35=D\u{0001}11=ABC\u{0001}55=IBM\u{0001}9999=CUSTOM\u{0001}10=000\u{0001}";
        let inputs = vec![LoadedInput {
            label: "sample.fix".to_string(),
            content: message.to_string(),
        }];
        let plain_obfuscator = fix::create_obfuscator(false);
        let secret_obfuscator = fix::create_obfuscator(true);
        let fix_messages =
            collect_fix_message_records(&inputs, &plain_obfuscator, &secret_obfuscator, '|', false, None);

        let bootstrap = UiBootstrap {
            raw_lines: vec![message.to_string()],
            decoded_plain_lines: vec!["decoded".to_string()],
            decoded_secret_lines: vec!["decoded".to_string()],
            fix_messages,
            initial_secret_enabled: false,
            source_label: "stdin".to_string(),
            warning_count: 0,
        };
        let mut model = FixUiModel::from_bootstrap(bootstrap);

        let conformance_on = model.available_tags_for_msg_types(&["D".to_string()]);
        assert!(
            !conformance_on.iter().any(|option| option.tag == 9999),
            "non-spec tag should be excluded when conformance is enabled"
        );

        model.set_fix_spec_conformance(false);
        let conformance_off = model.available_tags_for_msg_types(&["D".to_string()]);
        assert!(
            conformance_off.iter().any(|option| option.tag == 9999),
            "observed tag should be selectable when conformance is disabled"
        );
    }

    #[test]
    fn filtered_decoded_records_include_fix_message_line() {
        let message = "8=FIX.4.4\u{0001}35=D\u{0001}11=ABC\u{0001}10=000\u{0001}";
        let inputs = vec![LoadedInput {
            label: "sample.fix".to_string(),
            content: message.to_string(),
        }];
        let plain_obfuscator = fix::create_obfuscator(false);
        let secret_obfuscator = fix::create_obfuscator(true);
        let records = collect_fix_message_records(
            &inputs,
            &plain_obfuscator,
            &secret_obfuscator,
            '|',
            false,
            None,
        );

        let first = records
            .first()
            .and_then(|record| record.decoded_plain_lines.first())
            .cloned()
            .unwrap_or_default();
        assert_eq!(first, "8=FIX.4.4|35=D|11=ABC|10=000|");
    }

    #[test]
    fn msgtype_only_clause_filters_without_tag_prompt() {
        let msg_d = "8=FIX.4.4\u{0001}35=D\u{0001}11=A\u{0001}9001=X\u{0001}10=000\u{0001}";
        let msg_8 = "8=FIX.4.4\u{0001}35=8\u{0001}17=1\u{0001}9002=Y\u{0001}10=000\u{0001}";
        let inputs = vec![LoadedInput {
            label: "sample.fix".to_string(),
            content: format!("{msg_d}\n{msg_8}"),
        }];
        let plain_obfuscator = fix::create_obfuscator(false);
        let secret_obfuscator = fix::create_obfuscator(true);
        let fix_messages = collect_fix_message_records(
            &inputs,
            &plain_obfuscator,
            &secret_obfuscator,
            '|',
            false,
            None,
        );

        let mut model = FixUiModel::from_bootstrap(UiBootstrap {
            raw_lines: vec![msg_d.to_string(), msg_8.to_string()],
            decoded_plain_lines: vec!["decoded".to_string()],
            decoded_secret_lines: vec!["decoded".to_string()],
            fix_messages,
            initial_secret_enabled: false,
            source_label: "stdin".to_string(),
            warning_count: 0,
        });

        let pending = super::PendingFilterClause {
            msg_types: vec!["D".to_string()],
            ..Default::default()
        };
        assert!(model.create_clause_from_pending(&pending));
        assert_eq!(model.filtered_message_count, 1);
        assert!(
            model.filtered_raw_lines.iter().any(|line| line.contains("35=D")),
            "MsgType D should be kept"
        );
        assert!(
            !model.filtered_raw_lines.iter().any(|line| line.contains("35=8")),
            "MsgType 8 should be excluded"
        );
    }

    #[test]
    fn conformance_with_multiple_msgtypes_uses_subset_of_tags() {
        let msg_d = "8=FIX.4.4\u{0001}35=D\u{0001}11=A\u{0001}9001=X\u{0001}10=000\u{0001}";
        let msg_8 = "8=FIX.4.4\u{0001}35=8\u{0001}17=1\u{0001}9002=Y\u{0001}10=000\u{0001}";
        let inputs = vec![LoadedInput {
            label: "sample.fix".to_string(),
            content: format!("{msg_d}\n{msg_8}"),
        }];
        let plain_obfuscator = fix::create_obfuscator(false);
        let secret_obfuscator = fix::create_obfuscator(true);
        let fix_messages = collect_fix_message_records(
            &inputs,
            &plain_obfuscator,
            &secret_obfuscator,
            '|',
            false,
            None,
        );

        let mut model = FixUiModel::from_bootstrap(UiBootstrap {
            raw_lines: vec![msg_d.to_string(), msg_8.to_string()],
            decoded_plain_lines: vec!["decoded".to_string()],
            decoded_secret_lines: vec!["decoded".to_string()],
            fix_messages,
            initial_secret_enabled: false,
            source_label: "stdin".to_string(),
            warning_count: 0,
        });

        model.set_fix_spec_conformance(false);
        let off = model.available_tags_for_msg_types(&["D".to_string(), "8".to_string()]);
        assert!(off.iter().any(|tag| tag.tag == 9001));
        assert!(off.iter().any(|tag| tag.tag == 9002));

        model.set_fix_spec_conformance(true);
        let d_only = model.available_tags_for_msg_types(&["D".to_string()]);
        let eight_only = model.available_tags_for_msg_types(&["8".to_string()]);
        let both = model.available_tags_for_msg_types(&["D".to_string(), "8".to_string()]);
        assert!(!both.iter().any(|tag| tag.tag == 9001));
        assert!(!both.iter().any(|tag| tag.tag == 9002));

        let d_set: HashSet<u32> = d_only.iter().map(|entry| entry.tag).collect();
        let eight_set: HashSet<u32> = eight_only.iter().map(|entry| entry.tag).collect();
        for tag in both.iter().map(|entry| entry.tag) {
            assert!(d_set.contains(&tag));
            assert!(eight_set.contains(&tag));
        }
    }

    #[test]
    fn toggling_view_mode_preserves_vertical_position() {
        let bootstrap = UiBootstrap {
            raw_lines: (1..=20).map(|n| format!("raw-{n}")).collect(),
            decoded_plain_lines: (1..=20).map(|n| format!("decoded-{n}")).collect(),
            decoded_secret_lines: (1..=20).map(|n| format!("decoded-{n}")).collect(),
            fix_messages: Vec::new(),
            initial_secret_enabled: false,
            source_label: "stdin".to_string(),
            warning_count: 0,
        };
        let mut model = FixUiModel::from_bootstrap(bootstrap);
        model.term_width = 100;
        model.term_height = 12;
        model.refresh_layout();
        model.viewport.set_y_offset(8);
        assert_eq!(model.viewport.y_offset, 8);

        model.set_view_mode(ViewMode::Raw);
        assert_eq!(model.viewport.y_offset, 8);

        model.set_view_mode(ViewMode::Decoded);
        assert_eq!(model.viewport.y_offset, 8);
    }

    #[test]
    fn line_numbers_only_label_fix_message_lines_in_raw_and_decoded() {
        let raw_fix_1 = "8=FIX.4.4\u{0001}35=D\u{0001}11=A\u{0001}10=000\u{0001}".to_string();
        let raw_fix_2 = "8=FIX.4.4\u{0001}35=8\u{0001}17=1\u{0001}10=000\u{0001}".to_string();
        let bootstrap = UiBootstrap {
            raw_lines: vec![
                "noise line".to_string(),
                raw_fix_1.clone(),
                "more noise".to_string(),
                raw_fix_2.clone(),
            ],
            decoded_plain_lines: vec![
                "noise line".to_string(),
                "8=FIX.4.4|35=D|11=A|10=000|".to_string(),
                "11 (ClOrdID): A".to_string(),
                "8=FIX.4.4|35=8|17=1|10=000|".to_string(),
                "17 (ExecID): 1".to_string(),
            ],
            decoded_secret_lines: vec![
                "noise line".to_string(),
                "8=FIX.4.4|35=D|11=A|10=000|".to_string(),
                "11 (ClOrdID): A".to_string(),
                "8=FIX.4.4|35=8|17=1|10=000|".to_string(),
                "17 (ExecID): 1".to_string(),
            ],
            fix_messages: Vec::new(),
            initial_secret_enabled: false,
            source_label: "stdin".to_string(),
            warning_count: 0,
        };
        let mut model = FixUiModel::from_bootstrap(bootstrap);
        model.term_width = 120;
        model.term_height = 20;

        model.set_view_mode(ViewMode::Raw);
        model.refresh_layout();
        assert_eq!(model.display_line_numbers, vec![None, Some(1), None, Some(2)]);

        model.set_view_mode(ViewMode::Decoded);
        model.refresh_layout();
        assert_eq!(
            model.display_line_numbers,
            vec![None, Some(1), None, Some(2), None]
        );
    }
}
