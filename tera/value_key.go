package tera

// Key is a string, integer, or boolean map key.
type Key struct{ v Value }

// KeyFromValue validates and constructs a map key.
func KeyFromValue(v Value) (Key, error) {
	if v.IsString() || v.IsBool() || v.IsNumber() && !v.IsF64() {
		if v.IsString() {
			v.safe = false
		}
		return Key{v}, nil
	}
	return Key{}, ErrorMessage("Not a valid key type")
}

// StringKey constructs a string key.
func StringKey(s string) Key { return Key{NormalString(s)} }

// AsStr returns a string key's text.
func (k Key) AsStr() (string, bool) { return k.v.AsStr() }

// AsValue returns the equivalent value.
func (k Key) AsValue() Value { return k.v }

// String returns the Rust-compatible text representation.
func (k Key) String() string { return k.v.String() }

// Equal compares keys across numeric variants.
func (k Key) Equal(o Key) bool { return k.v.Equal(o.v) }

// Compare orders keys with bool before numbers before strings.
func (k Key) Compare(o Key) int { return k.v.Compare(o.v) }

// MapEntry is an ordered map entry.
type MapEntry struct {
	// Key is the entry key.
	Key Key
	// Value is the associated value.
	Value Value
}

// Map preserves insertion order and the position of replaced keys.
type Map struct{ entries []MapEntry }

// NewMap constructs an empty map.
func NewMap() *Map { return &Map{} }

// Len returns the number of entries.
func (m *Map) Len() int {
	if m == nil {
		return 0
	}
	return len(m.entries)
}

// Insert adds or replaces a key.
func (m *Map) Insert(k Key, v Value) {
	for i, p := range m.entries {
		if p.Key.Equal(k) {
			m.entries[i].Value = v
			return
		}
	}
	m.entries = append(m.entries, MapEntry{k, v})
}

// InsertString adds or replaces a string key.
func (m *Map) InsertString(k string, v Value) { m.Insert(StringKey(k), v) }

// Get finds a key.
func (m *Map) Get(k Key) (Value, bool) {
	if m != nil {
		for _, p := range m.entries {
			if p.Key.Equal(k) {
				return p.Value, true
			}
		}
	}
	return Undefined(), false
}

// GetString finds a string key.
func (m *Map) GetString(k string) (Value, bool) { return m.Get(StringKey(k)) }

// Remove removes a key, preserving the order of other keys.
func (m *Map) Remove(k Key) (Value, bool) {
	for i, p := range m.entries {
		if p.Key.Equal(k) {
			m.entries = append(m.entries[:i], m.entries[i+1:]...)
			return p.Value, true
		}
	}
	return Undefined(), false
}

// Iter returns entries in insertion order.
func (m *Map) Iter() []MapEntry {
	if m == nil {
		return nil
	}
	return append([]MapEntry{}, m.entries...)
}

// Keys returns the keys in insertion order.
func (m *Map) Keys() []Key {
	a := []Key{}
	for _, p := range m.Iter() {
		a = append(a, p.Key)
	}
	return a
}

// Clone copies a map.
func (m *Map) Clone() *Map { return &Map{entries: m.Iter()} }
