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
	"bytes"
	"encoding/json"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"fmt"
	"iter"
)

var (
	_ json.Marshaler   = &OrderedMap[string, any]{}
	_ json.Unmarshaler = &OrderedMap[string, any]{}
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

func (om *OrderedMap[K, V]) MarshalJSON() ([]byte, error) {
	if om == nil || om.pairs == nil {
		return []byte("null"), nil
	}

	buf := []byte{'{'}

	for pair := om.Oldest(); pair != nil; pair = pair.Next() {
		if len(buf) > 1 {
			buf = append(buf, ',')
		}

		key, err := json.Marshal(string(pair.Key))
		if err != nil {
			return nil, err
		}

		buf = append(buf, key...)
		buf = append(buf, ':')

		value, err := json.Marshal(pair.Value)
		if err != nil {
			return nil, err
		}

		buf = append(buf, value...)
	}

	return append(buf, '}'), nil
}

func (om *OrderedMap[K, V]) UnmarshalJSON(data []byte) error {
	// The v1 options let a duplicate member name and invalid UTF-8 through, the
	// way encoding/json does. The replaced package was equally lax, and a schema
	// that decoded before this change has to keep decoding.
	dec := jsontext.NewDecoder(bytes.NewReader(data), json.DefaultOptionsV1())

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

		err = jsonv2.UnmarshalDecode(dec, &value, json.DefaultOptionsV1())
		if err != nil {
			return err
		}

		om.Set(key, value)
	}

	_, err = dec.ReadToken()

	return err
}
