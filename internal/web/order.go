package web

// OrderField is one option of the home page's order select. Key is what goes
// in ?order= (kept stable for bookmarks and search history), Value the label,
// and Sort what Bleve sorts by.
type OrderField struct {
	Key   string
	Value string
	Sort  []string
}

type OrderFields []OrderField

// orderFields lists the options in the order the select shows them. Every
// sort ends with _id so items with equal values (the same rating, no marks)
// keep a stable order across pages. Text fields sort by their lowercased
// copies (search.SortKeys), not the analyzed fields.
var orderFields = OrderFields{
	{"", "Score", []string{"-_score", "_id"}},
}

func init() {
	fields := []struct {
		key, label, field string
	}{
		{"created_at", "Created", "created_at"},
		{"updated_at", "Updated", "updated_at"},
		{"title", "Title", "title_sort"},
		{"rating", "Rating", "rating"},
		{"marks", "Marks", "marks"},
		{"read_count", "Read count", "read_count"},
		{"pages", "Pages", "pages"},
		{"collection", "Collection", "collection_sort"},
		{"id", "Id", "id"},
	}
	for _, f := range fields {
		orderFields = append(orderFields,
			OrderField{f.key, f.label + " asc", []string{f.field, "_id"}},
			OrderField{"-" + f.key, f.label + " desc", []string{"-" + f.field, "_id"}},
		)
	}
}

func (of OrderFields) Find(key string) (OrderField, bool) {
	for _, v := range of {
		if v.Key == key {
			return v, true
		}
	}
	return OrderField{}, false
}
