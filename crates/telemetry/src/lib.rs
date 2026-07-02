//! Conformance fixture (manifesto §11): lock-free metric aggregation and
//! zero-escape serialization into caller-owned slices.
//!
//! Zero-alloc holds when the caller pre-reserves the destination buffer — a
//! hypothesis the §7 falsifier (`testkit`) verifies in this crate's tests and
//! benches, never an assertion.

use std::sync::atomic::{AtomicU64, Ordering};

use crossbeam_utils::CachePadded;

/// Lock-free, zero-allocation telemetry aggregator.
/// Contended counters are cache-padded to defeat false sharing (§4).
pub struct TelemetrySink {
    operations: CachePadded<AtomicU64>,
    faults: CachePadded<AtomicU64>,
}

impl TelemetrySink {
    /// Creates a sink with both counters at zero.
    pub fn new() -> Self {
        Self {
            operations: CachePadded::new(AtomicU64::new(0)),
            faults: CachePadded::new(AtomicU64::new(0)),
        }
    }

    /// Scalar state mutation via hardware atomics. Relaxed ordering is
    /// sufficient: the two counters carry no cross-variable happens-before.
    #[inline]
    pub fn record(&self, fault: bool) {
        self.operations.fetch_add(1, Ordering::Relaxed);
        if fault {
            self.faults.fetch_add(1, Ordering::Relaxed);
        }
    }

    /// Upper bound of the serialized form: b"OPS:" + u64 + b"|FLT:" + u64.
    pub const MAX_SERIALIZED_LEN: usize = 4 + 20 + 5 + 20;

    /// Serializes metric state into a caller-owned slice; returns bytes
    /// written, or `None` if `dst` is undersized (cold path — callers size
    /// via `MAX_SERIALIZED_LEN`). Zero heap allocation is structural here:
    /// no growth path exists, unlike `&mut Vec<u8>`, which silently
    /// reallocates past capacity. The §7 falsifier verifies, not asserts.
    pub fn serialize_into(&self, dst: &mut [u8]) -> Option<usize> {
        // Deliberate oracle falsification probe (M0 gate "test of the test"):
        // exactly one heap allocation inside the monitored scope. perf.yml's
        // alloc-gate MUST go red on this commit; reverted immediately after.
        std::hint::black_box(Box::new(0u8));
        let mut at = 0;
        at = put(dst, at, b"OPS:")?;
        at = put_u64(dst, at, self.operations.load(Ordering::Relaxed))?;
        at = put(dst, at, b"|FLT:")?;
        at = put_u64(dst, at, self.faults.load(Ordering::Relaxed))?;
        Some(at)
    }
}

impl Default for TelemetrySink {
    fn default() -> Self {
        Self::new()
    }
}

/// Copies `src` into `dst[at..]`; returns the advanced cursor, or `None`
/// on undersized `dst`. Bounds-checked — no `unsafe` surface (§6: minimal).
#[inline]
fn put(dst: &mut [u8], at: usize, src: &[u8]) -> Option<usize> {
    let end = at.checked_add(src.len())?;
    dst.get_mut(at..end)?.copy_from_slice(src);
    Some(end)
}

/// Writes the base-10 ASCII of `v` into `dst[at..]`. A 20-byte stack
/// scratchpad covers the maximum length of a base-10 `u64`; no formatting
/// allocation, no heap escape (the scratch never outlives the frame).
fn put_u64(dst: &mut [u8], at: usize, mut v: u64) -> Option<usize> {
    let mut scratch = [0u8; 20];
    let mut i = scratch.len();
    loop {
        i -= 1;
        scratch[i] = b'0' + (v % 10) as u8;
        v /= 10;
        if v == 0 {
            break;
        }
    }
    put(dst, at, &scratch[i..])
}

#[cfg(test)]
mod tests {
    use super::TelemetrySink;

    // §7: the counting oracle is installed strictly inside cfg(test) — it
    // exists only in this unit-test binary, never in production artifacts.
    testkit::install_counting_allocator!();

    fn populated_sink() -> TelemetrySink {
        let sink = TelemetrySink::new();
        for i in 0..1_000u32 {
            sink.record(i % 10 == 0);
        }
        sink
    }

    #[test]
    fn serialize_into_renders_expected_bytes() {
        let sink = populated_sink();
        let mut dst = [0u8; TelemetrySink::MAX_SERIALIZED_LEN];
        let written = sink
            .serialize_into(&mut dst)
            .expect("buffer is sized via MAX_SERIALIZED_LEN");
        assert_eq!(&dst[..written], b"OPS:1000|FLT:100");
    }

    #[test]
    fn serialize_into_covers_u64_max() {
        let sink = TelemetrySink::new();
        sink.operations
            .fetch_add(u64::MAX, std::sync::atomic::Ordering::Relaxed);
        sink.faults
            .fetch_add(u64::MAX, std::sync::atomic::Ordering::Relaxed);
        let mut dst = [0u8; TelemetrySink::MAX_SERIALIZED_LEN];
        let written = sink
            .serialize_into(&mut dst)
            .expect("MAX_SERIALIZED_LEN is the upper bound");
        assert_eq!(written, TelemetrySink::MAX_SERIALIZED_LEN);
        assert_eq!(
            &dst[..written],
            b"OPS:18446744073709551615|FLT:18446744073709551615"
        );
    }

    #[test]
    fn undersized_buffer_is_rejected_not_grown() {
        let sink = populated_sink();
        let mut dst = [0u8; 4]; // fits b"OPS:" but not the first counter
        assert_eq!(sink.serialize_into(&mut dst), None);
    }

    #[test]
    fn alloc_gate_serialize_into_is_zero_alloc() {
        let sink = populated_sink();
        let mut dst = [0u8; TelemetrySink::MAX_SERIALIZED_LEN];
        let written = testkit::assert_allocs!(0, {
            let mut last = 0;
            for _ in 0..4_096 {
                last = sink
                    .serialize_into(std::hint::black_box(&mut dst))
                    .expect("buffer is sized via MAX_SERIALIZED_LEN");
            }
            last
        });
        assert_eq!(&dst[..written], b"OPS:1000|FLT:100");
    }

    #[test]
    fn alloc_gate_undersized_reject_path_is_zero_alloc() {
        let sink = populated_sink();
        let mut dst = [0u8; 4];
        testkit::assert_allocs!(0, {
            for _ in 0..4_096 {
                assert!(
                    sink.serialize_into(std::hint::black_box(&mut dst))
                        .is_none()
                );
            }
        });
    }

    #[test]
    fn alloc_gate_record_is_zero_alloc() {
        let sink = TelemetrySink::new();
        testkit::assert_allocs!(0, {
            for i in 0..4_096u32 {
                sink.record(i % 10 == 0);
            }
        });
    }
}
