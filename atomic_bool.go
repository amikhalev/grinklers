package grinklers

import "sync/atomic"

type AtomicBool uint32

func b2i(value bool) uint32 {
	if value {
		return 1
	} else {
		return 0
	}
}

func NewAtomicBool(value bool) AtomicBool {
	return AtomicBool(b2i(value))
}

func (b *AtomicBool) Store(value bool) {
	atomic.StoreUint32((*uint32)(b), b2i(value))
}

func (b *AtomicBool) Load() (value bool) {
	return atomic.LoadUint32((*uint32)(b)) != 0
}

func (b *AtomicBool) StoreIf(expected bool, value bool) (stored bool) {
	return atomic.CompareAndSwapUint32((*uint32)(b), b2i(expected), b2i(value))
}
