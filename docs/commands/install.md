# install

Install every required addon from the lockfile.

```console
$ odoopack install
```

Aliases: `i`.

## Usage

```
odoopack install [--frozen-lockfile] [--no-cache]
```

## Flags

| Flag | Default | Description |
| --- | --- | --- |
| `--frozen-lockfile` | `false` | Fail instead of resolving again when `odoopack.lock` is missing or stale. Same as `ODOOPACK_FROZEN_LOCKFILE=1`. Use it in CI. |
| `--no-cache` | `false` | Install without the shared [package store](cache.md). Same as `ODOOPACK_NO_CACHE=1`. |

## Behavior

1. Loads `odoopack.json`. If there are no requirements, it prints `no addons installed` and exits.
2. Loads `odoopack.lock`. If the lockfile is **stale** (its content hash no longer matches the current requirements, indexes and `odoo` series), the requirements are re-resolved against the registry and the lockfile is rewritten before installing. With `--frozen-lockfile`, a missing or stale lockfile is an error instead, and nothing is written.
3. Checks `addons_path` before touching anything: it must be a relative path that resolves, also through symlinks, to a directory strictly inside the project, and must not be the project directory itself.
4. Removes the entire addons path and reinstalls all locked packages concurrently. Zip distributions come from the shared [package store](cache.md) and are only downloaded when the store does not have them yet.

Because the addons path is rebuilt from scratch, it always reflects exactly what is locked. Do not place unmanaged files there; they will be removed on install.

## The `.odoopack` marker

On install odoopack writes a file named `.odoopack` into the addons path to mark the directory as its own. `install` and `update` only clear the addons path when it does not exist, is empty, or contains this marker. A non-empty directory without the marker is refused with an error, so that a mistyped `addons_path` cannot wipe your own modules: empty the directory or choose another `addons_path`. `require` and `remove` refuse such a directory as well.

An addons path filled by an older odoopack version has no marker yet. It is adopted, and gets the marker, when it holds nothing but directories named after packages in `odoopack.lock`, as it was before this run or after resolving it again; a file, a symlink or any other directory in it keeps the refusal.

## Reproducibility

Commit both `odoopack.json` and `odoopack.lock`. A fresh checkout followed by `odoopack install` reproduces the exact same addon versions, verified by checksum for zip distributions.

## Frozen lockfile

`odoopack install --frozen-lockfile` installs exactly what `odoopack.lock` holds and never resolves again. When the lockfile is missing, or no longer matches `odoopack.json`, it fails before it changes anything:

```console
$ odoopack install --frozen-lockfile
odoopack.lock is out of date with odoopack.json; run 'odoopack install' without --frozen-lockfile (or 'odoopack update') and commit the lockfile
```

Use it in CI, so that a manifest change committed without its lockfile fails the build instead of installing versions nobody reviewed. `require`, `update` and `remove` always write the lockfile and do not take the flag.

A lockfile written by an odoopack before the `odoo` series became part of the content hash is still accepted, so upgrading odoopack does not fail a frozen install. Such a lockfile does not notice a change of the series until odoopack writes it again: run `odoopack update` after changing the series.
