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
go build -o eservctl ./cmd/eservectl
go test ./...                   # passes trivially — there are NO test files in the tree
go vet ./...
```

There is no linter/formatter config in the repo; `gofmt`/`go vet` are the only tooling.

**Running requires root.** Config paths are hardcoded (`/etc/eserved`, `/etc/epull`) and `InitConfigPath` calls `log.Fatalln` if it can't create the config directory. There is no env override.

Typical flow: start `eserved` (first run auto-generates `/etc/eserved/settings.json` with defaults, a CA, and a server cert **signed by that CA** — SANs = `eserver` + every non-loopback IPv4 on the host) → `eservctl token create` → `epull register -token <t> -server https://host:8080 -flavor <name> -stage <stage3file>`. Register pins the server CA automatically on first run (open `/api/v1/ca` → `/etc/epull/ca.crt`), so steady state is verified — no `-insecure` needed (the flag remains as a fallback when pinning isn't possible, e.g. an older server). Plain-https clients (portage) can verify the server cert by trusting the eserved CA in their system store.

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
- `/api/v1/ca` — open (no client cert): the eserved CA in PEM; epull pins it on first register and verifies against it after
- `/api/v1/provision` — 202 + `{job_id, binhost_url, flavor}`; switching flavor requires a fresh bearer token. `binhost_url` is per-flavor (`<base_binhost_url>/<flavor>`)
- `/api/v1/sync` — stores the client's portage-config tar.gz (64 MiB limit) at `/etc/eserved/sync/<flavor>/<cn>.tar.gz`, then applies it to the flavor chroot: canonical copy in `<chroot>/.eserved/` (via `os.Root`, symlinks/hardlinks/traversal rejected) and relative symlinks from `etc/portage/` into it. Requires the `X-Portage-Fingerprint` header and a provisioned flavor; records the fingerprint in `sync/<flavor>/fingerprint.json`
- `/api/v1/sync/check` — compares `X-Portage-Fingerprint` against the flavor's recorded fingerprint: 204 in sync, 409 (JSON `{flavor, fingerprint}`) when it differs or was never synced, 400 when the machine has no flavor
- `/api/v1/stages` — plain-text list of stage3 tarballs in the stage dir
- `/api/v1/jobs/stream` — **POST** (JSON `{job_id}`), returns a live event stream
- `/api/v1/binary` — GET (`?name=&arch=`), the server-hosted build of a binary (mTLS), for `epull selfupdate`
- `/api/v1/binary/manifest` — GET, its `{name, arch, sha256, size, uploaded_at}`
- `/api/v1/signing-key` — mTLS: the server's GPG public key (armored); epull imports it into the client's `/etc/epull/gnupg` (ultimate trust) during register
- `/pkgs/` — the binhost: `http.FileServer` over `repo_base` (prefix-stripped), **no client cert** (portage can't do mTLS); the `binaries/` subtree is hidden from it (stays mTLS-only at `/api/v1/binary`)

**The job stream is real SSE.** `/api/v1/jobs/stream` sends `event:` = the event type (`progress`, `output`, `done`, `error`, `cancelled`), `data:` = the message, one blank-terminated event per job line, flushed individually — the same events that go into the jsonl log (multi-line messages become multiple `data:` lines, per spec). Both clients parse it with the shared reader `protocol.EachSSEEvent` (cmd/epull/api/jobs.go, cmd/eservectl/admin/jobs.go): `done`/`cancelled` are clean stops, `error` is failure — and colorize for display locally (`Colorize` in `internal/protocol/job.go`); ANSI never appears on the wire.

### Server state

Everything is a JSON file under `/etc/eserved/` plus in-memory maps guarded by `sync.RWMutex`:

| File | Content |
|---|---|
| `settings.json` | auto-generated defaults on first run; the only real config (listen addr, chroot/stage paths, binhost URL) |
| `ca.crt`/`ca.key` | auto-generated ed25519 CA, 10 years |
| `server.crt`/`server.key` | server cert signed by the CA, 1 year, SANs = `eserver` + host IPv4s |
| `gnupg/` | the server's GPG home: a passphraseless ed25519 signing key (`internal/gpg`); copied whole into each build chroot at build time |
| `sync/<flavor>/<cn>.tar.gz` | uploaded client portage configs; `sync/<flavor>/fingerprint.json` records the last synced config (fingerprint + who/when) |
| `flavors/<name>/` | the flavor's own portage config (plain files: `make.conf`, `binrepos.conf`, ...); the server's layer, applied under every client's config. The flavor `make.conf` carries the build env + GPG signing config — a flavor without it publishes **unsigned** gpkgs |
| `jobs/<id>.jsonl` | live job log — **deleted when the job finishes** |

Outside `/etc/eserved/`: `<repo_base>/<flavor>/` is the binhost (default `/srv/pkgs`): immutable `snapshot-<unix>/` dirs of built binpkgs + a live `Packages`/`Packages.gz` index at the flavor root, and `<repo_base>/binaries/<arch>/<name>` (+ `.json` manifest) are the server-hosted client binaries.

**Job registry** (cmd/eserved/jobs/jobs.go): in-memory only. **Flavored jobs run one at a time per flavor** (the chroot is shared): they wait in a buffered channel (`queueBuffer = 8`, a full queue rejects fast), served by a per-flavor worker goroutine; jobs with `flavor == ""` run immediately in fresh goroutines. `Finish` is idempotent: work functions may end their own job with a custom terminal event, and if they just return, the registry marks them `done` (`<kind> complete`). Panics are recovered in both run modes. The log file is deleted on `Finish` with only the terminal event kept in memory; a janitor goroutine (10 min tick) reaps finished jobs after 1 hour. After that, all history is gone — there is no persistence.

**Provisioning** (cmd/eserved/chroot/provision.go): stagefile must be a plain filename (path-traversal guard); extraction is `tar -xpf` into `<chroot_base>/<flavor>` and writes the marker `etc/eserved-stage3.json`; extraction is skipped when the marker exists (unlike the old `etc/` check, it can't be fooled by synced config). After (re-)extraction, `restoreSyncedConfig` re-applies the machine's portage config: re-linking the `etc/portage/` symlinks into the surviving `<chroot>/.eserved/` dir, or re-extracting from the stored sync archive if that dir was lost — so a re-provision does not force a client re-sync. Cross-dev (client gcc-machine arch ≠ server's) is rejected — see `chroot.IsGccMachineDiff`, which only compares the part before `-`.

**Flavor config layering** (cmd/eserved/chroot/flavorconfig.go): every portage-config mutation builds a staging dir as **flavor config → client sync archive on top → flavor `make.conf` last**. The client wins file-by-file, but the flavor always wins `make.conf` (the build environment is the server's call — the scaffolded one strips `binpkg-request-signature`, which a stock stage3 make.conf requests). `ApplySync` and the admin flavor-apply both go through this, under the per-flavor `flavorlock`. `ApplyFlavorToChroot` re-applies from the stored per-client archives (the last-synced client goes last, same as the last sync); the merged `.eserved/` dir is never used as a merge source.

**Build jobs** (cmd/eserved/chroot/build.go): started by admin (`/admin/v1/build/start`), run through the flavor queue as `kind = "build"`. They take the flavor lock (so a sync can't swap config mid-build), then run the build in a sandbox: **bwrap when it's on PATH** (`--new-session --die-with-parent --unshare-pid --unshare-ipc --unshare-uts --bind <chroot> / --dev-bind /dev /dev --proc /proc --bind /sys /sys --bind /etc/resolv.conf /etc/resolv.conf --bind /etc/hosts /etc/hosts --chdir /` — no bind-mounts into the chroot, nothing left behind), **else plain `chroot`** (bind-mounts `/dev`, `/proc`, `/sys`, `/etc/resolv.conf`, `/etc/hosts` into the chroot and unmounts again after; idempotent via `/proc/self/mountinfo`). **The `--bind <chroot> /` root bind is required**: without it bwrap silently builds an **empty root** (no bind error) and the final exec dies with `execvp /usr/bin/env: No such file or directory`. The inner command is `env -i PATH=/usr/sbin:/usr/bin:/sbin:/bin HOME=/root /usr/bin/emerge ...`. Then they ensure the chroot has a working gentoo repo: fresh `.eserved-repo-updated` marker (< 7 days) → use as-is; otherwise `emerge --sync` inside the chroot, falling back to `cp -a` of the host's `/var/db/repos/gentoo` (works with no chroot network; the marker records the update either way). The build itself is `emerge --buildpkg --usepkg=n --getbinpkg=n -j<build_threads> <atoms...>` streamed to the job — `--getbinpkg=n` is load-bearing: the synced client config carries the stage-baked official binrepo (`[gentoo-x86-64-v3]`), and the client keyring the synced make.conf points at doesn't exist inside the chroot, so any remote binpkg fetch there fails GPG verification. Atoms are validated against a strict regex (no shell metacharacters, no use flags). If the flavor layer has signing config (`flavors/<name>/make.conf`: the `binpkg-signing` feature + `BINPKG_GPG_SIGNING_*`; the key itself is the one in `/etc/eserved/gnupg`, copied into the chroot at build time), portage signs the gpkgs (signatures **embedded in the gpkg**: `metadata.tar.zst.sig` + `image.tar.zst.sig`) — a fresh flavor **without** a `flavors/<name>/` dir publishes **unsigned** gpkgs (caught live), so scaffold the dir (see the `build` flavor) and `flavor apply` before its first build. `PublishBinpkgs` (cmd/eserved/chroot/binhost.go) then copies the chroot's `var/cache/binpkgs` into `<repo_base>/<flavor>/snapshot-<unix>/`, rewrites the index's global-header `URI:` to the snapshot's absolute URL (the 3.0 client joins header `URI` — falling back to the sync-uri — with each entry's `PATH`), publishes live `Packages` + `Packages.gz` at the flavor root, and prunes to the newest 3 snapshots.

### Client (`epull`)

State in `/etc/epull/`: `settings.json` (server URL + cert/key paths), `<hostname>.crt`/`.key` written by register, and **`ca.crt` — the pinned server CA**. `register` = **CA pin** (if `ca.crt` is missing: fetch `/api/v1/ca` over a one-shot unverified TLS hop, check it parses, store it — every epull request after that verifies against it, so no `-insecure` in steady state) → identity (verified against the pinned CA) → mTLS client → **stage** (`-stage <file>`; falls back to the interactive prompt when omitted) → provision → streams the job → **signing key** (mTLS GET `/api/v1/signing-key`; imported into `/etc/epull/gnupg` with ultimate trust; adds `BINPKG_GPG_VERIFY_*` + `GPG_VERIFY_USER_DROP=""` to `/etc/portage/make.conf`) → upserts a portage `binrepos.conf` section (`sync-uri`, **`priority = 10000`** — above the stage3-catalyst's baked-in `[gentoo-x86-64-v3]` at 9999, which would otherwise tie and win, pulling official gpkgs that our keyring rightly rejects, `location = /var/cache/binhost/<flavor>`, **`verify-signature = true`**; if `binrepos.conf` is a **directory** — Gentoo's drop-in layout — it writes `binrepos.conf/eserved.conf` instead; the leave-alone decision compares all three keys, so user-added keys survive) → checks portage sync (prompts if the flavor drifted; non-fatal). `epull sync` (also after `provision`) does the same check standalone: `SyncPortage` fingerprints `/etc/portage` over `protocol.PortageSyncPaths` (shared with the server), `CheckSync` compares, `UploadPortage` uploads — `-y` skips the prompt. `epull selfupdate` downloads the server-hosted `epull` for the local arch over mTLS (`/api/v1/binary`), verifies size + sha256 against the manifest, and atomically replaces the running binary (temp file + rename next to it). Ctrl+C once while streaming cancels the job (epull stops cleanly, not with an error); twice exits 130.

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
- `epull register` takes `-stage <file>` for non-interactive use; without it it falls back to the interactive prompt (blocks without a tty; the final sync check swallows the EOF and only warns).
- `eserved` has only one flag (`-admin`); everything else (listen addr, paths) is in `settings.json`.
- `MachineEntry.Arch` (json `arch`) is never populated; only `Subarch`/`Profile` are.
- The binhost is **signed**: gpkgs carry GPG signatures **embedded in the gpkg tar** (`metadata.tar.zst.sig`, `image.tar.zst.sig`), made by the passphraseless ed25519 key in `/etc/eserved/gnupg` (generated with `%no-protection` by `internal/gpg`; the chroot copies that whole dir and signs at build time). Clients verify against the key epull pinned in `/etc/epull/gnupg` — the client `make.conf` gets `BINPKG_GPG_VERIFY_*` + `GPG_VERIFY_USER_DROP=""` (portage drops verify to `nobody` by default, which can't read the root-only keyring), and the binrepo section says `verify-signature = true`. Signing is gated by the `binpkg-signing` feature (portage 3.0.81); `openpgp-key-package` is still unused.
- `settings.json`'s `base_binhost_url` is baked into the published index (`URI:` header) and into the provision response, so it must be a URL the *clients* can reach (the LAN IP, not `127.0.0.1`), and changing it changes what clients fetch.
- For portage to actually consume the binhost, the **client's system trust store must trust the eserved CA** (`/etc/eserved/ca.crt` — drop it in the distro's local CA dir and run its update tool); until then the index fetch fails `CERTIFICATE_VERIFY_FAILED` and emerge silently falls back to source builds.
- Portage 3.0 binrepo fetch semantics (verified on 3.0.81): the client fetches `<sync-uri>/Packages` itself (tries `.gz` first) and stores fetched gpkg under the section's `location`; `sync-type` in `binrepos.conf` is a **dead key** in 3.0 (the sync-module machinery is for portage-tree sync only) — don't write it.
- `build_threads` is used (`-j` for emerge builds, 0 = unlimited); `per_user_threads` is an unused leftover field.
- The dev Go toolchain on the build VMs rejects a trailing `...` spread preceded by more fixed args than the function has non-variadic params (e.g. `exec.CommandContext(ctx, "chroot", dir, "env", "-i", ..., args...)` doesn't compile). Build the full arg slice and spread once.
- ROADMAP's **NOT GOING TO BE IMPLEMENTED** list: fail2ban/rate limits, reverse-proxy support, automatic Let's Encrypt — don't propose these. (Note `ClientIP` does read `X-Forwarded-For` for logging only.)
- Naming is settled: the admin CLI is `eservctl` — source dir `cmd/eservctl`, binary `/usr/local/bin/eservctl`, same spelling in both (9 chars, eserve+ctl; unlike `eserved`/`epull` there was a period of a mistyped 8-char variant butchered by the terminal — don't reintroduce it; the old 8-char binary in the VM's `/usr/local/bin` is gone). Build: `go build -o eservctl ./cmd/eservctl`.
- `ln -sf <target> <linkname>` where `<linkname>` is a symlink **to a directory** creates the link **inside** that directory — `rm` the symlink first (this bit us switching the host's `make.profile`).
- **Dirty VM stops tear recent writes**: a hard stop of the guest once left `/usr/local/bin/epull` as a 0-data file and `/root/eserve-src` truncated. `go build` refuses to overwrite a non-ELF output file — `rm` it first, rebuild, and re-deploy from the local tree (the source of truth).
- **eserved has no init script**: on the VMs it's started manually after boot (`setsid nohup /usr/local/bin/eserved -admin=true >/var/log/eserved.log 2>&1 </dev/null &`). Starting a second instance while one holds `:8080` fatals the new one on the bind — but the attempt **orphans the admin socket path** (old listener keeps running, `/run/eserved.sock` file vanishes → eservctl socket errors). Restart eserved cleanly instead.

## Handoff — 2026-08-31 (two sessions folded in; read this first)

The work of both sessions is **committed on local `main`** (this commit); the tree is clean. The VM at 192.168.122.200 holds the deployed state; `/root/eserve-src` mirrors the local tree. What's still open is small and listed below.

### User instructions (keep honoring)
- KISS: least comments possible; stay on task; big picture; don't derail.
- C/Go probe tools on the VM are justified when the failing process lives ~3 ms (a shell can't observe it).
- Work gets verified on the VM, not just locally.
- When told to stop: save state here; no new long-running work.

### Done and verified (all on the VM)
- **bwrap root cause**: the "impossible" bwrap failure (eserved-spawned `execvp /usr/bin/env: No such file or directory`, manual runs fine) was never a kernel/lineage issue — the bwrap argv was missing the root bind `--bind <chroot> /` (an earlier dev-bind edit dropped it); bwrap then legally builds an **empty root** and the final exec fails with no bind error. Fixed in `cmd/eserved/chroot/build.go` (root bind added; the experimental shared-pipe stderr hack was a red herring and is reverted). Verified: builds complete in the bwrap sandbox, no host-mount leaks, no leftovers.
- **Repo signing, end to end**: passphraseless key (`/etc/eserved/gnupg`) → signed gpkgs (embedded `.sig`) → epull pins the key (`/etc/epull/gnupg`, ultimate trust) → portage 3.0.81 verifies over CA-verified https and installs. **Negative test**: different keyring → `GnuPG verification failed`, package rejected; positive control re-passed.
- **epull CA pinning**: new open route `/api/v1/ca`; register pins the CA on first run (no `-insecure`, no system-store entry); impostor-CA negative control fails as expected.
- **`epull register -stage <file>`**: non-interactive register works (used live all session).
- **Fixes**: `binrepos.conf` as a **directory** (Gentoo drop-in layout) — epull writes `binrepos.conf/eserved.conf`; server sync-apply now resolves file↔dir type conflicts between layers (`clearTypeMismatch` in `cmd/eserved/chroot/sync.go`, later layer wins). **Nil-map panic** in `buildStagedConfig` for flavors **without** a `flavors/<name>/` dir (fresh flavors) — `copyFlavorConfig` now returns an empty map; previously a sync on a fresh flavor panicked (`http: panic serving … assignment to entry in nil map`) and the client saw EOF.
- **USE flags**: `USE="spell"` → `nano-9.1-1` (360936 B, hunspell symbols); `USE="-spell"` → `nano-9.1-2` (**the slot changes with spell**, 356808 B, no hunspell) — two different signed gpkgs published under `/srv/pkgs/build/`. Chroot make.conf restored after.
- **Second flavor `gnome`**: created (stage3 extraction ~3 s), machine `eserver` re-identified → `machines.json` records profile `default/linux/amd64/23.0/desktop/gnome` (machine `epuller` still plasma), client config synced into `/srv/build/gnome/.eserved/` (incl. `profile/` drop-in); session ended with the gnome build about to start — finished next session (see below).
- **gnome-flavor E2E finished (day 2)**: created `flavors/gnome/{make.conf,binrepos.conf}` (mirroring `build`) and applied via `/admin/v1/flavor/apply` — without the flavor layer the first gnome build published an **unsigned** gpkg (the `binpkg-signing` feature comes from the flavor make.conf). Builds now pass `--getbinpkg=n`: the synced client config carries the stage-baked `[gentoo-x86-64-v3]` binrepo and the client keyring path from the synced make.conf doesn't exist inside the chroot, so remote binpkg fetches there would always fail GPG verification. Result: signed `nano-9.1-2` gpkg (both embedded `.sig`: good signature vs the eserved key), published under `/srv/pkgs/gnome/snapshot-1788172377/`, consumed by host portage from the `[gnome]` section (https + CA + GPG all verified). Host `make.profile` restored to base `default/linux/amd64/23.0`.
- **binrepo priority**: epull now writes `priority = 10000` (the catalyst bakes `[gentoo-x86-64-v3]` at 9999 into the stage3 `/etc/portage`; a tie meant the official repo wins and its Gentoo-signed gpkgs fail against our keyring — correctly refused, but it broke consumption too); the leave-it-alone check now compares the priority key.
- **Naming settled** (see gotcha above): the admin CLI is `eservctl` everywhere; the old 8-char binary is gone.
- **Torn writes from a dirty stop** (see gotcha above): a hard stop of the VM tore the just-deployed `/usr/local/bin/epull` and `/root/eserve-src`; a full redeploy from the local tree + rebuild fixed it before the E2E continued.

### Left to do (small; in order)
1. **The other VM** (the `epuller` machine, the client box): still runs a pre-priority epull and its `eserved.conf` sections are at priority 9999 — redeploy epull there + sed the priority to 10000 when that box is used again.
2. **Gnome flavor sync drift**: flavor `gnome`'s stored sync is still the gnome-profile config while the host `make.profile` is back at base — don't casually `epull sync -y` from the eserver (it would push base config into the gnome flavor); re-sync deliberately when the gnome flavor gets used again.
3. **Push to origin** when convenient — everything here is committed locally on `main` (forge: `git.fedesito.me/fedes1to/eserve`).

### Environment cheat-sheet
- VM: `sshpass -p drcwfxevs ssh root@192.168.122.200` (kernel 6.18.43, portage 3.0.81 at /usr/lib/python3.14/site-packages/portage, gpg 2.4.8, bwrap 0.11.2 at /usr/bin/bwrap). eserved is started manually after boot (no init script — see gotchas); check with `pgrep -x eserved`; log /var/log/eserved.log (truncated on each restart). The CA trust for portage lives in VM `/etc/ssl/certs/` (`eserved-ca.crt` + hash symlink `dc079d7b.0` — don't remove); `/etc/epull/ca.crt` is epull's pinned CA (steady-state verify).
- **Deploy flow**: edit local → `rsync -a --delete /home/fed/Projects/repos/eserve/ /tmp/eserve-full/` (the dev box `/tmp` is wiped on every restart — recreate the dir first) → in `/tmp/eserve-full`: `gofmt -l .` + `go build ./...` + `go vet ./...` must be clean → `tar cf - . | ssh … 'rm -rf /root/eserve-src && mkdir /root/eserve-src && tar xf - -C /root/eserve-src && cd /root/eserve-src && go build -o /usr/local/bin/eserved ./cmd/eserved && go build -o /usr/local/bin/epull ./cmd/epull && go build -o /usr/local/bin/eservctl ./cmd/eservctl'` → `pkill -x eserved; setsid nohup /usr/local/bin/eserved -admin=true >/var/log/eserved.log 2>&1 </dev/null &`.
- **`pkill -x`, never `-f`** — a `-f` pattern matches the ssh shell itself (killed a session once, exit 255).
- Tokens: `eservctl token create` prints the token on **stderr** (log line `New token: X`); one-shot; a flavor **switch** needs a fresh token.
- VM `/etc/ssl/certs/` has `eserved-ca.crt` + hash symlink `dc079d7b.0` (portage's https trust — don't remove); `/etc/epull/ca.crt` is epull's pinned CA (steady-state verify).
- The dev box shell has a broken zoxide hook (`cd` can fail with `__zoxide_z: command not found`) — use absolute paths or the `cwd` parameter.
- The local tree is **not** corrupted: a previous session's "corrupted `cmd/eservctl`" was a typo'd path (8-char vs 9-char spelling); the git object store is healthy.
