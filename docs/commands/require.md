# require

Add an addon dependency, pin it in the lockfile, and install it.

```console
$ odoopack require acme/health-check@19.0.1.0.0
added acme/health-check@19.0.1.0.0 (1 package(s) installed, 0 removed)
```

## Usage

```
odoopack require <addon>[@<version>]
```

Aliases: `add`, `req`.

## Arguments

- `<addon>[@<version>]`: the package name and, after `@`, a [constraint](../reference/manifest.md#constraints): an exact release (`19.0.1.0.0`), a series (`19.0`) or a branch build (`dev-19.0`). Without a version, `require` writes the `odoo_series` of the manifest, so the package follows the newest release of the series:

  ```console
  $ odoopack require acme/health-check
  added acme/health-check@19.0.2.1.0 (1 package(s) installed, 0 removed)
  ```

  The manifest then holds `"acme/health-check": "19.0"`, and the lockfile pins the release it resolved to. There is no range syntax.

## Flags

| Flag | Default | Description |
| --- | --- | --- |
| `--no-cache` | `false` | Install without the shared [package store](cache.md). Same as `ODOOPACK_NO_CACHE=1`. |

## Behavior

1. Loads `odoopack.json` (run [`init`](init.md) first if it is missing).
2. Resolves the addon through the configured [indexes](index-command.md).
3. Resolves all requirements again, including the new one and its dependencies. A [conflict](../reference/lockfile.md#resolution) stops here, before anything is written.
4. Adds the requirement to `odoopack.json`.
5. Writes the resolved versions, distribution types, URLs, references, checksums, and index URLs to `odoopack.lock` and recomputes its content hash.
6. Installs every package that is new or whose locked version or distribution changed, and deletes the directories of packages that are no longer locked. Packages that did not change are left as they are.

To add many addons at once, run `require` per addon, or edit `odoopack.json` and run [`install`](install.md).
