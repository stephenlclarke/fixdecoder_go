// SPDX-License-Identifier: AGPL-3.0-only
// Simple error logger that writes messages to a shared file and tracks count.

use once_cell::sync::Lazy;
use std::env;
use std::fs::OpenOptions;
use std::io::{self, Write};
use std::path::{Path, PathBuf};
use std::sync::Mutex;
use std::sync::atomic::{AtomicUsize, Ordering};

static LOG_PATH: Lazy<PathBuf> = Lazy::new(|| env::temp_dir().join("fixdecoder_errors.log"));

static LOG_FILE: Lazy<Mutex<Option<io::BufWriter<std::fs::File>>>> = Lazy::new(|| {
    let file = OpenOptions::new()
        .create(true)
        .append(true)
        .open(log_path())
        .ok();
    Mutex::new(file.map(io::BufWriter::new))
});

static ERROR_COUNT: AtomicUsize = AtomicUsize::new(0);

fn log_path() -> &'static Path {
    LOG_PATH.as_path()
}

pub fn log_error(message: &str) {
    if let Ok(mut file) = LOG_FILE.lock() {
        if let Some(file) = file.as_mut()
            && writeln!(file, "{message}").is_ok()
            && file.flush().is_ok()
        {
            ERROR_COUNT.fetch_add(1, Ordering::Relaxed);
        }
    }
}

pub fn summary() -> Option<(String, usize)> {
    let count = ERROR_COUNT.load(Ordering::Relaxed);
    if count > 0 {
        Some((log_path().display().to_string(), count))
    } else {
        None
    }
}
