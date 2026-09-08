package orderedmap

import (
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"reflect"
	"testing"
)

func keys(t *testing.T, om *OrderedMap[string, int]) []string {
	t.Helper()

	var out []string
	for pair := om.Oldest(); pair != nil; pair = pair.Next() {
		out = append(out, pair.Key)
	}

	return out
}

func filled(t *testing.T, in ...string) *OrderedMap[string, int] {
	t.Helper()

	om := New[string, int]()
	for i, key := range in {
		om.Set(key, i)
	}

	return om
}

func TestSetKeepsInsertionOrder(t *testing.T) {
	om := filled(t, "zebra", "apple", "mango")

	got := keys(t, om)
	want := []string{"zebra", "apple", "mango"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("order = %v, want %v", got, want)
	}
}

func TestSetOnAnExistingKeyKeepsItsPosition(t *testing.T) {
	om := filled(t, "first", "second", "third")

	old, found := om.Set("first", 99)
	if !found {
		t.Error("Set reported the key as new")
	}

	if old != 0 {
		t.Errorf("Set returned %d as the old value, want 0", old)
	}

	got := keys(t, om)
	want := []string{"first", "second", "third"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("order = %v, want %v", got, want)
	}

	value, _ := om.Get("first")
	if value != 99 {
		t.Errorf("Get = %d, want 99", value)
	}
}

func TestDeleteUnlinksThePair(t *testing.T) {
	tests := map[string]struct {
		delete string
		want   []string
	}{
		"head":   {delete: "a", want: []string{"b", "c"}},
		"middle": {delete: "b", want: []string{"a", "c"}},
		"tail":   {delete: "c", want: []string{"a", "b"}},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			om := filled(t, "a", "b", "c")

			_, found := om.Delete(test.delete)
			if !found {
				t.Fatalf("Delete(%q) reported the key as missing", test.delete)
			}

			got := keys(t, om)
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("order = %v, want %v", got, test.want)
			}

			if om.Len() != len(test.want) {
				t.Errorf("Len = %d, want %d", om.Len(), len(test.want))
			}
		})
	}
}

func TestDeletingEveryPairEmptiesTheChain(t *testing.T) {
	om := filled(t, "a", "b")

	om.Delete("a")
	om.Delete("b")

	if om.Oldest() != nil {
		t.Error("Oldest is not nil")
	}

	if om.Newest() != nil {
		t.Error("Newest is not nil")
	}

	om.Set("c", 1)

	got := keys(t, om)
	want := []string{"c"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("order = %v, want %v", got, want)
	}
}

func TestNewestAndPrevWalkBackward(t *testing.T) {
	om := filled(t, "a", "b", "c")

	var got []string
	for pair := om.Newest(); pair != nil; pair = pair.Prev() {
		got = append(got, pair.Key)
	}

	want := []string{"c", "b", "a"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("order = %v, want %v", got, want)
	}
}

// fabric-go reads Len and Get straight off Schema.Properties without a nil
// check, so a schema with no properties has to answer rather than panic. See
// module/queryable.go and the modules that call Properties.Get.
func TestANilMapAnswersInsteadOfPanicking(t *testing.T) {
	var om *OrderedMap[string, int]

	if om.Len() != 0 {
		t.Errorf("Len = %d, want 0", om.Len())
	}

	_, found := om.Get("anything")
	if found {
		t.Error("Get found a key")
	}

	if om.Oldest() != nil {
		t.Error("Oldest is not nil")
	}
}

func TestAllYieldsEveryPairInInsertionOrder(t *testing.T) {
	om := filled(t, "zebra", "apple", "mango")

	var gotKeys []string
	var gotValues []int

	for key, value := range om.All() {
		gotKeys = append(gotKeys, key)
		gotValues = append(gotValues, value)
	}

	wantKeys := []string{"zebra", "apple", "mango"}
	if !reflect.DeepEqual(gotKeys, wantKeys) {
		t.Errorf("keys = %v, want %v", gotKeys, wantKeys)
	}

	wantValues := []int{0, 1, 2}
	if !reflect.DeepEqual(gotValues, wantValues) {
		t.Errorf("values = %v, want %v", gotValues, wantValues)
	}
}

func TestAllStopsOnBreak(t *testing.T) {
	om := filled(t, "a", "b", "c")

	var got []string

	for key := range om.All() {
		got = append(got, key)
		if key == "b" {
			break
		}
	}

	want := []string{"a", "b"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("keys = %v, want %v", got, want)
	}
}

func TestAllOnANilMapYieldsNothing(t *testing.T) {
	var om *OrderedMap[string, int]

	for key := range om.All() {
		t.Errorf("yielded %q", key)
	}
}

// A Go map lets a range body delete the key it is on, and callers filtering a
// schema's properties expect the same here. Delete relinks the neighbors and
// leaves the removed pair pointing at the next one, so the loop keeps its
// place. Clearing those pointers in Delete would strand the loop.
func TestAllToleratesADeleteOfThePairItIsOn(t *testing.T) {
	om := filled(t, "a", "b", "c", "d")

	var got []string

	for key := range om.All() {
		got = append(got, key)

		if key == "b" || key == "c" {
			om.Delete(key)
		}
	}

	want := []string{"a", "b", "c", "d"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("visited %v, want %v", got, want)
	}

	left := keys(t, om)
	wantLeft := []string{"a", "d"}

	if !reflect.DeepEqual(left, wantLeft) {
		t.Errorf("remaining = %v, want %v", left, wantLeft)
	}
}

func TestMarshalKeepsInsertionOrder(t *testing.T) {
	om := filled(t, "zebra", "apple")

	got, err := json.Marshal(om)
	if err != nil {
		t.Fatalf("failed to marshal: %s", err)
	}

	want := `{"zebra":0,"apple":1}`
	if string(got) != want {
		t.Errorf("marshal = %s, want %s", got, want)
	}
}

func TestMarshalOfANilMapIsNull(t *testing.T) {
	var om *OrderedMap[string, int]

	got, err := json.Marshal(om)
	if err != nil {
		t.Fatalf("failed to marshal: %s", err)
	}

	if string(got) != "null" {
		t.Errorf("marshal = %s, want null", got)
	}
}

func TestUnmarshalKeepsDocumentOrder(t *testing.T) {
	var om OrderedMap[string, int]

	err := json.Unmarshal([]byte(`{"zebra":1,"apple":2,"mango":3}`), &om)
	if err != nil {
		t.Fatalf("failed to unmarshal: %s", err)
	}

	got := keys(t, &om)
	want := []string{"zebra", "apple", "mango"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("order = %v, want %v", got, want)
	}

	value, _ := om.Get("apple")
	if value != 2 {
		t.Errorf("Get = %d, want 2", value)
	}
}

func TestUnmarshalOfNullLeavesTheMapEmpty(t *testing.T) {
	var om OrderedMap[string, int]

	err := json.Unmarshal([]byte(`null`), &om)
	if err != nil {
		t.Fatalf("failed to unmarshal: %s", err)
	}

	if om.Len() != 0 {
		t.Errorf("Len = %d, want 0", om.Len())
	}
}

func TestUnmarshalRejectsANonObject(t *testing.T) {
	var om OrderedMap[string, int]

	err := json.Unmarshal([]byte(`["a"]`), &om)
	if err == nil {
		t.Fatal("expected an error")
	}
}

// UnmarshalJSONFrom reads from the caller's decoder, so the caller's options
// decide. encoding/json lets a duplicate member name through and the last one
// wins, and the replaced package was equally lax, so a schema that decoded
// before has to keep decoding rather than start erroring.
func TestUnmarshalTakesTheLastOfADuplicateNameUnderV1(t *testing.T) {
	var om OrderedMap[string, int]

	err := json.Unmarshal([]byte(`{"a":1,"b":2,"a":3}`), &om)
	if err != nil {
		t.Fatalf("failed to unmarshal: %s", err)
	}

	value, _ := om.Get("a")
	if value != 3 {
		t.Errorf("Get = %d, want 3", value)
	}

	got := keys(t, &om)
	want := []string{"a", "b"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("order = %v, want %v", got, want)
	}
}

// The reason this type implements MarshalerTo rather than MarshalJSON. Quoting
// the key with v1 json.Marshal escaped HTML no matter what the caller asked
// for. Writing it to the caller's encoder lets the caller decide.
func TestMarshalEscapesAKeyTheWayTheCallerAsked(t *testing.T) {
	om := New[string, int]()
	om.Set("a<b>&c", 1)

	v1, err := json.Marshal(om)
	if err != nil {
		t.Fatalf("failed to marshal with v1: %s", err)
	}

	want := `{"a\u003cb\u003e\u0026c":1}`
	if string(v1) != want {
		t.Errorf("v1 = %s, want %s", v1, want)
	}

	v2, err := jsonv2.Marshal(om)
	if err != nil {
		t.Fatalf("failed to marshal with v2: %s", err)
	}

	want = `{"a<b>&c":1}`
	if string(v2) != want {
		t.Errorf("v2 = %s, want %s", v2, want)
	}
}

// The other half of reading from the caller's decoder. json/v2 rejects a
// duplicate member name by default, and that strictness has to reach through
// this type rather than stop at it.
func TestUnmarshalRejectsADuplicateNameUnderV2(t *testing.T) {
	var om OrderedMap[string, int]

	err := jsonv2.Unmarshal([]byte(`{"a":1,"a":2}`), &om)
	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestARoundTripKeepsTheDocument(t *testing.T) {
	in := []byte(`{"zebra":1,"apple":2,"mango":3}`)

	var om OrderedMap[string, int]

	err := json.Unmarshal(in, &om)
	if err != nil {
		t.Fatalf("failed to unmarshal: %s", err)
	}

	out, err := json.Marshal(&om)
	if err != nil {
		t.Fatalf("failed to marshal: %s", err)
	}

	if string(out) != string(in) {
		t.Errorf("round trip = %s, want %s", out, in)
	}
}
