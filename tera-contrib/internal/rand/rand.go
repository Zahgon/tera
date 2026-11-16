// Package rand reproduces seeded rand 0.10 StdRng sampling and shuffling.
package rand

import (
	"encoding/binary"
	"math/bits"
	mathrand "math/rand"
)

// SeedFromString hashes a Rust String with DefaultHasher (SipHash-1-3).
func SeedFromString(seed string) uint64 {
	b := append([]byte(seed), 0xff)
	v0, v1, v2, v3 := uint64(0x736f6d6570736575), uint64(0x646f72616e646f6d), uint64(0x6c7967656e657261), uint64(0x7465646279746573)
	round := func() {
		v0 += v1
		v1 = bits.RotateLeft64(v1, 13)
		v1 ^= v0
		v0 = bits.RotateLeft64(v0, 32)
		v2 += v3
		v3 = bits.RotateLeft64(v3, 16)
		v3 ^= v2
		v0 += v3
		v3 = bits.RotateLeft64(v3, 21)
		v3 ^= v0
		v2 += v1
		v1 = bits.RotateLeft64(v1, 17)
		v1 ^= v2
		v2 = bits.RotateLeft64(v2, 32)
	}
	i := 0
	for ; i+8 <= len(b); i += 8 {
		m := binary.LittleEndian.Uint64(b[i:])
		v3 ^= m
		round()
		v0 ^= m
	}
	m := uint64(len(b)) << 56
	for j, c := range b[i:] {
		m |= uint64(c) << uint(j*8)
	}
	v3 ^= m
	round()
	v0 ^= m
	v2 ^= 0xff
	round()
	round()
	round()
	return v0 ^ v1 ^ v2 ^ v3
}

// StdRng is the ChaCha12 generator used by rand's StdRng.
type StdRng struct {
	state [16]uint32
	block [16]uint32
	pos   int
}

// SeedFromU64 expands seed with rand_core's PCG32 initialization.
func SeedFromU64(seed uint64) *StdRng {
	r := &StdRng{pos: 16}
	r.state[0] = 0x61707865
	r.state[1] = 0x3320646e
	r.state[2] = 0x79622d32
	r.state[3] = 0x6b206574
	for i := 4; i < 12; i++ {
		seed = seed*0x5851f42d4c957f2d + 0xa17654e46fbe17f3
		x := uint32(((seed >> 18) ^ seed) >> 27)
		r.state[i] = bits.RotateLeft32(x, -int(seed>>59))
	}
	return r
}

func (r *StdRng) next32() uint32 {
	if r.pos == 16 {
		x := r.state
		qr := func(a, b, c, d int) {
			x[a] += x[b]
			x[d] = bits.RotateLeft32(x[d]^x[a], 16)
			x[c] += x[d]
			x[b] = bits.RotateLeft32(x[b]^x[c], 12)
			x[a] += x[b]
			x[d] = bits.RotateLeft32(x[d]^x[a], 8)
			x[c] += x[d]
			x[b] = bits.RotateLeft32(x[b]^x[c], 7)
		}
		for i := 0; i < 6; i++ {
			qr(0, 4, 8, 12)
			qr(1, 5, 9, 13)
			qr(2, 6, 10, 14)
			qr(3, 7, 11, 15)
			qr(0, 5, 10, 15)
			qr(1, 6, 11, 12)
			qr(2, 7, 8, 13)
			qr(3, 4, 9, 14)
		}
		for i := range x {
			r.block[i] = x[i] + r.state[i]
		}
		r.state[12]++
		if r.state[12] == 0 {
			r.state[13]++
		}
		r.pos = 0
	}
	v := r.block[r.pos]
	r.pos++
	return v
}
func (r *StdRng) next64() uint64 { low := r.next32(); return uint64(low) | uint64(r.next32())<<32 }

// RandomRangeI64 samples start..end using rand's single-sample integer algorithm.
// For an empty range it returns start; the caller should reject such ranges.
func (r *StdRng) RandomRangeI64(start, end int64) int64 {
	if start >= end {
		return start
	}
	bound := uint64(end) - uint64(start)
	hi, lo := bits.Mul64(r.next64(), bound)
	if lo > -bound {
		h, _ := bits.Mul64(r.next64(), bound)
		_, carry := bits.Add64(lo, h, 0)
		hi += carry
	}
	return int64(uint64(start) + hi)
}
func (r *StdRng) range32(bound uint32) uint32 {
	p := uint64(r.next32()) * uint64(bound)
	hi, lo := uint32(p>>32), uint32(p)
	if lo > -bound {
		h := uint32(uint64(r.next32()) * uint64(bound) >> 32)
		_, carry := bits.Add32(lo, h, 0)
		hi += carry
	}
	return hi
}

// Shuffle permutes n elements using SliceRandom's increasing-uniform algorithm.
func (r *StdRng) Shuffle(n int, swap func(i, j int)) {
	if n <= 1 {
		return
	}
	if uint64(n) >= uint64(^uint32(0)) {
		for i := 0; i < n; i++ {
			swap(i, int(r.RandomRangeI64(0, int64(i)+1)))
		}
		return
	}
	var chunk uint32
	remaining := 1
	for i := 0; i < n; i++ {
		m := uint32(i + 1)
		if remaining == 0 {
			bound := uint64(m)
			current := m + 1
			remaining = 1
			for current > m && bound*uint64(current) <= uint64(^uint32(0)) {
				bound *= uint64(current)
				current++
				remaining++
			}
			chunk = r.range32(uint32(bound))
		}
		remaining--
		index := chunk
		if remaining != 0 {
			index = chunk % m
			chunk /= m
		}
		swap(i, int(index))
	}
}

// RandomRangeI64 samples an unseeded range using Go's default random source.
func RandomRangeI64(start, end int64) int64 {
	if start >= end {
		return start
	}
	bound := uint64(end) - uint64(start)
	hi, _ := bits.Mul64(mathrand.Uint64(), bound)
	return int64(uint64(start) + hi)
}

// Shuffle shuffles n elements using Go's default random source.
func Shuffle(n int, swap func(i, j int)) {
	if n > 1 {
		mathrand.Shuffle(n, swap)
	}
}
