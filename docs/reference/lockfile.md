# Lockfile (`odoopack.lock`)

The lockfile pins the exact distribution of every requirement so that installs are reproducible. It is generated and maintained by the CLI. Do not edit it by hand, but do commit it to version control.

## Example

```json
{
  "format_version": 1,
  "content_hash": "sha256:1976680cba9b714f50256eb050427a4b9d0a7ab12e5ed82fe104d6d797ca2f35",
  "packages": {
    "acme/health-check": {
      "version": "19.0.1.0.0",
      "dist": {
        "type": "zip",
        "url": "https://registry.example.com/registry/v1/zipball/<id>/<reference>",
        "reference": "<git-sha>",
        "digest": "sha256:<hex>"
      },
      "index_url": "https://registry.example.com"
    }
  }
}
```

## Fields

| Field | Type | Description |
| --- | --- | --- |
| `format_version` | integer | Version of the lockfile format, currently `1`. A lockfile without it is version 1. odoopack refuses a lockfile with a higher version than it knows and asks you to upgrade odoopack; it always writes the current version. Adding an optional field does not raise it. Not part of `content_hash`, so a lockfile does not become stale when it gains the field. Releases before `format_version` wrote the same number as `format`; odoopack still reads it and replaces it with `format_version` the next time it writes the lockfile. |
| `content_hash` | string | `sha256:` hash of the requirements, the URL and type of each index, the `odoo_series` of the manifest, and the locked packages. Used to detect when the lockfile is stale relative to the manifest. Index names are not part of it, so renaming an index does not make the lockfile stale. |
| `packages` | object | Map of addon name to the locked package. |

### `packages.<name>`

| Field | Type | Description |
| --- | --- | --- |
| `version` | string | The resolved version. |
| `dist.type` | string | Distribution type: `zip` or `git`. |
| `dist.url` | string | Download URL (zip) or clone URL (git). |
| `dist.reference` | string | Git reference (commit SHA) the distribution was built from. |
| `dist.digest` | string | `sha256:` digest of the zip. Verified on install; a mismatch aborts the install. |
| `direct` | boolean | `true` when `odoopack.json` requires the package, `false` when it is only a dependency of another package. |
| `depends` | array | Packages this package depends on. They are locked too. |
| `external_modules` | array | Module names this package depends on that no index provides, such as the modules that ship with Odoo. |
| `index_url` | string | URL of the index the package was resolved from, the built-in default registry included, with scheme and host in lower case and no trailing slash. Later resolutions ask this index only, as long as the manifest has an index with this URL, whatever its name. Missing in lockfiles written by earlier releases; such a package is resolved as if it were new. |
| `index` | string | Name of the index the package was resolved from, written by releases before `index_url` existed. odoopack still reads it and replaces it with `index_url` the next time it writes the package. Rename an index only after the lockfile has been written with `index_url`, for example by `odoopack update`. |

## Unknown keys

odoopack writes the lockfile from scratch and does not keep keys it does not know. Keys are matched exactly, and a key that appears twice in one object is an error. A newer odoopack that adds an optional key writes it again on its next write. Keys of an index in the manifest that odoopack does not know are not part of `content_hash`.

## Renamed keys

Releases before format version 1 was frozen wrote `format` instead of `format_version`, `dist.shasum` instead of `dist.digest`, `external` instead of `external_modules` and `registry` instead of `index_url`. odoopack still reads them and writes the new keys the next time it writes the lockfile. The renaming does not change `content_hash`, so such a lockfile is not stale and `install --frozen-lockfile` keeps working.

## Resolution

Every requirement in `odoopack.json` and every dependency of a locked package is a requirement on a package:

- A requirement in `odoopack.json` has the version you gave it: an exact version (`19.0.1.0.0`), a series (`19.0`), or a branch build (`dev-19.0`).
- A dependency has the `odoo_series` of the manifest. It is met by any release of that series and by any branch build, since Odoo depends on a module, not on a version of it.

A package required in `odoopack.json` is resolved by that requirement, even when a dependency reaches it first; a package that is only a dependency is resolved to the newest release of the series. Every other requirement on the package must then be met by the version it was resolved to. So `"acme/base-tools": "19.0.1.0.0"` in `odoopack.json` locks that version for every package that depends on `acme/base-tools`, even when the registry has a newer `19.0` release.

When the version does not meet another requirement on the package, resolution fails instead of letting the first requirement win, and names both requirements, each by the chain of packages it comes from:

```console
conflicting requirements for acme/base-tools: the manifest requires 19.0.1.0.0 and resolved 19.0.1.0.0, but acme/other -> acme/health-check requires 19.0.2.0.0
```

Nothing is written when resolution fails. Dependencies carry only the series for now, and a requirement in `odoopack.json` must already match the series, so in practice this applies once dependencies can ask for a version.

[`update <addon>`](../commands/update.md) and [`remove`](../commands/remove.md) keep a locked package as it is when its locked version still meets its requirements, comes from an index the manifest still has (by URL), and pins its content. [`install`](../commands/install.md) on a stale lockfile, [`require`](../commands/require.md), and `update` without an argument resolve every package again.

## Staleness

When you change `require`, the URL or type of an index, or `odoo_series` in the manifest, the lockfile's `content_hash` no longer matches. [`install`](../commands/install.md) detects this, re-resolves against the registry, and rewrites the lockfile before installing, or fails with `--frozen-lockfile`. [`update`](../commands/update.md) always re-resolves.
