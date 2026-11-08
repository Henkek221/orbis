# Licensing

Orbis is meant to be built on. Use it, change it, ship things made with it,
commercially too. The one rule: **always credit the author, Henrik Hellpap.**

## The rule

Anything you publish or distribute that contains, is derived from, or was
substantially built with Orbis code must credit the author by name, with the
link:

> Based on [Orbis](https://github.com/Henkek221/orbis) by Henrik Hellpap

Renaming, forking or rebranding does not change this: the credit names the
person, Henrik Hellpap, not only the project name.

Put it where people see credits: your README, docs, a credits or about screen,
or the release notes. If you copy Orbis files, also keep [`LICENSE`](LICENSE),
[`NOTICE`](NOTICE) and the `SPDX-License-Identifier` / copyright header at the
top of each file.

This is not a favour we ask: it is the condition of the
[Apache License 2.0](LICENSE) under which the code is released. Section 4(d)
requires every redistribution and derivative work to carry the contents of
[`NOTICE`](NOTICE), and that notice is the credit to Henrik Hellpap.

## What the license covers

Every file in this repository is released under Apache-2.0, copyright 2026
Henrik Hellpap, **except** the ones below. That includes the CLI (`cmd/orbis`),
the toolkit (`pkg/orbis`), the R3D decoder (`pkg/fly/r3d`), the GLB, USD(Z) and
Gaussian splat writers (`pkg/fly/exp`), the Minecraft writer (`pkg/mc`), the
browser UI (`internal/server`), and the promo sources (`promo/`).

## What it does not cover

- **Inherited code.** These files come from
  [retroplasma/flyover-reverse-engineering](https://github.com/retroplasma/flyover-reverse-engineering)
  and [christiangda's fork](https://github.com/christiangda/flyover-reverse-engineering),
  in some cases modified for Orbis. Their authors published no license file, so
  Orbis cannot grant one for them. Credit those projects too, and treat these
  files as "all rights reserved by their authors" until they license them.
  Orbis's own changes to them are offered under Apache-2.0 as far as they can be
  separated.

- `cmd/auth/main.go`
- `cmd/export-obj/main.go`
- `cmd/parse-c3m/main.go`
- `cmd/parse-c3mm/main.go`
- `pkg/bin/bin.go`
- `pkg/fly/bootstrap.go`
- `pkg/fly/c3m/c3m.go`
- `pkg/fly/c3m/codec/edgebreaker.go`
- `pkg/fly/c3m/codec/huffman.go`
- `pkg/fly/c3mm/c3mm.go`
- `pkg/fly/c3mm/codec/lzma.go`
- `pkg/fly/c3mm/util.go`
- `pkg/fly/exp/obj.go`
- `pkg/mps/ResourceManifest.pb.go`
- `pkg/mps/auth.go`
- `pkg/mps/auth/auth.go`
- `pkg/mps/auxiliary.go`
- `pkg/mps/bootstrap.go`
- `pkg/mps/cache.go`
- `pkg/mps/config/config.go`
- `pkg/mth/mth.go`
- `pkg/oth/oth.go`
- `pkg/web/web.go`
- `proto/ResourceManifest.proto`
- `proto/gen.sh`
- `scripts/center_scale_obj.js`
- `scripts/get_config.sh`
- `scripts/get_config_macos.sh`

- **Apple's data.** Everything Orbis downloads or exports, and the renders in
  `docs/images/`, `docs/launch/` and `promo/video/public/screens/`, is map data
  and imagery from Apple and subject to Apple's terms. Do not redistribute it or
  ship it in a product. See the README.
- **Third parties.** Code under `vendor/`, the sound files under
  `promo/video/public/audio/` (see its `LICENSES.md`), Leaflet and Lucide (loaded
  from unpkg), and the glTF sample asset in `internal/server/static/assets/`
  keep their own licenses.

## For coding agents

See [`AGENTS.md`](AGENTS.md). It states the same rule in the form agents follow.
