// Package orderedmap holds a map that keeps its insertion order.
//
// A JSON Schema needs that for properties. The manager's web UI builds a form
// from the schema, and the form has to show the fields in the order the module
// declared them, not in map order.
//
// It replaces github.com/pb33f/ordered-map/v2. That package carries a generic
// linked list and a third-party JSON parser to do the same job, because
// encoding/json could not read an object's members in document order. json/v2
// can, so the whole thing fits here.
//
// A key is a string rather than any comparable type, because a JSON object
// names its members with strings. Any other key type would need its own
// encoding, which is most of what the replaced package spent its lines on.
package orderedmap

import (
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"fmt"
	"iter"
)

// The v2 methods rather than MarshalJSON and UnmarshalJSON, so the encoder and
// the decoder carry the caller's options. Quoting a key with v1 json.Marshal
// would escape HTML even for a caller that asked json/v2 not to. Go 1.27
// encoding/json calls these, so a v1 caller needs nothing.
var (
	_ jsonv2.MarshalerTo     = &OrderedMap[string, any]{}
	_ jsonv2.UnmarshalerFrom = &OrderedMap[string, any]{}
)

type Pair[K ~string, V any] struct {
	Key   K
	Value V

	prev *Pair[K, V]
	next *Pair[K, V]
}

func (p *Pair[K, V]) Next() *Pair[K, V] {
	return p.next
}

func (p *Pair[K, V]) Prev() *Pair[K, V] {
	return p.prev
}

type OrderedMap[K ~string, V any] struct {
	pairs  map[K]*Pair[K, V]
	oldest *Pair[K, V]
	newest *Pair[K, V]
}

func New[K ~string, V any]() *OrderedMap[K, V] {
	return &OrderedMap[K, V]{pairs: make(map[K]*Pair[K, V])}
}

func (om *OrderedMap[K, V]) Get(key K) (V, bool) {
	if om == nil {
		var zero V
		return zero, false
	}

	pair, found := om.pairs[key]
	if !found {
		var zero V
		return zero, false
	}

	return pair.Value, true
}

// Set returns what Get would have returned before the call. An existing key
// keeps its position and takes the new value.
func (om *OrderedMap[K, V]) Set(key K, value V) (V, bool) {
	if om.pairs == nil {
		om.pairs = make(map[K]*Pair[K, V])
	}

	pair, found := om.pairs[key]
	if found {
		old := pair.Value
		pair.Value = value

		return old, true
	}

	pair = &Pair[K, V]{Key: key, Value: value, prev: om.newest}
	om.pairs[key] = pair

	if om.oldest == nil {
		om.oldest = pair
	} else {
		om.newest.next = pair
	}

	om.newest = pair

	var zero V

	return zero, false
}

func (om *OrderedMap[K, V]) Delete(key K) (V, bool) {
	if om == nil {
		var zero V
		return zero, false
	}

	pair, found := om.pairs[key]
	if !found {
		var zero V
		return zero, false
	}

	if pair.prev == nil {
		om.oldest = pair.next
	} else {
		pair.prev.next = pair.next
	}

	if pair.next == nil {
		om.newest = pair.prev
	} else {
		pair.next.prev = pair.prev
	}

	delete(om.pairs, key)

	return pair.Value, true
}

func (om *OrderedMap[K, V]) Len() int {
	if om == nil {
		return 0
	}

	return len(om.pairs)
}

func (om *OrderedMap[K, V]) Oldest() *Pair[K, V] {
	if om == nil {
		return nil
	}

	return om.oldest
}

func (om *OrderedMap[K, V]) Newest() *Pair[K, V] {
	if om == nil {
		return nil
	}

	return om.newest
}

// All yields every pair from the oldest to the newest. Deleting the pair the
// loop is on is safe, the way it is with a Go map: Delete relinks the neighbors
// and leaves the removed pair pointing at the next one.
func (om *OrderedMap[K, V]) All() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for pair := om.Oldest(); pair != nil; pair = pair.Next() {
			if !yield(pair.Key, pair.Value) {
				return
			}
		}
	}
}

func (om *OrderedMap[K, V]) MarshalJSONTo(enc *jsontext.Encoder) error {
	if om == nil || om.pairs == nil {
		return enc.WriteToken(jsontext.Null)
	}

	err := enc.WriteToken(jsontext.BeginObject)
	if err != nil {
		return err
	}

	for pair := om.Oldest(); pair != nil; pair = pair.Next() {
		err = enc.WriteToken(jsontext.String(string(pair.Key)))
		if err != nil {
			return err
		}

		// No options, so the value inherits the ones the encoder already holds.
		err = jsonv2.MarshalEncode(enc, pair.Value)
		if err != nil {
			return err
		}
	}

	return enc.WriteToken(jsontext.EndObject)
}

func (om *OrderedMap[K, V]) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	open, err := dec.ReadToken()
	if err != nil {
		return err
	}

	if open.Kind() == 'n' {
		return nil
	}

	if open.Kind() != '{' {
		return fmt.Errorf("expected a JSON object, got %q", open.Kind())
	}

	for dec.PeekKind() != '}' {
		name, err := dec.ReadToken()
		if err != nil {
			return err
		}

		// The next read invalidates the token, so take the key before decoding
		// the value. Reading it afterward panics inside jsontext.
		key := K(name.String())

		var value V

		// No options, so the value inherits the ones the decoder already holds.
		err = jsonv2.UnmarshalDecode(dec, &value)
		if err != nil {
			return err
		}

		om.Set(key, value)
	}

	_, err = dec.ReadToken()

	return err
}
