package orderedmap_test

import (
	"testing"

	"github.com/teldio-operations/jsonschema"
	"github.com/teldio-operations/jsonschema/ordered"
	"github.com/teldio-operations/jsonschema/orderedmap"
)

func takesTheOldMap(*orderedmap.OrderedMap[string, *jsonschema.Schema]) {}

func takesTheNewMap(*ordered.Map[string, *jsonschema.Schema]) {}

func takesTheOldPair(*orderedmap.Pair[string, *jsonschema.Schema]) {}

// OrderedMap and Pair have to be aliases rather than defined types. As defined
// types they still compile inside this package, but they stop satisfying
// Schema.Properties, which is what a caller assigns them to. The calls below
// are what would fail.
func TestTheOldNamesAreTheSameTypes(t *testing.T) {
	var schema jsonschema.Schema
	schema.Properties = orderedmap.New[string, *jsonschema.Schema]()

	takesTheNewMap(schema.Properties)
	takesTheOldMap(ordered.New[string, *jsonschema.Schema]())

	schema.Properties.Set("only", jsonschema.TrueSchema)
	takesTheOldPair(schema.Properties.Oldest())

	if schema.Properties.Len() != 1 {
		t.Errorf("Len = %d, want 1", schema.Properties.Len())
	}
}
