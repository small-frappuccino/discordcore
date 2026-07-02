//! `cargo xtask` — internal automation (zero external dependencies).
//!
//! Subcommands:
//! - `alloc-check`        §7 allocation-regression gate: runs the falsifier
//!   test/bench suite in release mode; any counted allocation fails the build.
//! - `bench-diff`         runs the criterion suite; compares against a saved
//!   baseline when one exists (`--save-baseline` on default branch, compare
//!   elsewhere).
//! - `llvm-lines-report`  per-instantiation code-mass report (§4 dispatch
//!   gate oracle); requires `cargo-llvm-lines`.

use std::process::{Command, ExitCode};

fn main() -> ExitCode {
    let args: Vec<String> = std::env::args().skip(1).collect();
    let (cmd, rest) = match args.split_first() {
        Some((c, r)) => (c.as_str(), r),
        None => {
            eprintln!("usage: cargo xtask <alloc-check|bench-diff|llvm-lines-report>");
            return ExitCode::FAILURE;
        }
    };

    let ok = match cmd {
        "alloc-check" | "alloc-regression-check" => alloc_check(),
        "bench-diff" => bench_diff(rest),
        "llvm-lines-report" => llvm_lines_report(),
        other => {
            eprintln!("unknown xtask subcommand: {other}");
            eprintln!("usage: cargo xtask <alloc-check|bench-diff|llvm-lines-report>");
            false
        }
    };
    if ok {
        ExitCode::SUCCESS
    } else {
        ExitCode::FAILURE
    }
}

fn run(step: &str, cmd: &mut Command) -> bool {
    eprintln!("[xtask] {step}: {cmd:?}");
    match cmd.status() {
        Ok(status) if status.success() => true,
        Ok(status) => {
            eprintln!("[xtask] {step} FAILED ({status})");
            false
        }
        Err(err) => {
            eprintln!("[xtask] {step} could not start: {err}");
            false
        }
    }
}

/// §7 gate. Release mode: the falsifier must hold on the optimized artifact
/// that ships, and optimization can only *remove* allocations — a release
/// failure is therefore always a genuine regression.
fn alloc_check() -> bool {
    run(
        "alloc-check/tests",
        Command::new("cargo").args(["test", "--release", "-p", "testkit", "-p", "telemetry"]),
    ) && run(
        "alloc-check/bench-falsifier",
        // `--test` runs each criterion bench once (no sampling): executes the
        // falsify_zero_alloc gate embedded in the bench binary.
        Command::new("cargo").args([
            "bench",
            "-p",
            "telemetry",
            "--bench",
            "serialize",
            "--",
            "--test",
        ]),
    )
}

/// Criterion run. With `--baseline <name>` compares (fails on missing
/// baseline); with `--save-baseline <name>` records. Extra args pass through.
fn bench_diff(rest: &[String]) -> bool {
    let mut cmd = Command::new("cargo");
    cmd.args(["bench", "-p", "telemetry", "--bench", "serialize", "--"]);
    if rest.is_empty() {
        cmd.arg("--noplot");
    } else {
        cmd.args(rest);
    }
    run("bench-diff", &mut cmd)
}

/// §4 oracle: per-instantiation code mass. Report-only at M0 (the
/// monomorphize-vs-dyn decisions it adjudicates arrive with M2's ingest
/// crate); CI uploads the artifact so diffs are reviewable.
fn llvm_lines_report() -> bool {
    let out = match Command::new("cargo")
        .args(["llvm-lines", "-p", "telemetry", "--release"])
        .output()
    {
        Ok(out) if out.status.success() => out,
        Ok(out) => {
            eprintln!(
                "[xtask] llvm-lines-report FAILED:\n{}",
                String::from_utf8_lossy(&out.stderr)
            );
            return false;
        }
        Err(err) => {
            eprintln!(
                "[xtask] cargo-llvm-lines not runnable ({err}); install with `cargo install cargo-llvm-lines`"
            );
            return false;
        }
    };

    let report = String::from_utf8_lossy(&out.stdout);
    let dir = std::path::Path::new("target/xtask");
    if let Err(err) = std::fs::create_dir_all(dir) {
        eprintln!("[xtask] cannot create {}: {err}", dir.display());
        return false;
    }
    let path = dir.join("llvm-lines-telemetry.txt");
    if let Err(err) = std::fs::write(&path, report.as_bytes()) {
        eprintln!("[xtask] cannot write {}: {err}", path.display());
        return false;
    }
    // Echo the head so the CI log carries the summary inline.
    for line in report.lines().take(25) {
        println!("{line}");
    }
    println!("[xtask] full report: {}", path.display());
    true
}
