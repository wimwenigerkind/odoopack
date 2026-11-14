# update

Re-resolve requirements against the registry and reinstall.

```console
$ odoopack update
updated 3 addon(s)
```

Aliases: `up`.

## Usage

```
odoopack update [addon]
```

## Arguments

- `[addon]`: optional. When given, only that addon and the packages it depends on are re-resolved. It must already be present in `odoopack.json`.

## Flags

| Flag | Default | Description |
| --- | --- | --- |
| `--no-cache` | `false` | Install without the shared [package store](cache.md). Same as `ODOOPACK_NO_CACHE=1`. |

## Behavior

- **Without an argument:** recomputes the whole lockfile from the current requirements and indexes, saves it, and reinstalls all addons.
- **With an argument:** re-resolves that addon and its dependency tree; every other locked package keeps its locked version as long as it still satisfies its requirements. New dependencies are added to `odoopack.lock`, dependencies nothing needs any more are dropped, and the addon stays marked as `direct`. Then it installs every package whose locked version or distribution changed and deletes the directories of dropped packages.

Since requirements are pinned to explicit versions, `update` picks up a change only when the version a requirement points to now resolves to a different distribution (for example a moved branch build), or when a requirement was edited in `odoopack.json`.
