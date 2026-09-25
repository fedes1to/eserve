# eserve
Make having a build server actually fun

A **prototype** Gentoo build server. Three binaries:

- `eserved` — the server: provisions per-machine Gentoo chroots ("flavors"), builds packages in them, publishes the results as a signed binhost
- `epull` — the client: registers a machine, provisions its flavor, syncs its portage config, consumes the binhost, self-updates
- `eservectl` — the admin CLI, talking to `eserved` over a unix socket

Gentoo-only for now. Pure Go, stdlib only, no external dependencies.

## Trust model

An admin mints a single-use bearer token over the unix socket. `epull` spends it once to POST a CSR and get a 1-year client cert signed by the server's CA; every request after that is mTLS, and the server checks the cert's CN + fingerprint against `machines.json`. Revocation is sticky. `epull` pins the server CA on first register, so steady state needs no `-insecure`.

A token can be bound to a CN and/or a flavor: `eservectl token create -cn <name> -flavor <name>`. **Joining a flavor that already exists needs a token bound to that flavor** — a flavor exists once it has a provisioned chroot, a machine on it, or a `flavors/<name>/` config dir, and an unbound token can only start a brand-new one (a fresh chroot nobody else is on). So adding a machine to an existing flavor is:

```sh
eservectl token create -flavor build
epull register -token <t> -server https://host:8080 -flavor build -stage <stage3file>
```

The same rule covers switching a machine onto an existing flavor (`eservectl token create -cn <name> -flavor <name>`), and the refusal is a 400 that names the command. A CN-bound token is the recovery path for a machine that lost its certs.

## The loop

1. `eservectl token create` — one-shot token (`-flavor <name>` to join a flavor that already exists)
2. `epull register -token <t> -server https://host:8080 -flavor <name> -stage <stage3file>` — pins the CA, identifies, provisions the flavor, imports the server's signing key
3. `epull sync` — uploads the client's portage config; the server layers it under the flavor's own config
4. `eservectl build start -flavor <name> -package <cat/pkg>` — admin-triggered build in a bwrap sandbox (plain chroot fallback), streamed live
5. the server publishes signed gpkgs to the binhost at `/pkgs/<flavor>/`, which clients consume with portage
6. `epull selfupdate` — replaces itself with the server-hosted build

## Build

```sh
go build ./...
go build -o eserved ./cmd/eserved
CGO_ENABLED=0 go build -o epull ./cmd/epull   # static, so it runs in musl chroots too
go build -o eservectl ./cmd/eservectl
go test ./...
go vet ./...
```

Running requires root: config paths are hardcoded (`/etc/eserved`, `/etc/epull`). The first `eserved` start auto-generates `settings.json`, a CA and a server cert signed by it.

## License

[PolyForm Noncommercial License 1.0.0](LICENSE.md).

## AI
up to [1ceb5e0](https://git.fedesito.me/fedes1to/eserve/commit/1ceb5e0932fba01b71265e09be132925bc42c040) there was no AI involved, after that commit there was

See [AGENTS.md](AGENTS.md) for the architecture, the gotchas and the conventions.
