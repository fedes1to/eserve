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

## TODO (atm)
- bubblewrap "bindings"
- repo signing (binhost is unsigned for now)
- almost everything cuh

## PLANNED
- crossdev support
- other distros/os' maybe?

## NOT GOING TO BE IMPLEMENTED
- fail2ban / rate limits
- OOTB support for reverse proxies
- automatic LE cert support
