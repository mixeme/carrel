//go:build !race

package backup

// scryptWork is the work factor of the key an archive is sealed under: 2^18,
// the one age itself takes — 256 MiB for every sealing and every opening.
const scryptWork = 18
