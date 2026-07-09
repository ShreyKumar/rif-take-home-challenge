// Standalone, dependency-free load generator. Kept in its own module so the
// backend module stays free of any load-testing code, and so it can be built
// and run with the standard library alone (no external tools required).
module loadgen

go 1.25
