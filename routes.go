package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"io"
	"log"
	"math"
	"math/rand/v2"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/bamiaux/rez"
	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search/query"
	"github.com/gorilla/mux"
)

func Routes(router *mux.Router) {
	router.HandleFunc("/files.json", RouteFilesJson)
	router.HandleFunc("/allFiles.json", RouteAllFilesJson)
	router.HandleFunc("/", RouteRoot)
	router.HandleFunc("/login", RouteLogin)
	router.HandleFunc("/logout", RouteLogout)
	router.HandleFunc("/thing/details/{hash:[a-z0-9]+}.json", RouteThingDetailsJson)
	router.HandleFunc("/thing/details/{hash:[a-z0-9]+}", RouteThingDetails)
	router.HandleFunc("/hentag.json", RouteHentagJson)
	router.HandleFunc("/thing/searchMetadata/{hash:[a-z0-9]+}", RouteThingSearchMetadata)
	router.HandleFunc("/thing/editMetadata/{hash:[a-z0-9]+}", RouteThingEditMetadata)
	router.HandleFunc("/thing/saveMetadata/{hash:[a-z0-9]+}", RouteThingSaveMetadata)
	router.HandleFunc("/thing/{hash:[a-z0-9]+}/rating.json", RouteThingRatingJson)
	router.HandleFunc("/thing/{hash:[a-z0-9]+}/cover.json", RouteThingCoverJson)
	router.HandleFunc("/thing/{hash:[a-z0-9]+}/addMark.json", RouteThingAddMark)
	router.HandleFunc("/thing/{hash:[a-z0-9]+}/subMark.json", RouteThingSubMark)
	router.HandleFunc("/thing/read/{hash:[a-z0-9]+}{page:/?[0-9]*}", RouteThingRead)
	router.HandleFunc("/thing/file/{hash:[a-z0-9]+}/{file:.+}", RouteThingFile)
	router.HandleFunc("/thing/pushRead/{hash:[a-z0-9]+}", RouteThingPushRead)
	router.HandleFunc("/system", RouteSystem)
	router.HandleFunc("/system/reindexStatus", RouteSystemReindexStatus)
	router.HandleFunc("/pending", RoutePending)
	router.HandleFunc("/favicon.ico", RouteFavicon)
}

func RouteRoot(w http.ResponseWriter, r *http.Request) {
	if _, ret := HandleAuth(w, r); ret {
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

	// case "covers":
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
			RenderError(w, r, err.Error())
			return
		}

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

	search := bleve.NewSearchRequest(query)

	if ok, _ := strconv.ParseBool(r.FormValue("random")); ok {
		sort := "random." + strconv.Itoa(rand.IntN(RANDOM_POOL_SIZE))
		search.SortBy([]string{sort})
		search.Size = RANDOM_SAMPLE_SIZE

		searchResults, err := bleveIndex.Search(search)
		if err != nil {
			RenderError(w, r, err.Error())
			return
		}
		hits := len(searchResults.Hits)
		pick := rand.IntN(hits)
		hash := searchResults.Hits[pick].ID
		thing, err := NewThingFromHash(hash)
		if err != nil {
			RenderError(w, r, err.Error())
			return
		}

		http.Redirect(w, r, thing.ReadUrl(), http.StatusFound)
		return
	}

	search.Size = pageSize + 1
	search.From = data.Page * pageSize
	search.Fields = []string{"*"}

	if data.Order != "" {
		search.SortBy([]string{data.Order})
	}

	if fDebug == "query" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(search)
		return
	}

	searchResults, err := bleveIndex.Search(search)
	if err != nil {
		RenderError(w, r, err.Error())
		return
	}
	data.Hits = searchResults.Total

	if fDebug == "raw" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(searchResults)
		return
	}

	for _, v := range searchResults.Hits {
		thing, err := NewThingFromHash(v.ID)
		if err != nil {
			RenderError(w, r, err.Error())
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

	if fDebug == "json" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(data)
		return
	}

	RenderPage(w, r, "index.gohtml", data)
}

func RouteFilesJson(w http.ResponseWriter, r *http.Request) {
	if _, ret := HandleAuth(w, r); ret {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(filePointers.List)
}

func RouteAllFilesJson(w http.ResponseWriter, r *http.Request) {
	if _, ret := HandleAuth(w, r); ret {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(filePointers.ByHash)
}

func RouteLogin(w http.ResponseWriter, r *http.Request) {
	data := struct {
		Title string
		Error string
	}{
		Title: "Login",
	}

	fUser := r.FormValue("user")
	fPassword := r.FormValue("password")

	if r.Method == http.MethodPost {
		found := false
		foundUser := ""
		foundPass := ""
		userLowercase := strings.ToLower(fUser)

		for k, v := range config.Users {
			if userLowercase == strings.ToLower(k) {
				foundUser = k
				foundPass = v
				found = found || true
			}
		}

		if !found {
			data.Error = "User not found"
			RenderPage(w, r, "login.gohtml", data)
			return
		}

		if !CheckPasswordHash(fPassword, foundPass) {
			data.Error = "Wrong or empty password"
			RenderPage(w, r, "login.gohtml", data)
			return
		}

		session, _ := sessionStore.Get(r, config.SessionCookieName)
		session.Values["authenticated"] = true
		session.Values["user"] = foundUser
		err := session.Save(r, w)

		if err != nil {
			log.Println(err)
		}

		returnPage := r.FormValue("return")
		if len(returnPage) > 0 && returnPage[0] == '/' {
			http.Redirect(w, r, returnPage, http.StatusFound)
			return
		}

		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	RenderPage(w, r, "login.gohtml", data)
}

func RouteLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		session, _ := sessionStore.Get(r, config.SessionCookieName)
		session.Values["authenticated"] = false
		session.Values["user"] = nil
		err := session.Save(r, w)
		if err != nil {
			log.Println(err)
		}
	}

	http.Redirect(w, r, "/login", http.StatusFound)
}

func RouteThingDetailsJson(w http.ResponseWriter, r *http.Request) {
	if _, ret := HandleAuth(w, r); ret {
		return
	}

	vars := mux.Vars(r)
	vHash := vars["hash"]

	thing, err := NewThingFromHash(vHash)
	if err != nil {
		RenderError(w, r, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(thing)
}

func RouteThingDetails(w http.ResponseWriter, r *http.Request) {
	if _, ret := HandleAuth(w, r); ret {
		return
	}

	vars := mux.Vars(r)
	vHash := vars["hash"]

	thing, err := NewThingFromHash(vHash)
	if err != nil {
		RenderError(w, r, err.Error())
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

	data.FilesRaw, _ = thing.ListFilesRaw()
	RenderPage(w, r, "thingDetails.gohtml", data)
}

func RouteHentagJson(w http.ResponseWriter, r *http.Request) {
	var err error
	if _, ret := HandleAuth(w, r); ret {
		return
	}

	title := r.FormValue("title")
	language := r.FormValue("language")

	request, err := json.Marshal(HentagV1VaultSearchRequest{
		Title:    title,
		Language: language,
	})

	if err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(JsonError{err.Error()})
		return
	}

	url := "https://hentag.com/api/v1/search/vault/title"
	res, err := http.Post(url, "application/json", bytes.NewBuffer(request))

	if err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(JsonError{err.Error()})
		return
	}
	defer res.Body.Close()

	var result HentagV1VaultSearchResponse
	err = json.NewDecoder(res.Body).Decode(&result)
	if err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(JsonError{err.Error()})
		return
	}

	type WorkResult struct {
		Id        string              `json:"id"`
		Title     string              `json:"title"`
		Cover     string              `json:"cover"`
		Tags      map[string][]string `json:"tags"`
		Raw       string              `json:"raw"`
		Locations []string            `json:"locations"`
	}

	var data struct {
		Works []WorkResult `json:"works"`
	}

	data.Works = make([]WorkResult, len(result))
	for k, v := range result {
		raw, _ := json.Marshal(v)

		data.Works[k] = WorkResult{
			Locations: v.Locations,
			Title:     v.Title,
			Cover:     v.CoverImageURL,
			Tags:      v.ToTags(),
			Raw:       string(raw),
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func RouteThingSearchMetadata(w http.ResponseWriter, r *http.Request) {
	if _, ret := HandleAuth(w, r); ret {
		return
	}

	vars := mux.Vars(r)
	vHash := vars["hash"]

	thing, err := NewThingFromHash(vHash)
	if err != nil {
		RenderError(w, r, err.Error())
		return
	}

	basename := filepath.Base(thing.File.PathKey)
	ext := filepath.Ext(thing.File.PathKey)
	search := strings.TrimSuffix(basename, ext)

	data := struct {
		Title            string
		Thing            *Thing
		Search           string
		SearchLanguages  map[int]string
		SelectedLanguage string
	}{
		Title:            thing.Title,
		Thing:            thing,
		Search:           search,
		SearchLanguages:  HentagSearchLanguages,
		SelectedLanguage: config.HentagSearchLanguage,
	}

	RenderPage(w, r, "thingSearchMetadata.gohtml", data)
}

func RouteThingEditMetadata(w http.ResponseWriter, r *http.Request) {
	var err error
	if _, ret := HandleAuth(w, r); ret {
		return
	}

	vars := mux.Vars(r)
	vHash := vars["hash"]

	thing, err := NewThingFromHash(vHash)
	if err != nil {
		RenderError(w, r, err.Error())
		return
	}

	format := r.FormValue("format")

	var metadata = thing.FileMetadataStatic

	switch format {
	case "hentag":
		jsonStr := r.FormValue("json")

		var work HentagV1Work
		err = json.Unmarshal([]byte(jsonStr), &work)
		if err != nil {
			RenderError(w, r, err.Error())
			return
		}

		work.FillMetadata(&metadata)

	case "edit":
		metadata = thing.FileMetadataStatic

	default:
		RenderError(w, r, "Invalid or unknown format")
		return
	}

	data := struct {
		Title    string
		Thing    *Thing
		Metadata FileMetadataStatic
	}{
		Title:    thing.Title,
		Thing:    thing,
		Metadata: metadata,
	}

	RenderPage(w, r, "thingEditMetadata.gohtml", data)
}

func RouteThingSaveMetadata(w http.ResponseWriter, r *http.Request) {
	var err error
	if _, ret := HandleAuth(w, r); ret {
		return
	}

	if r.Method != http.MethodPost {
		w.WriteHeader(405)
		RenderError(w, r, "method not allowed")
	}

	vars := mux.Vars(r)
	vHash := vars["hash"]

	thing, err := NewThingFromHash(vHash)
	if err != nil {
		RenderError(w, r, err.Error())
		return
	}

	thing.CoverImageUrl()
	r.ParseForm()

	thing.FileMetadataStatic, err = NewFileMetadataStaticFromForm(r.PostForm)
	if err != nil {
		w.WriteHeader(400)
		RenderError(w, r, err.Error())
		return
	}

	err = thing.TrySaveStatic()
	if err != nil {
		w.WriteHeader(400)
		RenderError(w, r, err.Error())
		return
	}

	err = thing.File.Reindex()
	if err != nil {
		w.WriteHeader(400)
		RenderError(w, r, err.Error())
		return
	}
	http.Redirect(w, r, thing.DetailsUrl(), http.StatusFound)
}

func RouteThingRatingJson(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	_, ret := CheckAuth(w, r)
	if ret {
		w.WriteHeader(403)
		json.NewEncoder(w).Encode(JsonResponse{"Unauthorized"})
		return
	}

	vars := mux.Vars(r)
	vHash := vars["hash"]

	thing, err := NewThingFromHash(vHash)
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

		if fToggle && thing.Rating == fRate {
			err = thing.TrySaveRating(0)
		} else {
			err = thing.TrySaveRating(fRate)
		}

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

func RouteThingCoverJson(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	_, ret := CheckAuth(w, r)
	if ret {
		w.WriteHeader(403)
		json.NewEncoder(w).Encode(JsonResponse{"Unauthorized"})
		return
	}

	vars := mux.Vars(r)
	vHash := vars["hash"]

	thing, err := NewThingFromHash(vHash)
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

func RouteThingAddMark(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	_, ret := CheckAuth(w, r)
	if ret {
		w.WriteHeader(403)
		json.NewEncoder(w).Encode(JsonResponse{"Unauthorized"})
		return
	}

	vars := mux.Vars(r)
	vHash := vars["hash"]

	thing, err := NewThingFromHash(vHash)
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

func RouteThingSubMark(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	_, ret := CheckAuth(w, r)
	if ret {
		w.WriteHeader(403)
		json.NewEncoder(w).Encode(JsonResponse{"Unauthorized"})
		return
	}

	vars := mux.Vars(r)
	vHash := vars["hash"]

	thing, err := NewThingFromHash(vHash)
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

func RouteThingRead(w http.ResponseWriter, r *http.Request) {
	if _, ret := HandleAuth(w, r); ret {
		return
	}

	vars := mux.Vars(r)
	vHash := vars["hash"]
	vPage, _ := strconv.Atoi(strings.TrimLeft(vars["page"], "/"))

	thing, err := NewThingFromHash(vHash)
	if err != nil {
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(JsonError{err.Error()})
		return
	}

	data := struct {
		Title         string
		Thing         *Thing
		Files         []string
		Page          int
		Hash          string
		ReadThreshold int
	}{
		Title: thing.Title,
		Thing: thing,
		Page:  vPage,
		Hash:  vHash,
	}

	data.Files, err = thing.ListFiles()
	if err != nil {
		RenderError(w, r, err.Error())
		return
	}

	data.ReadThreshold = min(24, int(math.Ceil(float64(len(data.Files))/3.0)))

	RenderPage(w, r, "thingRead.gohtml", data)
}

func RouteThingPushRead(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	_, ret := CheckAuth(w, r)
	if ret {
		w.WriteHeader(403)
		json.NewEncoder(w).Encode(JsonResponse{"Unauthorized"})
		return
	}

	vars := mux.Vars(r)
	vHash := vars["hash"]

	thing, err := NewThingFromHash(vHash)
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

func RouteThingFile(w http.ResponseWriter, r *http.Request) {
	if _, ret := HandleAuth(w, r); ret {
		return
	}

	vars := mux.Vars(r)
	vHash := vars["hash"]
	vFile := vars["file"]
	fSize := r.FormValue("size")

	if fSize == "thumb" {
		if RenderThumbCache(w, r, vHash, vFile) {
			return
		}
	}

	thing, err := NewThingFromHash(vHash)
	if err != nil {
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(JsonError{err.Error()})
		return
	}

	reader, closers, err := thing.getFileReader(vFile)
	if err != nil {
		RenderError(w, r, err.Error())
		return
	}

	defer MultiClose(closers)

	wantedMime := mime.TypeByExtension(filepath.Ext(vFile))

	if fSize == "thumb" {
		original, _, err := image.Decode(reader)
		if err != nil {
			RenderError(w, r, err.Error())
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
			RenderError(w, r, err.Error())
			return
		}
		in = nil

		w.Header().Set("Content-Type", "image/jpeg")
		SetCacheHeader(w, r, 31536000)

		jpegOptions := jpeg.Options{
			Quality: 60,
		}

		if config.CacheThumbnails {
			cacheWriter := ThumbCacheTarget(vHash, vFile)
			if cacheWriter != nil {
				jpeg.Encode(cacheWriter, out, &jpegOptions)
				cacheWriter.Close()
			}
		}

		err = jpeg.Encode(w, out, &jpegOptions)
		out = nil
		if err != nil {
			return
		}
	} else {
		w.Header().Set("Content-Type", wantedMime)
		SetCacheHeader(w, r, 31536000)
		_, err = io.Copy(w, reader)
		if err != nil {
			RenderError(w, r, err.Error())
			return
		}
	}
}

func RouteSystem(w http.ResponseWriter, r *http.Request) {
	if _, ret := HandleAuth(w, r); ret {
		return
	}

	var err error
	r.ParseForm()

	if r.Method == http.MethodPost {
		fAction := r.FormValue("action")

		switch fAction {
		case "reindex":
			w.Header().Set("Content-Type", "application/json")
			if reindexJob.Running {
				json.NewEncoder(w).Encode(reindexJob)
				return
			}

			err = reindexJob.Start()
			if err != nil {
				json.NewEncoder(w).Encode(JsonError{err.Error()})
				return
			}

			http.Redirect(w, r, "/system", http.StatusFound)
			// http.Redirect(w, r, "/system?action=reindexStatus", http.StatusFound)
			return

		case "cancelReindex":
			reindexJob.RequestCancel = true

			http.Redirect(w, r, "/system?action=reindexStatus", http.StatusFound)
			return

		case "reload":
			w.Header().Set("Content-Type", "application/json")
			prev := len(filePointers.List)
			err = InitializeFilePointers()
			if err != nil {
				w.WriteHeader(500)
				json.NewEncoder(w).Encode(JsonError{err.Error()})
				return
			}

			count := len(filePointers.List)
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
		json.NewEncoder(w).Encode(reindexJob)
		return
	}

	data := struct {
		Title   string
		Reindex ReindexJob
	}{
		Title:   "System",
		Reindex: reindexJob,
	}

	RenderPage(w, r, "system.gohtml", data)
}

func RouteSystemReindexStatus(w http.ResponseWriter, r *http.Request) {
	_, notLoggedIn := CheckAuth(w, r)

	data := struct {
		Stop    bool   `json:"Stop,omitempty"`
		Message string `json:"Message,omitempty"`
	}{}

	if notLoggedIn || len(reindexJob.Log) == 0 {
		data.Stop = true
	} else {
		dtLimit := reindexJob.FinishTime.Add(time.Minute)

		if !reindexJob.Running && time.Now().After(dtLimit) {
			data.Stop = true
		} else {
			data.Message = reindexJob.Log[len(reindexJob.Log)-1]
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func RoutePending(w http.ResponseWriter, r *http.Request) {
	if _, ret := HandleAuth(w, r); ret {
		return
	}

	rawquery := bleve.NewQueryStringQuery("collection:\"No Collection *\"")
	query, err := rawquery.Parse()

	if err != nil {
		RenderError(w, r, err.Error())
		return
	}
	search := bleve.NewSearchRequest(query)
	search.Size = len(filePointers.List)
	search.Fields = []string{"*"}
	searchResults, err := bleveIndex.Search(search)

	if err != nil {
		RenderError(w, r, err.Error())
		return
	}

	ret := map[string]string{}
	for _, thing := range searchResults.Hits {
		value, exists := thing.Fields["title"]
		if exists {
			ret[thing.ID] = value.(string)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ret)
}

func RouteFavicon(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "image/x-icon")
	http.ServeFile(w, r, "static/favicon.ico")
}
