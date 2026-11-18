# immich-kiosk (fork personale)

Lingua: italiano, risposte concise. Fork di `damongolding/immich-kiosk` (Go + templ + HTMX, frontend in bun) con in più un overlay calendario ICS.

## Obiettivo
Mostrare sul kiosk Immich gli eventi in arrivo di un calendario ICS pubblico, nell'angolo libero dell'overlay. Il resto del codice è upstream: tenere le modifiche al minimo per non complicare il merge da `origin`.

## Remote e rilascio
- `origin` = upstream `damongolding/immich-kiosk` (solo fetch/merge, non fare push).
- `fork` = `jeanpaulrh/immich-kiosk`. Branch di lavoro: `main`.
- Ogni push su `main` fa partire `.github/workflows/docker-build.yml`: immagine multi-arch (amd64/arm64) su `ghcr.io/jeanpaulrh/immich-kiosk` con tag `latest` e SHA corto. Login al registry con il secret `GHCR_TOKEN`. `VERSION` = SHA del commit (serve per il cache busting del CSS).
- Gli altri workflow (`create-binaries`, `ghcr-release`, `post-to-reddit`, `codeql`) sono ereditati da upstream.
- Non pushare e non riavviare né aggiornare container senza chiedere.

## Deploy
Il servizio gira su **docker-vm**. Compose in `/var/lib/docker-data/compose/immich-kiosk/docker-compose.yml` (qui montato in sola lettura in `~/mnt/docker-compose/immich-kiosk`: da qui non si modifica né si fa deploy). Il deploy lo fa l'utente su docker-vm, io non applico nulla.
- Il compose usa `image: ghcr.io/jeanpaulrh/immich-kiosk:latest` (nessun `build:`): il codice arriva solo dall'immagine costruita dal workflow.
- Config in `/var/lib/docker-data/immich-kiosk/config` (bind su `/config`, contiene `config.yaml` col vero URL ICS). Una nuova opzione di config va aggiunta lì a mano su docker-vm.
- Esposto solo via Traefik (rete `traefik-net`, middleware `local-only`), nessuna porta pubblicata. Healthcheck su `/health`. `TZ=Europe/Rome`.

Procedura per una modifica:
1. Verificare in locale (`task templ`, `task test`, `task build`), committare.
2. Con l'ok dell'utente, push su `fork/main`: parte `docker-build.yml` (verificare che sia verde su GitHub Actions).
3. Su docker-vm, in `/var/lib/docker-data/compose/immich-kiosk`: `docker compose pull && docker compose up -d`.
4. Controllare `docker compose ps` (healthy) e `docker compose logs --tail=50 immich-kiosk`. Se serve un hard refresh nel browser del kiosk per il CSS.
- Rollback: tag SHA corto dell'immagine precedente nel compose, poi `docker compose up -d`.
- Se cambia anche il compose, la patch va applicata dall'utente su docker-vm: preparare il diff e consegnarlo, senza scrivere in `~/mnt`.

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
- Prima di toccare il calendario leggere i file sopra, non affidarsi alla memoria: in caso di dubbio vale il codice.
- Nuove opzioni di config: aggiornare insieme `config.go`, `config.schema.json`, `config.example.yaml` (il commit `a8a27c74` ha dovuto rimediare allo schema).
- Dopo modifiche a `.templ` o CSS: `task templ` e `task build` prima di dichiarare che funziona. Dichiarare cosa non è stato verificato.
- Commit piccoli, in stile conventional (`feat:`, `fix:`, `ci:`, `docs:`).
- Prima di ogni commit: `git add -n -A` e controllare l'elenco. Mai committare `.env`, `config.yaml` o URL di calendari privati (l'URL ICS può contenere un token segreto).
- Non incollare in chat né nei commit l'URL reale del calendario.
- La memory di Claude per questo progetto sta in `~/.claude/projects/-home-claude-projects-immich-kiosk/memory/`.

## Decisioni (con motivo)
- Overlay in basso a destra (prima a sinistra, `467106ce`): è l'angolo libero dell'overlay del kiosk.
- Aggiornamento via polling HTMX (`POST /calendar`) come l'overlay meteo, con cache e refresh in background (`dd3b2b09`): riusa il pattern upstream.
- Orari UTC convertiti nel fuso locale (`1afb1806`): motivo esteso non annotato, da verificare.
- Modifiche limitate ai file elencati sopra: il resto è upstream e va tenuto allineabile per i merge da `origin`.
- Workflow `docker-build.yml` separato per il fork (`08b3b066`); `VERSION` = SHA per il cache busting del CSS (`cc9a982e`).
- Login GHCR con il secret `GHCR_TOKEN` al posto di `GITHUB_TOKEN` (`3b827ab7`): motivo non annotato, da verificare.
- Il compose usa solo l'immagine del fork, nessun `build:`: il deploy passa sempre da CI.

## Cose aperte
- `internal/calendar` non ha test.
- Task Docker in `taskfile.docker.yml` mai verificati.
- Ultimo merge da `origin` e stato di allineamento con upstream: da verificare.