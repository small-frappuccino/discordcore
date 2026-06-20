/*
Package control provides the primary control API and complementary dashboard-serving layer.

This package owns the control plane routing, auth/session handling, HTTP dashboard serving,
and settings feature routes for the complementary web surface. It must defer config rule
evaluation to pkg/files and preserve the boundary separating Discord runtime behavior
from dashboard orchestration.

Strict adherence to explicit context propagation, synchronized lifecycle transitions,
and zero-allocation observability pipelines is enforced across the control surface.
*/
package control
