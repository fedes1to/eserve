# AGENTS.md

## What this is

`eserve` is a **prototype** Gentoo build server: a server (`eserved`) that provisions per-machine Gentoo chroots ("flavors"), builds packages in them, and serves the results back to clients (`epull`) over mTLS (binpkgs over a plain-https binhost). The loop is working end-to-end on two VMs (register → provision → sync → admin-triggered build → binhost consume → epull selfupdate) but it is Gentoo-only for now. Read `ROADMAP.md` for what is done vs. planned before proposing features.

- Pure Go, **zero external dependencies** (no requires in `go.mod`, Go 1.26.4). Keep it stdlib-only.
- Module path is a private forge (`git.fedesito.me/fedes1to/eserve`), not publicly fetchable — imports must keep that prefix.
- License: PolyForm Noncommercial 1.0.0.
- "racc" in log strings/comments is the nickname for the job worker (an in-joke). Don't "fix" it.

## Commands

```sh
go build ./...                  # build everything (no Makefile, no CI, no task runner)
go build -o eserved ./cmd/eserved
go build -o epull ./cmd/epull
go build -o eservectl ./cmd/eservectl
go test ./...                   # passes trivially — there are NO test files in the tree
go vet ./...
```

There is no linter/formatter config in the repo; `gofmt`/`go vet` are the only tooling.

**Running requires root.** Config paths are hardcoded (`/etc/eserved`, `/etc/epull`) and `InitConfigPath` calls `log.Fatalln` if it can't create the config directory. There is no env override.

Typical flow: start `eserved` (first run auto-generates `/etc/eserved/settings.json` with defaults, a CA, and a server cert **signed by that CA** — SANs = `eserver` + every non-loopback IPv4 on the host) → `eservectl token create` → `epull register -token <t> -server https://host:8080 -flavor <name> -insecure`. The `-insecure` flag is still effectively required for the mTLS API (epull skips server verification), but plain-https clients (portage) can verify the server cert by trusting the eserved CA in their system store.

## Architecture

### Trust model (understand this first)

1. **Bootstrap**: admin creates a single-use bearer token via the unix socket.
2. **Identity** (`POST /api/v1/identity`, the only endpoint without a client cert): `epull` generates an ed25519 keypair + CSR (CN = hostname) and POSTs it with the token. The server signs a 1-year client cert, records the machine keyed by CN with `sha256(cert.Raw)` as fingerprint, and consumes the token.
3. **mTLS afterwards**: `requireClientCert` (cmd/eserved/http.go:23) validates CN + fingerprint against `machines.json` and rejects revoked machines, then puts a `ClientIdentity` in the request context.

Non-obvious details:

- The TLS config uses `tls.RequestClientCert`, **not** `Required` — enforcement is per-route via `requireClientCert` in the mux setup, so new protected endpoints must be wrapped explicitly.
- Both sides enforce TLS 1.3 minimum.
- **Tokens are one-shot** (single identity, or single flavor-switch during provision). New tokens via `eservectl token create`. `isTokenAvailableLocked` treats a used token with an existing machine as unavailable.
- **Revocation is sticky**: `eservectl machine revoke -cn <name>` sets `RevokedAt`; neither re-identifying nor re-provisioning clears it, and there is no un-revoke endpoint.
- Flavor is used as a chroot **directory name** — no slashes/spaces. `chroot.validFlavor` enforces this in the sync/provision-apply paths; identity itself only checks non-empty.

### HTTP surface

Admin API on unix socket `/run/eserved.sock` (chmod 600; disabled entirely with `-admin=false`): `/admin/v1/token/create`, `/admin/v1/token/list`, `/admin/v1/machine/revoke`, `/admin/v1/machine/list`, `/admin/v1/build/start`, `/admin/v1/jobs/list`, `/admin/v1/jobs/cancel`, `/admin/v1/jobs/stream`, `/admin/v1/flavor/apply`, `/admin/v1/binary/upload`, `/admin/v1/binary/list`. The client uses the Go net/http unix-socket trick: a `DialContext` that dials `"unix"` and URLs of the form `http://unix<suburl>` (cmd/eservectl/admin/http.go).

Public API on TLS at `settings.json` `listen_addr` (default `127.0.0.1:8080`), all routes in `internal/urls/api.go`:

- `/api/v1/identity` — bootstrap (no client cert)
- `/api/v1/provision` — 202 + `{job_id, binhost_url, flavor}`; switching flavor requires a fresh bearer token. `binhost_url` is per-flavor (`<base_binhost_url>/<flavor>`)
- `/api/v1/sync` — stores the client's portage-config tar.gz (64 MiB limit) at `/etc/eserved/sync/<flavor>/<cn>.tar.gz`, then applies it to the flavor chroot: canonical copy in `<chroot>/.eserved/` (via `os.Root`, symlinks/hardlinks/traversal rejected) and relative symlinks from `etc/portage/` into it. Requires the `X-Portage-Fingerprint` header and a provisioned flavor; records the fingerprint in `sync/<flavor>/fingerprint.json`
- `/api/v1/sync/check` — compares `X-Portage-Fingerprint` against the flavor's recorded fingerprint: 204 in sync, 409 (JSON `{flavor, fingerprint}`) when it differs or was never synced, 400 when the machine has no flavor
- `/api/v1/stages` — plain-text list of stage3 tarballs in the stage dir
- `/api/v1/jobs/stream` — **POST** (JSON `{job_id}`), returns a live event stream
- `/api/v1/binary` — GET (`?name=&arch=`), the server-hosted build of a binary (mTLS), for `epull selfupdate`
- `/api/v1/binary/manifest` — GET, its `{name, arch, sha256, size, uploaded_at}`
- `/pkgs/` — the binhost: `http.FileServer` over `repo_base` (prefix-stripped), **no client cert** (portage can't do mTLS); the `binaries/` subtree is hidden from it (stays mTLS-only at `/api/v1/binary`)

**The job stream is real SSE.** `/api/v1/jobs/stream` sends `event:` = the event type (`progress`, `output`, `done`, `error`, `cancelled`), `data:` = the message, one blank-terminated event per job line, flushed individually — the same events that go into the jsonl log (multi-line messages become multiple `data:` lines, per spec). Both clients parse it with the shared reader `protocol.EachSSEEvent` (cmd/epull/api/jobs.go, cmd/eservectl/admin/jobs.go): `done`/`cancelled` are clean stops, `error` is failure — and colorize for display locally (`Colorize` in `internal/protocol/job.go`); ANSI never appears on the wire.

### Server state

Everything is a JSON file under `/etc/eserved/` plus in-memory maps guarded by `sync.RWMutex`:

| File | Content |
|---|---|
| `settings.json` | auto-generated defaults on first run; the only real config (listen addr, chroot/stage paths, binhost URL) |
| `ca.crt`/`ca.key` | auto-generated ed25519 CA, 10 years |
| `server.crt`/`server.key` | server cert signed by the CA, 1 year, SANs = `eserver` + host IPv4s |
| `sync/<flavor>/<cn>.tar.gz` | uploaded client portage configs; `sync/<flavor>/fingerprint.json` records the last synced config (fingerprint + who/when) |
| `flavors/<name>/` | the flavor's own portage config (plain files: `make.conf`, `binrepos.conf`, ...); the server's layer, applied under every client's config |
| `jobs/<id>.jsonl` | live job log — **deleted when the job finishes** |

Outside `/etc/eserved/`: `<repo_base>/<flavor>/` is the binhost (default `/srv/pkgs`): immutable `snapshot-<unix>/` dirs of built binpkgs + a live `Packages`/`Packages.gz` index at the flavor root, and `<repo_base>/binaries/<arch>/<name>` (+ `.json` manifest) are the server-hosted client binaries.

**Job registry** (cmd/eserved/jobs/jobs.go): in-memory only. **Flavored jobs run one at a time per flavor** (the chroot is shared): they wait in a buffered channel (`queueBuffer = 8`, a full queue rejects fast), served by a per-flavor worker goroutine; jobs with `flavor == ""` run immediately in fresh goroutines. `Finish` is idempotent: work functions may end their own job with a custom terminal event, and if they just return, the registry marks them `done` (`<kind> complete`). Panics are recovered in both run modes. The log file is deleted on `Finish` with only the terminal event kept in memory; a janitor goroutine (10 min tick) reaps finished jobs after 1 hour. After that, all history is gone — there is no persistence.

**Provisioning** (cmd/eserved/chroot/provision.go): stagefile must be a plain filename (path-traversal guard); extraction is `tar -xpf` into `<chroot_base>/<flavor>` and writes the marker `etc/eserved-stage3.json`; extraction is skipped when the marker exists (unlike the old `etc/` check, it can't be fooled by synced config). After (re-)extraction, `restoreSyncedConfig` re-applies the machine's portage config: re-linking the `etc/portage/` symlinks into the surviving `<chroot>/.eserved/` dir, or re-extracting from the stored sync archive if that dir was lost — so a re-provision does not force a client re-sync. Cross-dev (client gcc-machine arch ≠ server's) is rejected — see `chroot.IsGccMachineDiff`, which only compares the part before `-`.

**Flavor config layering** (cmd/eserved/chroot/flavorconfig.go): every portage-config mutation builds a staging dir as **flavor config → client sync archive on top → flavor `make.conf` last**. The client wins file-by-file, but the flavor always wins `make.conf` (the build environment is the server's call — the scaffolded one strips `binpkg-request-signature`, which a stock stage3 make.conf requests). `ApplySync` and the admin flavor-apply both go through this, under the per-flavor `flavorlock`. `ApplyFlavorToChroot` re-applies from the stored per-client archives (the last-synced client goes last, same as the last sync); the merged `.eserved/` dir is never used as a merge source.

**Build jobs** (cmd/eserved/chroot/build.go): started by admin (`/admin/v1/build/start`), run through the flavor queue as `kind = "build"`. They take the flavor lock (so a sync can't swap config mid-build), bind-mount `/dev`, `/proc`, `/sys`, `/etc/resolv.conf`, `/etc/hosts` into the chroot (idempotent via `/proc/self/mountinfo`; unmounted again after), then ensure the chroot has a working gentoo repo: fresh `.eserved-repo-updated` marker (< 7 days) → use as-is; otherwise `emerge --sync` inside the chroot, falling back to `cp -a` of the host's `/var/db/repos/gentoo` (works with no chroot network; the marker records the update either way). The build itself is `emerge --buildpkg --usepkg=y -j<build_threads> <atoms...>` streamed to the job; atoms are validated against a strict regex (no shell metacharacters, no use flags). `PublishBinpkgs` (cmd/eserved/chroot/binhost.go) then copies the chroot's `var/cache/binpkgs` into `<repo_base>/<flavor>/snapshot-<unix>/`, rewrites the index's global-header `URI:` to the snapshot's absolute URL (the 3.0 client joins header `URI` — falling back to the sync-uri — with each entry's `PATH`), publishes live `Packages` + `Packages.gz` at the flavor root, and prunes to the newest 3 snapshots.

### Client (`epull`)

State in `/etc/epull/`: `settings.json` (server URL + cert/key paths) and `<hostname>.crt`/`.key` written by register. `register` runs identity → **interactive stage prompt** (`fmt.Scanln`, no `-stage` flag on register) → provision → streams the job → upserts a portage `binrepos.conf` section (created when missing, rewritten when the server hands out a new sync-uri) → checks portage sync (prompts to re-sync if the flavor drifted). `epull sync` (also after `provision`) does the same check standalone: `SyncPortage` fingerprints `/etc/portage` over `protocol.PortageSyncPaths` (shared with the server), `CheckSync` compares, `UploadPortage` uploads — `-y` skips the prompt. `epull selfupdate` downloads the server-hosted `epull` for the local arch over mTLS (`/api/v1/binary`), verifies size + sha256 against the manifest, and atomically replaces the running binary (temp file + rename next to it). Ctrl+C once while streaming cancels the job (epull stops cleanly, not with an error); twice exits 130.

### Shared internals

- `internal/protocol` — wire structs (snake_case JSON tags) + ANSI colorization
- `internal/urls` — route constants, socket path
- `internal/config` — `SafeSaveJsonFile` (tmp + rename), `LoadJsonFile` (missing file = OK, empty struct), hardcoded paths
- `internal/cli` — ordered `InitStep`/`MustInit` and the usage template
- `internal/sysinfo` — Gentoo/gcc detection: `gcc -dumpmachine`, `portageq envvar CFLAGS`, `/etc/portage/make.profile`
- `internal/sharedStorage` — stage listing; lazily loads server settings
- `internal/flavorlock` — one mutex per flavor; every chroot mutation (sync apply, provision config restore, flavor apply, build) takes it so they never run in parallel for the same flavor

## Conventions

- Global config vars (`serverConfig.Settings`, `clientConfig.Settings`, `storage.CaCertificate`, `jobs.Registry`) set once at startup. No DI framework; follow the pattern.
- `*Locked` function suffix means **only call while holding the corresponding mutex** (the author's comments are loud about it).
- CLI subcommand handlers return `(error, int)` where int is the exit code: 1 = failure, 2 = usage error.
- Errors wrapped with `%w`, casual tone; logging via stdlib `log`.

## Gotchas / known issues

Other things that will bite:

- The identity context value is asserted directly (`r.Context().Value(api.CtxKeyIdentity).(ClientIdentity)`) — the key is a typed constant (`type ctxKey string`) so other code can't collide with it, but the value assertion has no compile-time safety.
- `epull register` blocks on the interactive stage prompt — it won't work non-interactively as-is.
- `eserved` has only one flag (`-admin`); everything else (listen addr, paths) is in `settings.json`.
- `MachineEntry.Arch` (json `arch`) is never populated; only `Subarch`/`Profile` are.
- The binhost is **unsigned**: the epuller's `binrepos.conf` must have `verify-signature = false` (the template does), and a client whose `make.conf` has `binpkg-request-signature` in `FEATURES` will reject the binpkgs — the scaffolded flavor `make.conf` strips it for the build chroot, but a *client* machine needs it stripped in its own config.
- `settings.json`'s `base_binhost_url` is baked into the published index (`URI:` header) and into the provision response, so it must be a URL the *clients* can reach (the LAN IP, not `127.0.0.1`), and changing it changes what clients fetch.
- For portage to actually consume the binhost, the **client's system trust store must trust the eserved CA** (`/etc/eserved/ca.crt` — drop it in the distro's local CA dir and run its update tool); until then the index fetch fails `CERTIFICATE_VERIFY_FAILED` and emerge silently falls back to source builds.
- Portage 3.0 binrepo fetch semantics (verified on 3.0.81): the client fetches `<sync-uri>/Packages` itself (tries `.gz` first) and stores fetched gpkg under the section's `location`; `sync-type` in `binrepos.conf` is a **dead key** in 3.0 (the sync-module machinery is for portage-tree sync only) — don't write it. `verify-signature = false` stays required until repo signing lands (`openpgp-key-package` is the 3.0 slot for it).
- `build_threads` is used (`-j` for emerge builds, 0 = unlimited); `per_user_threads` is an unused leftover field.
- The dev Go toolchain on the build VMs rejects a trailing `...` spread preceded by more fixed args than the function has non-variadic params (e.g. `exec.CommandContext(ctx, "chroot", dir, "env", "-i", ..., args...)` doesn't compile). Build the full arg slice and spread once.
- ROADMAP's **NOT GOING TO BE IMPLEMENTED** list: fail2ban/rate limits, reverse-proxy support, automatic Let's Encrypt — don't propose these. (Note `ClientIP` does read `X-Forwarded-For` for logging only.)
