// Package orderedmap names the ordered package's types under their old spelling.
//
// The type moved to ordered.Map, because orderedmap.OrderedMap stutters. These
// are aliases rather than new types, so a value of one is a value of the other
// and Schema.Properties accepts either name.
package orderedmap

import "github.com/teldio-operations/jsonschema/ordered"

type OrderedMap[K ~string, V any] = ordered.Map[K, V]

type Pair[K ~string, V any] = ordered.Pair[K, V]

func New[K ~string, V any]() *OrderedMap[K, V] {
	return ordered.New[K, V]()
}
