# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

Sugoi is a self-hosted, single-binary Go web app for browsing, searching and reading image archives (zip/cbz/rar/7z etc.) with per-item metadata, ratings, read tracking and full-text search. `main.go` at the repo root wires the packages under `internal/`.

## Commands

```bash
go build -o sugoi .            # build (binary name is gitignored)
./sugoi                        # run; reads ./config/sugoi.json by default
./sugoi -c path/to/config.json # alternate config
./sugoi -u                     # interactive: add user / change password, rewrites the config file
go test -race ./...            # all tests (web tests read templates/static from the repo root)
go test ./internal/web -run TestPages   # a single test
go vet ./...
go generate                    # runs cmd/static/main.go: re-downloads vendored jQuery/Bootstrap(Bootswatch darkly)/FontAwesome into static/
```

The binary must run with the repo root (or deploy dir) as working directory: templates (`templates/`, `pages/`) and `static/` are loaded from relative paths at runtime (`web.NewServer(..., ".")`), not embedded. See `sugoi.service` for the systemd setup. SIGINT/SIGTERM shut down gracefully (finish requests, cancel a running reindex, close the index).

Setup: copy `config/sugoi.example.json` to `config/sugoi.json`. Startup fails unless `MaxConcurrency > 0`, `SessionCookieMaxAge > 0`, and `SessionCookieKey` is base64 of ≥32 bytes (the app prints a freshly generated one if not). `config/`, `db/` and `cache/` contents are gitignored.

Test data must stay neutral: no real site names, adult tags or Portuguese names in tests or fixtures.

## Architecture

Packages, each depending only on the ones above it (no package-level mutable state outside `web.Server` / `catalog.Catalog`):

```
config, fsutil  →  metadata, library  →  search  →  catalog  →  web  →  main
```

**`internal/config`** — `Config` loaded from JSON (`Load` validates; `SessionKeyError` carries a suggested key). Saved with 0600 (holds password hashes).

**`internal/fsutil`** — `WriteFileAtomic` (temp file + rename, optional fsync) used for every write: metadata, config, page-list and thumbnail caches.

**`internal/library`** — `db/files.txt`: one archive path per line (`#`, `//`, `;` comments). Paths may contain `{{VAR}}` placeholders from `config.DirVars`. Each line is a `library.File` whose identity is `Hash = sha1(raw line)`, the ID used in every URL (`/thing/.../{hash}`); changing a line (including the DirVar name) changes the hash and orphans its metadata. `Library` is an immutable snapshot; `/system` → reload builds a new one and swaps it (`catalog.Reload`), keeping the old list on error. `Pages`/`Open` read archives via `github.com/mholt/archives`; page images (`.png/.jpg/.jpeg/.webp`, case-insensitive, natural-sorted) are cached in `cache/thing/file/<hash>/.files`. `File.Lock` is the per-item lock for metadata writes.

**`internal/metadata`** — `db/meta/<PathKey>/` (`PathKey` = key with `{{VAR}}` lowercased):
- `static.json` → `Static`: descriptive fields, mostly produced by the separate importer project `theplan` (converts ccdc06/metadata YAML). The edit form is applied onto the file on disk (`ApplyForm`), so fields it doesn't show (id, circle, pages, files…) are kept.
- `dynamic.json` → `Dynamic`: user state (rating, marks, cover, read counts/timestamps).

Reading a missing file returns empty metadata and never creates it (reads run outside the item lock; an empty `static.json` would also make `theplan` skip the item). `StringArray` accepts a string or array; `IdArray` accepts theplan's array or the older `{"source":"id"}` object and is always written as an array.

**`internal/search`** — Bleve index at `db/sugoi.bleve`, wrapped by `search.Index` (RWMutex; `Recreate` swaps in an empty index for a full reindex). `BuildMapping` only applies when the index is created, so mapping changes need a full reindex. The indexed `Document` is static + dynamic + `*_kw` untokenized copies (`KeywordFields`) + `random.<n>` sort keys + flattened `url`. Query semantics: the home page parses `?q=` with Bleve's query string, then `ShouldToMust` makes unprefixed terms required (`A B` = A AND B). Analyzed fields match words (`artist:bar` also hits "foo bar"); exact values use `*_kw` (`artist_kw:"bar"`; `series` is itself keyword). `Term`/`KeywordField` build the `+field:"value"` links used by badges. `Autocompleter` keeps every value of tags/artist/parody/collection/series (+ `rating:`/`marks:` metatags) in memory, built from facets over a match-all search (not `FieldDict`, whose counts include deleted docs) and rebuilt lazily after changes (`MarkDirty`).

**`internal/catalog`** — `Catalog` = library + index + autocompleter + `ReindexJob`. `Get(hash)` returns a `catalog.Thing` (file + static + dynamic, with placeholder title/collection/pages filled in memory only). Writes (`TrySaveRating`, `AddMark`, `PushRead`, `SetCover`, `SaveStaticFromForm`) take the item lock, re-read the file, apply the change, write it atomically and reindex that one document; they return `ErrReindexRunning` during a full reindex (a queued older copy would overwrite the change). `ReindexJob` state is mutex-guarded; read it via `Status()`.

**`internal/web`** — `Server` (config, catalog, session store, template cache, Basic-auth cache); `Handler()` builds the gorilla/mux router with a `MaxConcurrency` semaphore. Handlers are `srv.routeX` methods registered in `routes()`. Auth: HTML routes call `handleAuth` (HTTP Basic or cookie session, redirects to `/login`); JSON routes call `checkAnyAuth` and return 403. Successful Basic checks are cached in memory (bcrypt cost 14 is ~1s). `web.Thing` embeds `*catalog.Thing` and adds the URL/HTML helpers the templates call (`CoverImageUrl`, `SortedTags`, `HtmlTable`, `UrlButtons`…). Thumbnails (`?size=thumb`): 256px JPEG, cached under `cache/thing/file/<hash>/<file>` when `CacheThumbnails`; error pages are sent with `Cache-Control: no-store`.

**Templates.** `templates/*.gohtml` are partials (`e_head`, `e_header`, `e_footer`, `e_details_table`), `pages/*.gohtml` full pages rendered by filename (`srv.renderPage(w, r, "index.gohtml", data)`). Template funcs (`templateFuncs`) must not depend on the request, since parsed templates are shared. With `Debug` true templates are re-parsed per request (edit without restarting), otherwise once; either way the latest set is kept for partials rendered from Go (`Thing.HtmlTable` → `currentTemplates()`). The HTTP tests run with Debug on and off.

**Frontend.** Plain jQuery + Bootstrap 5, no build step. `static/main.js` (global UI, rating/marks/cover AJAX), `static/queryHistory.js` (search history), `static/autocomplete.js` (search box suggestions, enabled by `data-autocomplete`), `static/reader.js` (the page reader, bootstrapped by `window.sugoi.reader.load(ReaderData)` in `thingRead.gohtml`; back/forward between items loaded in place uses `/thing/read/<hash>/<page>?json=1`). Source icons for URL buttons are in `static/source-icons/`.

**Logging.** `log/slog` text handler on stderr; Debug level when `config.Debug`.
