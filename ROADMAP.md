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

## TODO (atm)
- almost everything cuh

## PLANNED
- crossdev support
- other distros/os' maybe?

## NOT GOING TO BE IMPLEMENTED
- fail2ban / rate limits
- OOTB support for reverse proxies
- automatic LE cert support
