package web

import (
	"encoding/json"
	"net/http"
)

func (srv *Server) routeAutocomplete(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if _, ret := srv.checkAnyAuth(w, r); ret {
		w.WriteHeader(403)
		json.NewEncoder(w).Encode(JsonResponse{"Unauthorized"})
		return
	}

	json.NewEncoder(w).Encode(srv.cat.Autocomplete.Suggest(r.FormValue("q"), !srv.cat.Reindex.Running()))
}
