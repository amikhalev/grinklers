package grinklers

import "sync/atomic"

// AtomicBool is a wrapper around an atomic uint32 which acts like a boolean
type AtomicBool uint32

func b2i(value bool) uint32 {
	if value {
		return 1
	}
	return 0
}

// NewAtomicBool creates a new AtomicBool with the specified value
func NewAtomicBool(value bool) AtomicBool {
	return AtomicBool(b2i(value))
}

// Store stores the specified value atomically in the AtomicBool
func (b *AtomicBool) Store(value bool) {
	atomic.StoreUint32((*uint32)(b), b2i(value))
}

// Load atomically loads the value from the AtomicBool
func (b *AtomicBool) Load() (value bool) {
	return atomic.LoadUint32((*uint32)(b)) != 0
}

// StoreIf stores the value if the AtomicBool is equal to the expected value.
// Returns true if the value was updated
func (b *AtomicBool) StoreIf(expected bool, value bool) (stored bool) {
	return atomic.CompareAndSwapUint32((*uint32)(b), b2i(expected), b2i(value))
}
