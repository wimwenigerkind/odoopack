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

- `[addon]`: optional. When given, only that addon is re-resolved and reinstalled. It must already be present in `odoopack.json`.

## Behavior

- **Without an argument:** recomputes the whole lockfile from the current requirements and indexes, saves it, and reinstalls all addons.
- **With an argument:** re-resolves that one addon, updates its entry and the content hash in `odoopack.lock`, and reinstalls only that addon.

Since requirements are pinned to explicit versions, `update` picks up a change only when the version a requirement points to now resolves to a different distribution (for example a moved branch build), or when a requirement was edited in `odoopack.json`.
