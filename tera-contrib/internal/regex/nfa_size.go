package regex

import (
	"encoding/binary"
	"sync"
	"unicode/utf8"
)

// Rust's Unicode classes lower to byte-range tries. Counting rune ranges alone
// substantially overstates their size, so model the shared UTF-8 suffix states.
var classMemoryCache sync.Map

type byteInterval struct{ lo, hi byte }
type byteTransition struct {
	lo, hi byte
	next   uint32
}
type utf8Node struct {
	trans   []byteTransition
	last    byteInterval
	pending bool
}
type suffixEntry struct {
	key byteTransition
	id  uint32
}

type utf8Entry struct {
	trans []byteTransition
	id    uint32
}
type utf8Size struct {
	nodes  []utf8Node
	cache  map[uint64]utf8Entry
	next   uint32
	memory uint64
}

func unicodeClassMemory(rs []rune) uint64 {
	keyBytes := make([]byte, len(rs)*4)
	for i, r := range rs {
		binary.LittleEndian.PutUint32(keyBytes[i*4:], uint32(r))
	}
	key := string(keyBytes)
	if v, ok := classMemoryCache.Load(key); ok {
		return v.(uint64)
	}
	reverseMemory := uint64(64)
	reverseNext := uint32(2)
	suffixes := make(map[uint64]suffixEntry)
	c := utf8Size{nodes: []utf8Node{{}}, cache: make(map[uint64]utf8Entry), next: 1, memory: 32}
	for i := 0; i+1 < len(rs); i += 2 {
		utf8Sequences(interval{rs[i], rs[i+1]}, func(seq []byteInterval) {
			end := uint32(1)
			for _, b := range seq {
				key := byteTransition{b.lo, b.hi, end}
				h := uint64(14695981039346656037)
				h = (h ^ uint64(end)) * 1099511628211
				h = (h ^ uint64(b.lo)) * 1099511628211
				h = (h ^ uint64(b.hi)) * 1099511628211
				slot := h % 1000
				if old, ok := suffixes[slot]; ok && old.key == key {
					end = old.id
					continue
				}
				id := reverseNext
				reverseNext++
				reverseMemory += 32
				suffixes[slot] = suffixEntry{key, id}
				end = id
			}
			reverseMemory += 4

			common := 0
			for common < len(seq) && common < len(c.nodes) && c.nodes[common].pending && c.nodes[common].last == seq[common] {
				common++
			}
			c.freeze(common)
			last := len(c.nodes) - 1
			c.nodes[last].last = seq[common]
			c.nodes[last].pending = true
			for _, b := range seq[common+1:] {
				c.nodes = append(c.nodes, utf8Node{last: b, pending: true})
			}
		})
	}
	c.freeze(0)
	c.intern(c.nodes[0].trans)
	if reverseMemory > c.memory {
		c.memory = reverseMemory
	}
	classMemoryCache.Store(key, c.memory)
	return c.memory
}
func (c *utf8Size) freeze(from int) {
	next := uint32(0)
	for len(c.nodes) > from+1 {
		index := len(c.nodes) - 1
		n := c.nodes[index]
		c.nodes = c.nodes[:index]
		if n.pending {
			n.trans = append(n.trans, byteTransition{n.last.lo, n.last.hi, next})
		}
		next = c.intern(n.trans)
	}
	n := &c.nodes[len(c.nodes)-1]
	if n.pending {
		n.trans = append(n.trans, byteTransition{n.last.lo, n.last.hi, next})
		n.pending = false
	}
}
func (c *utf8Size) intern(trans []byteTransition) uint32 {
	h := uint64(14695981039346656037)
	for _, t := range trans {
		h = (h ^ uint64(t.lo)) * 1099511628211
		h = (h ^ uint64(t.hi)) * 1099511628211
		h = (h ^ uint64(t.next)) * 1099511628211
	}
	slot := h % 10000
	if old, ok := c.cache[slot]; ok && len(old.trans) == len(trans) {
		equal := true
		for i := range trans {
			if trans[i] != old.trans[i] {
				equal = false
				break
			}
		}
		if equal {
			return old.id
		}
	}
	id := c.next
	c.next++
	c.memory += 32 + uint64(len(trans))*8
	c.cache[slot] = utf8Entry{append([]byteTransition(nil), trans...), id}
	return id
}
func utf8Sequences(r interval, emit func([]byteInterval)) {
	stack := []interval{r}
	for len(stack) > 0 {
		index := len(stack) - 1
		r = stack[index]
		stack = stack[:index]
		for {
			if r.lo < 0xe000 && r.hi > 0xd7ff {
				stack = append(stack, interval{0xe000, r.hi})
				r.hi = 0xd7ff
				continue
			}
			if r.lo > r.hi {
				break
			}
			split := false
			for _, max := range []rune{0x7f, 0x7ff, 0xffff} {
				if r.lo <= max && max < r.hi {
					stack = append(stack, interval{max + 1, r.hi})
					r.hi = max
					split = true
					break
				}
			}
			if split {
				continue
			}
			if r.hi < 128 {
				emit([]byteInterval{{byte(r.lo), byte(r.hi)}})
				break
			}
			for i := 1; i < 4; i++ {
				mask := rune(1<<(6*i) - 1)
				if r.lo&^mask != r.hi&^mask {
					if r.lo&mask != 0 {
						stack = append(stack, interval{(r.lo | mask) + 1, r.hi})
						r.hi = r.lo | mask
						split = true
						break
					}
					if r.hi&mask != mask {
						stack = append(stack, interval{r.hi &^ mask, r.hi})
						r.hi = (r.hi &^ mask) - 1
						split = true
						break
					}
				}
			}
			if split {
				continue
			}
			var first, last [utf8.UTFMax]byte
			n := utf8.EncodeRune(first[:], r.lo)
			utf8.EncodeRune(last[:], r.hi)
			var seq [utf8.UTFMax]byteInterval
			for i := 0; i < n; i++ {
				seq[i] = byteInterval{first[i], last[i]}
			}
			emit(seq[:n])
			break
		}
	}
}
