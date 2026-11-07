package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	_ "image/png"
	"log/slog"

	"io"
	"math"
	"math/rand/v2"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "golang.org/x/image/webp"

	"github.com/bamiaux/rez"
	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search/query"
	"github.com/gorilla/mux"

	"github.com/blamoo/sugoi/internal/fsutil"

	"github.com/blamoo/sugoi/internal/search"

	"github.com/blamoo/sugoi/internal/catalog"

	"github.com/blamoo/sugoi/internal/config"
	"github.com/blamoo/sugoi/internal/metadata"
)

func (srv *Server) routes(router *mux.Router) {
	router.HandleFunc("/files.json", srv.routeFilesJson)
	router.HandleFunc("/allFiles.json", srv.routeAllFilesJson)
	router.HandleFunc("/", srv.routeRoot)
	router.HandleFunc("/login", srv.routeLogin)
	router.HandleFunc("/logout", srv.routeLogout)
	router.HandleFunc("/thing/details/{hash:[a-z0-9]+}.json", srv.routeThingDetailsJson)
	router.HandleFunc("/thing/details/{hash:[a-z0-9]+}", srv.routeThingDetails)
	router.HandleFunc("/thing/editMetadata/{hash:[a-z0-9]+}", srv.routeThingEditMetadata)
	router.HandleFunc("/thing/saveMetadata/{hash:[a-z0-9]+}", srv.routeThingSaveMetadata)
	router.HandleFunc("/thing/{hash:[a-z0-9]+}/rating.json", srv.routeThingRatingJson)
	router.HandleFunc("/thing/{hash:[a-z0-9]+}/cover.json", srv.routeThingCoverJson)
	router.HandleFunc("/thing/{hash:[a-z0-9]+}/addMark.json", srv.routeThingAddMark)
	router.HandleFunc("/thing/{hash:[a-z0-9]+}/subMark.json", srv.routeThingSubMark)
	router.HandleFunc("/thing/read/{hash:[a-z0-9]+}{page:/?[0-9]*}", srv.routeThingRead)
	router.HandleFunc("/thing/file/{hash:[a-z0-9]+}/{file:.+}", srv.routeThingFile)
	router.HandleFunc("/thing/pushRead/{hash:[a-z0-9]+}", srv.routeThingPushRead)
	router.HandleFunc("/system", srv.routeSystem)
	router.HandleFunc("/system/reindexStatus", srv.routeSystemReindexStatus)
	router.HandleFunc("/favicon.ico", srv.routeFavicon)
	router.HandleFunc("/autocomplete.json", srv.routeAutocomplete)
}

func (srv *Server) routeRoot(w http.ResponseWriter, r *http.Request) {
	if _, ret := srv.handleAuth(w, r); ret {
		return
	}

	data := struct {
		Title          string
		Things         []*Thing
		HasNext        bool
		HasPrev        bool
		Page           int
		PageNextUrl    string
		PagePrevUrl    string
		Format         string
		OrderAvailable OrderFields
		Order          string
		Search         string
		Hits           uint64
		SearchTags     []SearchTerm
		QuickFilters   []QuickFilter
	}{
		Title:          "Home",
		HasNext:        false,
		HasPrev:        false,
		OrderAvailable: orderFields,
	}
	titleBuilder := strings.Builder{}

	data.Page, _ = strconv.Atoi(r.FormValue("page"))

	if data.Page < 0 {
		data.Page = 0
	}

	data.Order = r.FormValue("order")

	if _, ok := orderFields.Find(data.Order); !ok {
		data.Order = "CreatedAt"
	}
	data.Search = r.FormValue("q")

	fFormat := r.FormValue("format")
	var pageSize int
	switch fFormat {
	case "table":
		data.Format = "table"
		pageSize = 50

	case "full":
		data.Format = "full"
		pageSize = 50

	default:
		data.Format = "covers"
		pageSize = 48
	}

	fQ := strings.TrimSpace(r.FormValue("q"))
	fDebug := r.FormValue("debug")

	var query query.Query
	if len(fQ) == 0 {
		titleBuilder.WriteString("Home")
		query = bleve.NewMatchAllQuery()
	} else {
		rawquery := bleve.NewQueryStringQuery(fQ)
		parsed, err := rawquery.Parse()

		if err != nil {
			srv.renderError(w, r, err.Error())
			return
		}

		parsed = search.ShouldToMust(parsed)

		r := strings.NewReplacer(
			"+", "",
			"\"", "",
		)
		titleBuilder.WriteString(r.Replace(fQ))

		query = parsed
	}

	if data.Page >= 1 {
		titleBuilder.WriteString(" - Page ")
		titleBuilder.WriteString(strconv.Itoa(data.Page))
	}

	data.Title = titleBuilder.String()

	searchReq := bleve.NewSearchRequest(query)

	if ok, _ := strconv.ParseBool(r.FormValue("random")); ok {
		sort := "random." + strconv.Itoa(rand.IntN(search.RandomPoolSize))
		searchReq.SortBy([]string{sort})
		searchReq.Size = randomSampleSize

		searchResults, err := srv.cat.Index.Search(searchReq)
		if err != nil {
			srv.renderError(w, r, err.Error())
			return
		}
		hits := len(searchResults.Hits)
		if hits != 0 {
			pick := rand.IntN(hits)
			hash := searchResults.Hits[pick].ID
			thing, err := srv.thing(hash)
			if err != nil {
				srv.renderError(w, r, err.Error())
				return
			}

			if ok, _ := strconv.ParseBool(r.FormValue("refresh")); ok {
				readerData, err := NewReaderData(thing, 0)
				if err != nil {
					srv.renderError(w, r, err.Error())
					return
				}

				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(readerData)
				return
			}

			http.Redirect(w, r, thing.ReadUrl(), http.StatusFound)
			return
		}
	}

	searchReq.Size = pageSize + 1
	searchReq.From = data.Page * pageSize
	if fDebug == "raw" {
		searchReq.Fields = []string{"*"}
	} else {
		searchReq.Fields = []string{"ID"}
	}

	if data.Order != "" {
		searchReq.SortBy([]string{data.Order})
	}

	if fDebug == "query" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(searchReq)
		return
	}

	for _, field := range search.KeywordFields {
		searchReq.AddFacet(field, bleve.NewFacetRequest(field+"_kw", 50))
	}

	searchResults, err := srv.cat.Index.Search(searchReq)
	if err != nil {
		srv.renderError(w, r, err.Error())
		return
	}
	data.Hits = searchResults.Total

	for _, facet := range searchResults.Facets {
		for _, term := range facet.Terms.Terms() {
			if len(term.Term) == 0 {
				continue
			}

			name, _ := strings.CutSuffix(facet.Field, "_kw")
			searchTerm := NewSearchTerm(name, term.Term)
			searchTerm.Count = term.Count
			searchTerm.Weight = slices.Index(search.KeywordFields, name)
			data.SearchTags = append(data.SearchTags, searchTerm)
		}
	}

	sort.SliceStable(data.SearchTags, func(i, j int) bool {
		return data.SearchTags[i].Count > data.SearchTags[j].Count
	})

	if len(data.SearchTags) > 100 {
		data.SearchTags = data.SearchTags[:100]
	}

	sort.SliceStable(data.SearchTags, func(i, j int) bool {
		// if data.SearchTags[i].Weight != data.SearchTags[j].Weight {
		// 	return data.SearchTags[i].Weight < data.SearchTags[j].Weight
		// }

		if data.SearchTags[i].Count != data.SearchTags[j].Count {
			return data.SearchTags[i].Count > data.SearchTags[j].Count
		}

		return data.SearchTags[i].Label > data.SearchTags[j].Label
	})

	if fDebug == "raw" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(searchResults)
		return
	}

	for _, v := range searchResults.Hits {
		thing, err := srv.thing(v.ID)
		if err != nil {
			srv.renderError(w, r, err.Error())
			return
		}
		data.Things = append(data.Things, thing)
	}

	if len(data.Things) > pageSize {
		data.HasNext = true
		data.Things = data.Things[:pageSize]
		q := r.URL.Query()
		q.Set("page", strconv.Itoa(data.Page+1))
		u := url.URL{Path: r.URL.Path, RawQuery: q.Encode()}
		data.PageNextUrl = u.String()
	}

	if data.Page > 0 {
		data.HasPrev = true
		q := r.URL.Query()
		q.Set("page", strconv.Itoa(data.Page-1))
		u := url.URL{Path: r.URL.Path, RawQuery: q.Encode()}
		data.PagePrevUrl = u.String()
	}

	data.QuickFilters = srv.quickFilters(r.Form)

	if fDebug == "json" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(data)
		return
	}

	srv.renderPage(w, r, "index.gohtml", data)
}

func (srv *Server) routeFilesJson(w http.ResponseWriter, r *http.Request) {
	if _, ret := srv.handleAuth(w, r); ret {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(srv.cat.Library().List)
}

func (srv *Server) routeAllFilesJson(w http.ResponseWriter, r *http.Request) {
	if _, ret := srv.handleAuth(w, r); ret {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(srv.cat.Library().ByHash)
}

func (srv *Server) routeLogin(w http.ResponseWriter, r *http.Request) {
	fUser := r.FormValue("user")

	data := struct {
		Title string
		Error string
		User  string
	}{
		Title: "Login",
		User:  fUser,
	}

	fPassword := r.FormValue("password")

	if r.Method == http.MethodPost {
		found := false
		foundUser := ""
		foundPass := ""
		userLowercase := strings.ToLower(fUser)

		for k, v := range srv.cfg.Users {
			if userLowercase == strings.ToLower(k) {
				foundUser = k
				foundPass = v
				found = found || true
			}
		}

		if !found {
			data.Error = "User not found"
			srv.renderPage(w, r, "login.gohtml", data)
			return
		}

		if !CheckPasswordHash(fPassword, foundPass) {
			data.Error = "Wrong or empty password"
			srv.renderPage(w, r, "login.gohtml", data)
			return
		}

		session, _ := srv.sessions.Get(r, srv.cfg.SessionCookieName)
		session.Values["authenticated"] = true
		session.Values["user"] = foundUser
		err := session.Save(r, w)

		if err != nil {
			slog.Error("saving session", "err", err)
		}

		returnPage := r.FormValue("return")
		if len(returnPage) > 0 && returnPage[0] == '/' {
			http.Redirect(w, r, returnPage, http.StatusFound)
			return
		}

		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	srv.renderPage(w, r, "login.gohtml", data)
}

func (srv *Server) routeLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		session, _ := srv.sessions.Get(r, srv.cfg.SessionCookieName)
		session.Values["authenticated"] = false
		session.Values["user"] = nil
		err := session.Save(r, w)
		if err != nil {
			slog.Error("saving session", "err", err)
		}
	}

	http.Redirect(w, r, "/login", http.StatusFound)
}

func (srv *Server) routeThingDetailsJson(w http.ResponseWriter, r *http.Request) {
	if _, ret := srv.handleAuth(w, r); ret {
		return
	}

	vars := mux.Vars(r)
	vHash := vars["hash"]

	thing, err := srv.thing(vHash)
	if err != nil {
		srv.renderError(w, r, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(thing)
}

func (srv *Server) routeThingDetails(w http.ResponseWriter, r *http.Request) {
	if _, ret := srv.handleAuth(w, r); ret {
		return
	}

	vars := mux.Vars(r)
	vHash := vars["hash"]

	thing, err := srv.thing(vHash)
	if err != nil {
		srv.renderError(w, r, err.Error())
		return
	}

	data := struct {
		Title    string
		Thing    *Thing
		FilesRaw []string
	}{
		Title: thing.Title,
		Thing: thing,
	}

	data.FilesRaw, _ = thing.Pages()
	srv.renderPage(w, r, "thingDetails.gohtml", data)
}

func (srv *Server) routeThingEditMetadata(w http.ResponseWriter, r *http.Request) {
	var err error
	if _, ret := srv.handleAuth(w, r); ret {
		return
	}

	vars := mux.Vars(r)
	vHash := vars["hash"]

	thing, err := srv.thing(vHash)
	if err != nil {
		srv.renderError(w, r, err.Error())
		return
	}

	format := r.FormValue("format")

	staticMeta := thing.Static

	switch format {
	case "edit":
		staticMeta = thing.Static

	default:
		srv.renderError(w, r, "Invalid or unknown format")
		return
	}

	data := struct {
		Title    string
		Thing    *Thing
		Metadata metadata.Static
	}{
		Title:    thing.Title,
		Thing:    thing,
		Metadata: staticMeta,
	}

	srv.renderPage(w, r, "thingEditMetadata.gohtml", data)
}

func (srv *Server) routeThingSaveMetadata(w http.ResponseWriter, r *http.Request) {
	var err error
	if _, ret := srv.handleAuth(w, r); ret {
		return
	}

	if r.Method != http.MethodPost {
		w.WriteHeader(405)
		srv.renderError(w, r, "method not allowed")
		return
	}

	vars := mux.Vars(r)
	vHash := vars["hash"]

	thing, err := srv.thing(vHash)
	if err != nil {
		srv.renderError(w, r, err.Error())
		return
	}

	thing.CoverImageUrl()
	r.ParseForm()

	err = thing.SaveStaticFromForm(r.PostForm)
	if err != nil {
		srv.renderError(w, r, err.Error())
		return
	}
	http.Redirect(w, r, thing.DetailsUrl(), http.StatusFound)
}

func (srv *Server) routeThingRatingJson(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	_, ret := srv.checkAnyAuth(w, r)
	if ret {
		w.WriteHeader(403)
		json.NewEncoder(w).Encode(JsonResponse{"Unauthorized"})
		return
	}

	vars := mux.Vars(r)
	vHash := vars["hash"]

	thing, err := srv.thing(vHash)
	if err != nil {
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(JsonError{err.Error()})
		return
	}

	if r.Method == http.MethodPost {
		fRate, err := strconv.Atoi(r.FormValue("rate"))
		fToggle, _ := strconv.ParseBool(r.FormValue("toggle"))

		if err != nil {
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(JsonError{err.Error()})
			return
		}

		if fRate > 5 {
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(JsonResponse{"Rating cannot be > 5"})
			return
		}

		if fRate < 0 {
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(JsonResponse{"Rating cannot be < 0"})
			return
		}

		err = thing.TrySaveRating(fRate, fToggle)

		if err != nil {
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(JsonError{err.Error()})
			return
		}

		type Response struct {
			JsonResponse
			Rating int
		}
		var response Response
		response.Rating = thing.Rating

		response.Message = fmt.Sprintf("Rating updated to %d", thing.Rating)

		json.NewEncoder(w).Encode(response)
		return
	}

	if r.Method == http.MethodGet {
		json.NewEncoder(w).Encode(struct{ Rating int }{thing.Rating})
		return
	}

	w.WriteHeader(405)
	json.NewEncoder(w).Encode(JsonResponse{"method not allowed"})
}

func (srv *Server) routeThingCoverJson(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	_, ret := srv.checkAnyAuth(w, r)
	if ret {
		w.WriteHeader(403)
		json.NewEncoder(w).Encode(JsonResponse{"Unauthorized"})
		return
	}

	vars := mux.Vars(r)
	vHash := vars["hash"]

	thing, err := srv.thing(vHash)
	if err != nil {
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(JsonError{err.Error()})
		return
	}

	if r.Method == http.MethodPost {
		fFile := r.FormValue("file")
		fFile, err = url.PathUnescape(fFile)

		if err != nil {
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(JsonError{err.Error()})
			return
		}

		changed := (thing.Cover != fFile)
		if changed {
			err := thing.TrySaveCover(fFile, false)
			if err != nil {
				w.WriteHeader(400)
				json.NewEncoder(w).Encode(JsonError{err.Error()})
				return
			}
		}

		type Response struct {
			JsonResponse
			Cover string
		}
		var response Response
		response.Cover = thing.Cover

		if changed {
			response.Message = fmt.Sprintf("Cover updated to %s", thing.Cover)
		} else {
			response.Message = "This is the cover already"
		}

		json.NewEncoder(w).Encode(response)
		return
	}

	if r.Method == http.MethodGet {
		json.NewEncoder(w).Encode(struct{ Cover string }{thing.Cover})
		return
	}

	w.WriteHeader(405)
	json.NewEncoder(w).Encode(JsonResponse{"method not allowed"})
}

func (srv *Server) routeThingAddMark(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	_, ret := srv.checkAnyAuth(w, r)
	if ret {
		w.WriteHeader(403)
		json.NewEncoder(w).Encode(JsonResponse{"Unauthorized"})
		return
	}

	vars := mux.Vars(r)
	vHash := vars["hash"]

	thing, err := srv.thing(vHash)
	if err != nil {
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(JsonError{err.Error()})
		return
	}

	if r.Method == http.MethodPost {
		err = thing.AddMark()

		if err != nil {
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(JsonError{err.Error()})
			return
		}

		type Response struct {
			JsonResponse
			Marks  int
			Rating int
		}
		var response Response
		response.Rating = thing.Rating
		response.Marks = thing.Marks
		response.Message = fmt.Sprintf("Marks: %d", thing.Marks)

		json.NewEncoder(w).Encode(response)
		return
	}

	if r.Method == http.MethodGet {
		json.NewEncoder(w).Encode(struct{ Rating int }{thing.Rating})
		return
	}

	w.WriteHeader(405)
	json.NewEncoder(w).Encode(JsonResponse{"method not allowed"})
}

func (srv *Server) routeThingSubMark(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	_, ret := srv.checkAnyAuth(w, r)
	if ret {
		w.WriteHeader(403)
		json.NewEncoder(w).Encode(JsonResponse{"Unauthorized"})
		return
	}

	vars := mux.Vars(r)
	vHash := vars["hash"]

	thing, err := srv.thing(vHash)
	if err != nil {
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(JsonError{err.Error()})
		return
	}

	if r.Method == http.MethodPost {
		err = thing.SubMark()

		if err != nil {
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(JsonError{err.Error()})
			return
		}

		type Response struct {
			JsonResponse
			Marks  int
			Rating int
		}
		var response Response
		response.Rating = thing.Rating
		response.Marks = thing.Marks
		response.Message = fmt.Sprintf("Marks: %d", thing.Marks)

		json.NewEncoder(w).Encode(response)
		return
	}

	if r.Method == http.MethodGet {
		json.NewEncoder(w).Encode(struct{ Rating int }{thing.Rating})
		return
	}

	w.WriteHeader(405)
	json.NewEncoder(w).Encode(JsonResponse{"method not allowed"})
}

func (srv *Server) routeThingRead(w http.ResponseWriter, r *http.Request) {
	if _, ret := srv.handleAuth(w, r); ret {
		return
	}

	vars := mux.Vars(r)
	vHash := vars["hash"]
	vPage, _ := strconv.Atoi(strings.TrimLeft(vars["page"], "/"))

	thing, err := srv.thing(vHash)
	if err != nil {
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(JsonError{err.Error()})
		return
	}

	readerData, err := NewReaderData(thing, vPage)
	if err != nil {
		srv.renderError(w, r, err.Error())
		return
	}

	if ok, _ := strconv.ParseBool(r.FormValue("json")); ok {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(readerData)
		return
	}

	data := struct {
		Config     config.Config
		Title      string
		ReaderData ReaderData
	}{
		Title:      thing.Title,
		Config:     *srv.cfg,
		ReaderData: readerData,
	}

	srv.renderPage(w, r, "thingRead.gohtml", data)
}

func (srv *Server) routeThingPushRead(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	_, ret := srv.checkAnyAuth(w, r)
	if ret {
		w.WriteHeader(403)
		json.NewEncoder(w).Encode(JsonResponse{"Unauthorized"})
		return
	}

	vars := mux.Vars(r)
	vHash := vars["hash"]

	thing, err := srv.thing(vHash)
	if err != nil {
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(JsonError{err.Error()})
		return
	}

	if r.Method == http.MethodPost {
		err = thing.PushRead()

		if err != nil {
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(JsonError{err.Error()})
			return
		}

		json.NewEncoder(w).Encode(thing.ReadCount)
		return
	}

	w.WriteHeader(405)
	json.NewEncoder(w).Encode(JsonResponse{"method not allowed"})
}

func (srv *Server) routeThingFile(w http.ResponseWriter, r *http.Request) {
	if _, ret := srv.handleAuth(w, r); ret {
		return
	}

	vars := mux.Vars(r)
	vHash := vars["hash"]
	vFile := vars["file"]
	fSize := r.FormValue("size")

	if fSize == "thumb" {
		if srv.renderThumbCache(w, r, vHash, vFile) {
			return
		}
	}

	thing, err := srv.thing(vHash)
	if err != nil {
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(JsonError{err.Error()})
		return
	}

	reader, closers, err := thing.Open(vFile)
	if err != nil {
		srv.renderError(w, r, err.Error())
		return
	}

	defer fsutil.MultiClose(closers)

	wantedMime := mime.TypeByExtension(filepath.Ext(vFile))

	if fSize == "thumb" {
		original, _, err := image.Decode(reader)
		if err != nil {
			srv.renderError(w, r, err.Error())
			return
		}

		oriRect := original.Bounds()
		factor := float64(oriRect.Max.X) / 256.0
		dstHeight := float64(oriRect.Max.Y) / factor

		dstRect := image.Rect(0, 0, 256, int(math.Ceil(dstHeight)))

		in := image.NewRGBA(oriRect)
		draw.Draw(in, oriRect, original, oriRect.Min, draw.Src)
		original = nil

		out := image.NewRGBA(dstRect)
		err = rez.Convert(out, in, rez.NewBicubicFilter())
		if err != nil {
			srv.renderError(w, r, err.Error())
			return
		}
		in = nil

		w.Header().Set("Content-Type", "image/jpeg")
		SetCacheHeader(w, r, 31536000)

		jpegOptions := jpeg.Options{
			Quality: 60,
		}

		// Encode once; the same bytes go to the cache and the response.
		var encoded bytes.Buffer
		err = jpeg.Encode(&encoded, out, &jpegOptions)
		out = nil
		if err != nil {
			srv.renderError(w, r, err.Error())
			return
		}

		if srv.cfg.CacheThumbnails {
			srv.thumbCacheWrite(vHash, vFile, encoded.Bytes())
		}

		w.Write(encoded.Bytes())
	} else {
		w.Header().Set("Content-Type", wantedMime)
		SetCacheHeader(w, r, 31536000)
		_, err = io.Copy(w, reader)
		if err != nil {
			srv.renderError(w, r, err.Error())
			return
		}
	}
}

func (srv *Server) routeSystem(w http.ResponseWriter, r *http.Request) {
	if _, ret := srv.handleAuth(w, r); ret {
		return
	}

	var err error
	r.ParseForm()

	if r.Method == http.MethodPost {
		fAction := r.FormValue("action")

		switch fAction {
		case "reindex":
			w.Header().Set("Content-Type", "application/json")
			if srv.cat.Reindex.Running() {
				json.NewEncoder(w).Encode(srv.cat.Reindex.Status())
				return
			}

			err = srv.cat.Reindex.Start()
			if err != nil {
				json.NewEncoder(w).Encode(JsonError{err.Error()})
				return
			}

			http.Redirect(w, r, "/system", http.StatusFound)
			// http.Redirect(w, r, "/system?action=reindexStatus", http.StatusFound)
			return

		case "cancelReindex":
			srv.cat.Reindex.Cancel()

			http.Redirect(w, r, "/system?action=reindexStatus", http.StatusFound)
			return

		case "reload":
			w.Header().Set("Content-Type", "application/json")
			prev, count, err := srv.cat.Reload()
			if err != nil {
				w.WriteHeader(500)
				json.NewEncoder(w).Encode(JsonError{err.Error()})
				return
			}

			json.NewEncoder(w).Encode(JsonResponse{fmt.Sprintf("%d files found (previously %d files)", count, prev)})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(JsonError{fmt.Sprintf("Unknown operation '%s'", fAction)})
		return
	}

	fAction := r.FormValue("action")

	switch fAction {
	case "reindexStatus":
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(srv.cat.Reindex.Status())
		return
	}

	data := struct {
		Title   string
		Reindex catalog.ReindexStatus
	}{
		Title:   "System",
		Reindex: srv.cat.Reindex.Status(),
	}

	srv.renderPage(w, r, "system.gohtml", data)
}

func (srv *Server) routeSystemReindexStatus(w http.ResponseWriter, r *http.Request) {
	_, notLoggedIn := srv.checkAnyAuth(w, r)

	data := struct {
		Stop    bool   `json:"Stop,omitempty"`
		Message string `json:"Message,omitempty"`
	}{}

	status := srv.cat.Reindex.Status()
	if notLoggedIn || len(status.Log) == 0 {
		data.Stop = true
	} else {
		dtLimit := status.FinishTime.Add(time.Second * 30)

		if !status.Running && time.Now().After(dtLimit) {
			data.Stop = true
		} else {
			data.Message = status.Log[len(status.Log)-1]
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func (srv *Server) routeFavicon(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "image/x-icon")
	http.ServeFile(w, r, filepath.Join(srv.assetsDir, "static", "favicon.ico"))
}
