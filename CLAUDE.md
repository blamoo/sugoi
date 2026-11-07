# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

Sugoi is a self-hosted, single-binary Go web app for browsing, searching and reading image archives (zip/cbz/rar/7z etc.) with per-item metadata, ratings, read tracking and full-text search. Everything is `package main` at the repo root; there are no tests.

## Commands

```bash
go build -o sugoi .            # build (binary name is gitignored)
./sugoi                        # run; reads ./config/sugoi.json by default
./sugoi -c path/to/config.json # alternate config
./sugoi -u                     # interactive: add user / change password, rewrites the config file
go generate                    # runs cmd/static/main.go: re-downloads vendored jQuery/Bootstrap(Bootswatch darkly)/FontAwesome into static/
go vet ./...
```

The binary must run with the repo root (or deploy dir) as working directory: templates (`templates/`, `pages/`) and `static/` are loaded from relative paths at runtime, not embedded. See `sugoi.service` for the systemd setup.

Setup: copy `config/sugoi.example.json` to `config/sugoi.json`. Startup fails unless `MaxConcurrency > 0`, `SessionCookieMaxAge > 0`, and `SessionCookieKey` is base64 of ≥32 bytes (the app prints a freshly generated one if not). `config/`, `db/` and `cache/` contents are gitignored.

## Architecture

**Library source — `db/files.txt`.** One archive path per line (`#`, `//`, `;` comments allowed). Paths may contain `{{VAR}}` placeholders resolved through `config.DirVars` (`FilePointer.RealLocation`). Each line becomes a `FilePointer` (`filepointer.go`) whose identity is `Hash = sha1(raw line)`; the hash is the ID used in every URL (`/thing/.../{hash}`). Changing a line in `files.txt` (including the DirVar name) changes the hash and orphans its metadata. The list is loaded at startup and by `/system` → reload (not safe under concurrent requests; accepted downtime, since files are only added/removed together with a full reindex).

**Metadata on disk — `db/meta/<PathKey>/`.** `PathKey` is the raw key with `{{VAR}}` lowercased to `var` (`BuildPathKey`). Two JSON files per item (`metadata.go`):
- `static.json` → `FileMetadataStatic`: descriptive fields (title, tags, artist, urls, optional explicit `files` list…), edited via the editMetadata form. The form is applied onto the file on disk (`ApplyForm`), so fields it doesn't show (id, circle, pages, files…) are kept. Most are produced by the separate importer project `theplan` (converts ccdc06/metadata YAML).
- `dynamic.json` → `FileMetadataDynamic`: user state (rating, marks, cover, read counts/timestamps), mutated by JSON endpoints.

Missing files are auto-created as `{}`. Empty title/collection fall back to values derived from the filename/parent directory (`FillEmptyFields`, `PlaceholderTitle`). `StringArray`/`IdArray` (`stringarray.go`) accept either a string or array in JSON.

**`Thing`** (`thing.go`) = `FilePointer` + static + dynamic metadata, built per request via `NewThingFromHash`. It's the main view-model; templates call its methods directly (`CoverImageUrl`, `SortedTags`, `HtmlTable`, `UrlButtons`…). Mutators (`TrySaveRating`, `AddMark`, `PushRead`, `TrySaveCover`, …) go through `updateDynamic`, and the edit form through `SaveStaticFromForm`: under a per-item lock (`FilePointer.mu`) they re-read the file from disk, apply the change, write it and call `File.Reindex()` for that one document. Both refuse with `ErrReindexRunning` during a full reindex (a queued older copy would overwrite the change in the index).

**Search — Bleve** (`bleve.go`), index at `db/sugoi.bleve`. The indexed document is `FileMetadata` (static + dynamic + `*_kw` keyword copies + a `random` map used for random ordering + flattened `url`). The mapping is defined in `BuildNewMapping` and only applied when the index is created, so mapping changes require a full reindex (`/system` → reindex, `reindex.go`: deletes and rebuilds the index in a background goroutine, batching 500 docs). The home page (`RouteRoot`) runs a Bleve query-string query from `?q=`, then `shouldToMust` makes unprefixed terms required (`A B` = A AND B). Analyzed fields match words (`artist:bar` also hits "foo bar"); exact values use the `*_kw` keyword fields (`artist_kw:"bar"`; `series` is itself a keyword field). Tag badges / quick filters (`searchterm.go`, `quickfilter.go`, `config.QuickFilters`) build and strip `+field:"value"` terms, using `_kw` when it exists (`searchField`). Sort keys come from `order.go`.

**Autocomplete** (`autocomplete.go`, `static/autocomplete.js`, enabled by `data-autocomplete` on the search input). `/autocomplete.json` filters an in-memory list of tags/artist/parody/collection/series values plus `rating:`/`marks:` metatags. The list is built from facets over a match-all search (not `FieldDict`, whose counts include deleted docs) and rebuilt lazily after any index change (`MarkAutocompleteDirty`).

**Archive access.** Archives are opened with `github.com/mholt/archives` as an `fs.FS`. Image list (`.png/.jpg/.jpeg/.webp`, natural-sorted) is cached in `cache/thing/file/<hash>/.files` unless `static.json` has an explicit `files` list. `/thing/file/{hash}/{file}` streams an entry; `?size=thumb` resizes to 256px wide JPEG (rez bicubic) and caches it under `cache/thing/file/<hash>/<file>` when `CacheThumbnails` is on. Deleting `cache/` is safe.

**HTTP layer** (`main.go`, `routes.go`). gorilla/mux; all routes registered in `Routes()`. A global semaphore middleware limits concurrent requests to `config.MaxConcurrency`. Auth (`session.go`): HTML routes call `HandleAuth` (HTTP Basic or cookie session, redirects to `/login`); JSON routes call `CheckAnyAuth` (same, without redirect) and return 403 instead. Successful Basic checks are cached in memory (bcrypt cost 14 is ~1s). Users/bcrypt hashes live in the config file.

**Templates** (`render.go`). `templates/*.gohtml` are partials (`e_head`, `e_header`, `e_footer`, `e_details_table`), `pages/*.gohtml` are full pages rendered by filename (`RenderPage(w, r, "index.gohtml", data)`). Custom funcs (`templateFuncs`) must not depend on the request, since parsed templates are shared: `asset`, `noescape`, `getBuildTime`, `getBuildDiff`, `formatTime`. With `config.Debug` true, templates are re-parsed on every request (no restart needed for template edits); otherwise parsed once. Either way the latest set is kept in `templateStore`, which partials rendered from Go (`Thing.HtmlTable`) read via `currentTemplates()`.

**Frontend.** Plain jQuery + Bootstrap 5, no build step. `static/main.js` (global UI, rating/marks/cover AJAX), `static/queryHistory.js` (search history), `static/reader.js` (the page reader, bootstrapped by `window.sugoi.reader.load(ReaderData)` in `thingRead.gohtml`; `ReaderData` is built in `reader.go`). Source icons for URL buttons are in `static/source-icons/`.
