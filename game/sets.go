package game

// SetsValueTrue is the value every name in a SetsField is stored as. Sets are
// presence-only: the name being written is the whole signal.
const SetsValueTrue = "true"

// SetsField lists the variable names written when a block or objective context
// completes. Each name is set to SetsValueTrue.
type SetsField []string
