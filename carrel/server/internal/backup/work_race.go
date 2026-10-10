//go:build race

package backup

// scryptWork under the race detector, which no shipped binary is built with:
// 2^12. At 2^18 every sealing and opening takes 256 MiB, the detector's
// shadow of it several times more, and the suite does not fit into the
// 2 GB the Linux gates have (scripts/linux-gates.sh, 10.10.2026). The
// detector looks for races, not for the strength of the key; the factor the
// product seals with is checked without it (TestArchiveIsSealedAtFullWork).
const scryptWork = 12
