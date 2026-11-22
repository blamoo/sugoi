package web

import "testing"

func TestOrderFields(t *testing.T) {
	// fixed order: it used to come from map iteration and change on every start
	wantKeys := []string{"",
		"created_at", "-created_at", "updated_at", "-updated_at", "title", "-title",
		"rating", "-rating", "marks", "-marks", "read_count", "-read_count",
		"pages", "-pages", "collection", "-collection", "id", "-id"}
	if len(orderFields) != len(wantKeys) {
		t.Fatalf("%d options, want %d", len(orderFields), len(wantKeys))
	}
	for i, f := range orderFields {
		if f.Key != wantKeys[i] {
			t.Errorf("option %d = %q, want %q", i, f.Key, wantKeys[i])
		}
		if n := len(f.Sort); n < 2 || f.Sort[n-1] != "_id" {
			t.Errorf("%q sorts by %v, want a final _id tie-break", f.Key, f.Sort)
		}
	}

	for key, want := range map[string]string{"title": "title_sort", "-title": "-title_sort", "collection": "collection_sort", "-rating": "-rating"} {
		if f, ok := orderFields.Find(key); !ok || f.Sort[0] != want {
			t.Errorf("%q sorts by %v, want %s first", key, f.Sort, want)
		}
	}
	if _, ok := orderFields.Find("bogus"); ok {
		t.Error("found an unknown order")
	}
}
