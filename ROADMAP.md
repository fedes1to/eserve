# ROADMAP for eserve
Simple roadmap to check progress/features

## DONE
- /api/v1/identity
- mTLS security model
- unix socket for eservectl model
- token creation logic
- /api/v1/provision
- /api/v1/sync + /api/v1/sync/check (portage config fingerprint + chroot setup)
- real SSE job stream (event:/data: framing, replay then live, cancel)
- per-flavor job queue (one job at a time per flavor)
- per-flavor portage config (server layer under the client's, make.conf always server's)
- chroot build jobs (repo ensure + emerge --buildpkg, streamed)
- binhost publishing (/pkgs/<flavor>/ snapshots + Packages index)
- CA-signed server cert (binhost verifiable by clients that trust the eserved CA)
- binary hosting + epull selfupdate
- eservectl job/build/flavor/binary commands
- bubblewrap build sandbox (root bind required) + chroot fallback
- repo signing end to end (passphraseless server key, embedded gpkg sigs, epull key bootstrap, portage verifies)
- CA pinning for epull (open /api/v1/ca, no -insecure in steady state)
- register -stage (non-interactive register)
- binrepo priority 10000 (beats the stage-baked official repos)
- machine/token delete + health endpoint + OpenRC service (pidfile, boot start)
- second VM (epuller) on the full steady state, signed consumption there
- build flavor builds the init system (user-tuned package.use drop-ins)
- crossdev support (cross.conf per flavor, toolchain sdk via crossdev, target binpkgs from the sysroot emerge wrapper)
- flavor-bound tokens required to join an existing flavor (identity + flavor switch), clear 400 naming the fix
- switch-token refund when the job never started or failed before the switch was written
- startup tightening of the legacy state file modes (settings.json, sync/**, jobs/, the keys)
- the chroot's make.profile follows the client's profile (admin override, else the last provisioner/syncer), so binpkgs are actually usable

## TODO (atm)
- almost everything cuh

## PLANNED
- other distros/os' maybe?

## NOT GOING TO BE IMPLEMENTED
- fail2ban / rate limits
- OOTB support for reverse proxies
- automatic LE cert support
