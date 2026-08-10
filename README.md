# odoopack

A package manager for Odoo addons. odoopack resolves addon dependencies from one or more registries, pins exact versions in a lockfile, and installs the addons into your project's addons path.

## Features

- Declarative project manifest (`odoopack.json`) listing required addons and registries.
- Reproducible installs via a lockfile (`odoopack.lock`) with checksum verification.
- Multiple registries per project, plus a built-in default.
- Private registries via bearer-token authentication.
- First-class CI support through the [setup-odoopack](https://github.com/wimwenigerkind/setup-odoopack) GitHub Action.

## Install

### Homebrew (macOS/Linux)

```console
$ brew tap wimwenigerkind/homebrew-tap
$ brew install --cask odoopack
```

### Docker

```console
$ docker run --rm -v "$PWD:/work" -w /work ghcr.io/wimwenigerkind/odoopack:latest install
```

### Binary releases

Download the archive for your platform from the [releases](https://github.com/wimwenigerkind/odoopack/releases), extract the `odoopack` binary, and place it on your `PATH`.

## Quickstart

```console
$ odoopack init
$ odoopack require acme/health-check@19.0.1.0.0
$ odoopack install
```

Point Odoo at the addons path (default `addons/odoopack/`) and you are done. Commit `odoopack.json` and `odoopack.lock` so that `odoopack install` reproduces the exact versions everywhere.

## Documentation

Full documentation lives in [`docs/`](docs/index.md):

- [Getting started](docs/getting-started/index.md)
- [Commands](docs/commands/index.md)
- [Guides](docs/guides/index.md): private registries, CI
- [Reference](docs/reference/index.md): manifest, lockfile, configuration

## License

[MIT](LICENSE)
