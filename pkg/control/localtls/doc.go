/*
Package localtls manages local certificate authority and server certificate generation for development environments.

This package provides automated TLS certificate provisioning, rotation, and operating-system-level
trust store injection (specifically on Windows) to enable secure local testing without external
certificate dependencies.
*/
package localtls
