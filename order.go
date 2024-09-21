package main

import "fmt"

var orderFields OrderFields

type OrderFields []OrderField
type OrderField struct {
	Key   string
	Value string
}

func InitializeOrder() {
	fields := map[string]string{
		"created_at": "Created",
		"id":         "Id",
		"title":      "Title",
		"rating":     "Rating",
		"updated_at": "Updated",
		"collection": "Collection",
		"marks":      "Marks",
		"pages":      "Pages",
		"read_count": "Read count",
	}

	orderFields = append(orderFields, OrderField{"", "Score"})

	for key, label := range fields {
		orderFields = append(orderFields, OrderField{key, fmt.Sprintf("%s asc", label)})
		rkey := fmt.Sprintf("-%s", key)
		orderFields = append(orderFields, OrderField{rkey, fmt.Sprintf("%s desc", label)})
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
