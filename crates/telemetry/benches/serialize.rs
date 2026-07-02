//! Criterion microbenchmark for `TelemetrySink::serialize_into`, run under
//! the §7 falsifier: the zero-alloc hypothesis is checked mechanically
//! before any timing is taken (a fast-but-allocating regression must fail,
//! not merely slow down).

// criterion_group! expands to an undocumented public item; not API surface.
#![allow(missing_docs)]

use std::hint::black_box;

use criterion::{Criterion, criterion_group, criterion_main};
use telemetry::TelemetrySink;

// Bench targets build with `harness = false` (no cfg(test)); installing the
// oracle at binary top level keeps it equally confined to non-production
// artifacts.
testkit::install_counting_allocator!();

fn populated_sink() -> TelemetrySink {
    let sink = TelemetrySink::new();
    for i in 0..1_000u32 {
        sink.record(i % 10 == 0);
    }
    sink
}

/// §7 gate: falsify `allocs == 0` over the exact loop criterion will time.
fn falsify_zero_alloc(sink: &TelemetrySink, dst: &mut [u8]) {
    let measurement = testkit::measure(|| {
        for _ in 0..4_096 {
            black_box(sink.serialize_into(black_box(dst)));
        }
    });
    assert_eq!(
        measurement.allocs, 0,
        "alloc falsifier: serialize_into hot loop performed {} allocation event(s); \
         the zero-alloc hypothesis is refuted (§7)",
        measurement.allocs,
    );
}

fn bench_serialize(c: &mut Criterion) {
    let sink = populated_sink();
    let mut dst = [0u8; TelemetrySink::MAX_SERIALIZED_LEN];

    // Gate first: refute the hypothesis before spending cycles timing it.
    falsify_zero_alloc(&sink, &mut dst);

    c.bench_function("serialize_into/max_sized_dst", |b| {
        b.iter(|| black_box(sink.serialize_into(black_box(&mut dst))))
    });

    c.bench_function("serialize_into/undersized_reject", |b| {
        let mut small = [0u8; 4];
        b.iter(|| black_box(sink.serialize_into(black_box(&mut small))))
    });
}

criterion_group!(benches, bench_serialize);
criterion_main!(benches);
