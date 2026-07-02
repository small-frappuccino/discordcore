//! Allocation-count oracle (manifesto §7).
//!
//! "0 allocations" is a hypothesis to *falsify*, not an invariant to declare.
//! This crate provides the falsifier: a counting [`GlobalAlloc`] wrapper over
//! the system allocator plus a measurement harness.
//!
//! # Regime
//!
//! The oracle must never link into production artifacts. Two mechanisms
//! enforce that:
//!
//! 1. Consumers depend on `testkit` only via `[dev-dependencies]`, so the
//!    crate is compiled exclusively for test and bench targets.
//! 2. Inside a library, the allocator is installed within a
//!    `#[cfg(test)]`-gated module. Bench targets (`harness = false`) build
//!    without `cfg(test)`; there, [`install_counting_allocator!`] is invoked
//!    at the top of the bench binary, which is equally outside any production
//!    artifact.
//!
//! # Usage
//!
//! ```
//! testkit::install_counting_allocator!();
//!
//! fn main() {
//!     let written = testkit::assert_allocs!(0, {
//!         let mut buf = [0u8; 64];
//!         buf[0] = b'!';
//!         buf[0]
//!     });
//!     assert_eq!(written, b'!');
//! }
//! ```
//!
//! # Accounting model
//!
//! The counter is **thread-local**: only allocation events performed by the
//! measuring thread are attributed to the measured scope. Allocator noise
//! from concurrent test threads or criterion bookkeeping threads cannot
//! pollute a measurement. `alloc`, `alloc_zeroed`, and `realloc` each count
//! as one event; `dealloc` is free (releasing memory is not an escape).

use std::alloc::{GlobalAlloc, Layout, System};
use std::cell::Cell;

thread_local! {
    // `const` initializer: no lazy-init branch, no TLS destructor
    // registration, and — critically — no heap allocation inside the
    // allocator's own accounting path.
    static ALLOC_EVENTS: Cell<u64> = const { Cell::new(0) };
}

/// Counting allocator: forwards to [`System`], recording one event per
/// `alloc`/`alloc_zeroed`/`realloc` on the calling thread.
///
/// Install with [`install_counting_allocator!`]; a manual
/// `#[global_allocator]` static works identically.
pub struct CountingAlloc;

#[inline]
fn bump() {
    ALLOC_EVENTS.with(|c| c.set(c.get() + 1));
}

// SAFETY: every method forwards layout/pointer arguments unchanged to the
// System allocator, so System's own contract (validity of returned pointers,
// layout fitting) is preserved verbatim. The thread-local bump performs no
// allocation and cannot reenter the allocator.
unsafe impl GlobalAlloc for CountingAlloc {
    unsafe fn alloc(&self, layout: Layout) -> *mut u8 {
        bump();
        // SAFETY: caller upholds GlobalAlloc's contract for `layout`;
        // forwarded unchanged.
        unsafe { System.alloc(layout) }
    }

    unsafe fn alloc_zeroed(&self, layout: Layout) -> *mut u8 {
        bump();
        // SAFETY: caller upholds GlobalAlloc's contract for `layout`;
        // forwarded unchanged.
        unsafe { System.alloc_zeroed(layout) }
    }

    unsafe fn realloc(&self, ptr: *mut u8, layout: Layout, new_size: usize) -> *mut u8 {
        bump();
        // SAFETY: caller guarantees `ptr` was allocated by this allocator
        // (which forwards to System) with `layout`; forwarded unchanged.
        unsafe { System.realloc(ptr, layout, new_size) }
    }

    unsafe fn dealloc(&self, ptr: *mut u8, layout: Layout) {
        // SAFETY: caller guarantees `ptr`/`layout` match a prior allocation
        // from this allocator; forwarded unchanged.
        unsafe { System.dealloc(ptr, layout) }
    }
}

/// Total allocation events recorded on the current thread since it started.
///
/// Monotonic; meaningful only as a delta (see [`measure`]). Returns 0 forever
/// if [`CountingAlloc`] is not the installed global allocator.
#[inline]
pub fn alloc_events() -> u64 {
    ALLOC_EVENTS.with(|c| c.get())
}

/// Result of a [`measure`] run: the allocation-event delta and the closure's
/// return value.
#[derive(Debug)]
pub struct Measurement<R> {
    /// Allocation events attributed to the measured scope (current thread).
    pub allocs: u64,
    /// Value returned by the measured closure.
    pub value: R,
}

/// Runs `f` and reports the allocation events it performed on this thread.
///
/// The closure result is routed through [`std::hint::black_box`] so LLVM
/// cannot fold the measured computation away and vacuously report zero.
#[inline]
pub fn measure<R>(f: impl FnOnce() -> R) -> Measurement<R> {
    let before = alloc_events();
    let value = std::hint::black_box(f());
    Measurement {
        allocs: alloc_events() - before,
        value,
    }
}

/// Asserts the exact allocation-event count of a scope; evaluates to the
/// scope's value.
///
/// ```
/// testkit::install_counting_allocator!();
///
/// fn main() {
///     let v = testkit::assert_allocs!(0, { 2 + 2 });
///     assert_eq!(v, 4);
/// }
/// ```
#[macro_export]
macro_rules! assert_allocs {
    ($expected:expr, $body:expr $(,)?) => {{
        let measurement = $crate::measure(|| $body);
        assert_eq!(
            measurement.allocs, $expected,
            "allocation falsifier: scope performed {} allocation event(s), expected {} (§7)",
            measurement.allocs, $expected,
        );
        measurement.value
    }};
}

/// Installs [`CountingAlloc`] as the global allocator of the enclosing
/// binary.
///
/// Invoke once per test/bench binary: inside `#[cfg(test)] mod tests` for
/// library unit tests, or at the top level of a bench/integration-test file.
#[macro_export]
macro_rules! install_counting_allocator {
    () => {
        #[global_allocator]
        static TESTKIT_COUNTING_ALLOC: $crate::CountingAlloc = $crate::CountingAlloc;
    };
}

#[cfg(test)]
mod tests {
    // §7: the oracle strictly under cfg(test) — this binary is the lib's
    // unit-test target only.
    crate::install_counting_allocator!();

    #[test]
    fn stack_only_scope_counts_zero() {
        let m = super::measure(|| {
            let mut buf = [0u8; 128];
            buf[42] = 7;
            std::hint::black_box(buf[42])
        });
        assert_eq!(m.allocs, 0);
        assert_eq!(m.value, 7);
    }

    #[test]
    fn boxed_scope_counts_one() {
        let m = super::measure(|| Box::new(0xDEAD_BEEF_u64));
        assert_eq!(m.allocs, 1);
        assert_eq!(*m.value, 0xDEAD_BEEF);
    }

    #[test]
    // vec![] would collapse this into a single allocation; the explicit
    // with_capacity + push sequence exists precisely to provoke the realloc
    // event being counted.
    #[allow(clippy::vec_init_then_push)]
    fn vec_growth_counts_reallocs() {
        let m = super::measure(|| {
            let mut v = Vec::with_capacity(1); // 1 alloc
            v.push(0u64);
            v.push(1u64); // growth: 1 realloc event
            v
        });
        assert_eq!(m.allocs, 2);
        assert_eq!(m.value.len(), 2);
    }

    #[test]
    fn assert_allocs_passes_value_through() {
        let v = crate::assert_allocs!(1, { Box::new(3u8) });
        assert_eq!(*v, 3);
    }
}
