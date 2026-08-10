# install

Install every required addon from the lockfile.

```console
$ odoopack install
```

Aliases: `i`.

## Usage

```
odoopack install
```

## Behavior

1. Loads `odoopack.json`. If there are no requirements, it prints `no addons installed` and exits.
2. Loads `odoopack.lock`. If the lockfile is **stale** (its content hash no longer matches the current requirements and indexes), the requirements are re-resolved against the registry and the lockfile is rewritten before installing.
3. Removes the entire addons path and reinstalls all locked packages concurrently.

Because the addons path is rebuilt from scratch, it always reflects exactly what is locked. Do not place unmanaged files there; they will be removed on install.

## Reproducibility

Commit both `odoopack.json` and `odoopack.lock`. A fresh checkout followed by `odoopack install` reproduces the exact same addon versions, verified by checksum for zip distributions.
