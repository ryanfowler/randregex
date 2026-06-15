package randregex

import (
	crand "crypto/rand"
	"fmt"
	"io"
	"sync"
)

// CryptoRand is a Rand value backed by crypto/rand.Reader.
//
// Pass it to GenerateWithRand or AppendWithRand when generated strings are
// used as secrets or other security-sensitive identifiers. CryptoRand is safe
// for concurrent use. It panics if crypto/rand.Reader fails or if IntN is
// called with n <= 0.
var CryptoRand = &cryptoRand{}

const cryptoRandBufferSize = 4096

type cryptoRand struct {
	mu  sync.Mutex
	buf [cryptoRandBufferSize]byte
	off int
	n   int
}

// IntN returns a uniform random integer in [0, n).
func (r *cryptoRand) IntN(n int) int {
	if n <= 0 {
		panic("randregex: CryptoRand.IntN called with n <= 0")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if n <= 1<<8 {
		return r.intN8Locked(n)
	}
	if n <= 1<<16 {
		return r.intN16Locked(n)
	}
	if uint64(n) <= 1<<32 {
		return r.intN32Locked(uint64(n))
	}
	return r.intN64Locked(uint64(n))
}

func (r *cryptoRand) intN8Locked(n int) int {
	threshold := (1 << 8) % n
	for {
		x := int(r.byteLocked())
		if x >= threshold {
			return x % n
		}
	}
}

func (r *cryptoRand) intN16Locked(n int) int {
	threshold := (1 << 16) % n
	for {
		x := int(r.uint16Locked())
		if x >= threshold {
			return x % n
		}
	}
}

func (r *cryptoRand) intN32Locked(n uint64) int {
	threshold := (uint64(1) << 32) % n
	for {
		x := uint64(r.uint32Locked())
		if x >= threshold {
			return int(x % n)
		}
	}
}

func (r *cryptoRand) intN64Locked(n uint64) int {
	threshold := -n % n
	for {
		x := r.uint64Locked()
		if x >= threshold {
			return int(x % n)
		}
	}
}

func (r *cryptoRand) uint64Locked() uint64 {
	v := uint64(r.uint32Locked())
	return v | uint64(r.uint32Locked())<<32
}

func (r *cryptoRand) uint32Locked() uint32 {
	v := uint32(r.uint16Locked())
	return v | uint32(r.uint16Locked())<<16
}

func (r *cryptoRand) uint16Locked() uint16 {
	v := uint16(r.byteLocked())
	return v | uint16(r.byteLocked())<<8
}

func (r *cryptoRand) byteLocked() byte {
	if r.off == r.n {
		r.refillLocked()
	}
	b := r.buf[r.off]
	r.off++
	return b
}

func (r *cryptoRand) refillLocked() {
	if _, err := io.ReadFull(crand.Reader, r.buf[:]); err != nil {
		panic(fmt.Errorf("randregex: crypto random source failed: %w", err))
	}
	r.off = 0
	r.n = len(r.buf)
}
