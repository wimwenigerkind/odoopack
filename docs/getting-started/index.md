# Getting started

## Install the CLI

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

Download the archive for your platform from the [GitHub releases](https://github.com/wimwenigerkind/odoopack/releases), extract the `odoopack` binary, and place it on your `PATH`.

### GitHub Actions

In CI, use the [setup-odoopack](https://github.com/wimwenigerkind/setup-odoopack) action to install the CLI and configure registry authentication. See [CI with GitHub Actions](../guides/ci-github-actions.md).

## Create a project

Initialize a manifest in the current directory:

```console
$ odoopack init
Initialized project "odoopack"
```

This writes an `odoopack.json` with sensible defaults, including an addons path of `addons/odoopack/`.

## Add an addon

Requirements are always pinned to an explicit version. Use `name@version`, where the version must exactly match a version advertised by the registry (for example a release version or a branch build):

```console
$ odoopack require acme/health-check@19.0.1.0.0
Added acme/health-check@19.0.1.0.0
```

`require` resolves the addon, updates `odoopack.json` and `odoopack.lock`, and installs that addon immediately.

## Install everything

To install every requirement from the lockfile, for example on a fresh checkout or in CI:

```console
$ odoopack install
```

`install` rebuilds the addons path from scratch, so it always reflects exactly what is locked.
