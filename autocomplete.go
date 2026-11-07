package main

import (
	"encoding/json"
	"net/http"
)

func RouteAutocomplete(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if _, ret := CheckAnyAuth(w, r); ret {
		w.WriteHeader(403)
		json.NewEncoder(w).Encode(JsonResponse{"Unauthorized"})
		return
	}

	json.NewEncoder(w).Encode(cat.Autocomplete.Suggest(r.FormValue("q"), !cat.Reindex.Running()))
}
