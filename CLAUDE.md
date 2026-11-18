# immich-kiosk (fork personale)

Lingua: italiano, risposte concise. Fork di `damongolding/immich-kiosk` (Go + templ + HTMX, frontend in bun) con in più un overlay calendario ICS.

## Obiettivo
Mostrare sul kiosk Immich gli eventi in arrivo di un calendario ICS pubblico, nell'angolo libero dell'overlay. Il resto del codice è upstream: tenere le modifiche al minimo per non complicare il merge da `origin`.

## Remote e rilascio
- `origin` = upstream `damongolding/immich-kiosk` (solo fetch/merge, non fare push).
- `fork` = `jeanpaulrh/immich-kiosk`. Branch di lavoro: `main`.
- Ogni push su `main` fa partire `.github/workflows/docker-build.yml`: immagine multi-arch (amd64/arm64) su `ghcr.io/jeanpaulrh/immich-kiosk` con tag `latest` e SHA corto. Login al registry con il secret `GHCR_TOKEN`. `VERSION` = SHA del commit (serve per il cache busting del CSS).
- Gli altri workflow (`create-binaries`, `ghcr-release`, `post-to-reddit`, `codeql`) sono ereditati da upstream.
- Non pushare e non riavviare né aggiornare container senza chiedere. Dove gira l'immagine (probabilmente uno stack su docker-vm, vedi il CLAUDE.md di `~/homelab`) non è verificato.

## Struttura
- `main.go`: registra le route (`POST /calendar`) e avvia `calendar.StartUpdates()` se `Calendar.URL != ""`.
- `internal/config/config.go`: `CalendarConfig` e campo `Calendar` in `Config`. Ogni nuova opzione va anche in `config.schema.json` e `config.example.yaml`.
- `internal/calendar/calendar.go`: download, parsing e cache dell'ICS. Gli orari UTC vengono convertiti nel fuso locale.
- `internal/routes/routes_calendar.go`: handler di `POST /calendar`.
- `internal/templates/partials/calendar.templ`: HTML degli eventi. `views/views_home.templ` mostra `CalendarContainer` se l'URL è impostato.
- `frontend/src/css/calendar.css`, importato da `kiosk.css`: posizionato `absolute`, **in basso a destra** (era a sinistra, spostato nel commit `467106ce`).
- Altre cartelle upstream: `internal/{immich,weather,cache,video,webhooks,i18n,...}`, `locales/`, `iframes/`.
- Il file `immich-kiosk` nella root è un binario Mach-O arm64 già tracciato da upstream: non toccarlo.
- `*_templ.go`, `dist/`, `config.yaml` e `.env` sono ignorati da git: i `_templ.go` si rigenerano con `task templ`.

## Config del calendario (`config.yaml`)
```yaml
calendar:
  url: "https://example.com/calendar.ics"  # ICS pubblico; vuoto = funzione disattivata
  max_events: 5          # default 5
  update_interval: 3600  # secondi, default 3600
  days_ahead: 7          # default 7
```

## Comandi (taskfile, richiede `task`, Go, bun)
- `task dev`: avvio con `air` (hot reload, log debug). `task verbose`: debug verboso.
- `task build`: frontend + templ + binario in `dist/kiosk`. `task`: build e run.
- `task templ`: rigenera i template (`go tool templ generate`). Da fare dopo ogni modifica ai `.templ`.
- `task test`: templ + `go test ./...`. `go test ./internal/calendar/...` per il solo calendario (non ha ancora test).
- `task lint`: `golangci-lint run` + lint frontend (biome).
- `task frontend`: lint, test, css, js e url-builder in `frontend/`.
- `task outdated`: dipendenze obsolete. Task Docker in `taskfile.docker.yml` (non verificati).

## Regole
- Prima di toccare il calendario leggere i file sopra, non affidarsi alla memoria: la memory dice ancora "bottom-left", ma il codice è corretto "bottom-right".
- Nuove opzioni di config: aggiornare insieme `config.go`, `config.schema.json`, `config.example.yaml` (il commit `a8a27c74` ha dovuto rimediare allo schema).
- Dopo modifiche a `.templ` o CSS: `task templ` e `task build` prima di dichiarare che funziona. Dichiarare cosa non è stato verificato.
- Commit piccoli, in stile conventional (`feat:`, `fix:`, `ci:`, `docs:`).
- Prima di ogni commit: `git add -n -A` e controllare l'elenco. Mai committare `.env`, `config.yaml` o URL di calendari privati (l'URL ICS può contenere un token segreto).
- Non incollare in chat né nei commit l'URL reale del calendario.
- La memory di Claude per questo progetto sta in `~/.claude/projects/-home-claude-homelab-projects-immich-kiosk/memory/`. Il percorso `/home/gianpaolo/immich-kiosk` citato lì è della vecchia macchina.
