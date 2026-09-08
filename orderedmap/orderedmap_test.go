package orderedmap

import (
	"encoding/json"
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

func TestMarshalJSONKeepsInsertionOrder(t *testing.T) {
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

func TestMarshalJSONOfAZeroMapIsNull(t *testing.T) {
	var om *OrderedMap[string, int]

	got, err := json.Marshal(om)
	if err != nil {
		t.Fatalf("failed to marshal: %s", err)
	}

	if string(got) != "null" {
		t.Errorf("marshal = %s, want null", got)
	}
}

func TestUnmarshalJSONKeepsDocumentOrder(t *testing.T) {
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

func TestUnmarshalJSONOfNullLeavesTheMapEmpty(t *testing.T) {
	var om OrderedMap[string, int]

	err := json.Unmarshal([]byte(`null`), &om)
	if err != nil {
		t.Fatalf("failed to unmarshal: %s", err)
	}

	if om.Len() != 0 {
		t.Errorf("Len = %d, want 0", om.Len())
	}
}

func TestUnmarshalJSONRejectsANonObject(t *testing.T) {
	var om OrderedMap[string, int]

	err := json.Unmarshal([]byte(`["a"]`), &om)
	if err == nil {
		t.Fatal("expected an error")
	}
}

// encoding/json lets a duplicate member name through and the last one wins. The
// replaced package was equally lax, so a schema that decoded before has to keep
// decoding rather than start erroring.
func TestUnmarshalJSONTakesTheLastOfADuplicateName(t *testing.T) {
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
