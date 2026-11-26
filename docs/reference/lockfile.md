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
      "module": "acme_health_check",
      "dist": {
        "type": "zip",
        "url": "https://registry.example.com/registry/v1/zipball/<id>/<reference>",
        "reference": "<git-sha>",
        "digest": "sha256:<hex>"
      },
      "direct": true,
      "index_url": "https://registry.example.com"
    }
  }
}
```

## Fields

| Field | Type | Description |
| --- | --- | --- |
| `format_version` | integer | Version of the lockfile format, currently `1`. A lockfile without it is version 1. odoopack refuses a lockfile with a higher version than it knows and asks you to upgrade odoopack; it always writes the current version. Adding an optional field does not raise it. Not part of `content_hash`, so a lockfile does not become stale when it gains the field. |
| `content_hash` | string | `sha256:` hash of what the manifest resolves the packages from. Used to detect when the lockfile is stale relative to the manifest. See [Content hash](#content-hash). |
| `packages` | object | Map of addon name to the locked package. |

### `packages.<name>`

| Field | Type | Description |
| --- | --- | --- |
| `version` | string | The resolved version. |
| `module` | string | Directory the package installs to inside `addons_dir`, the technical name of its Odoo module: lowercase letters, digits and `_`. odoopack sets it when it locks the package, currently by replacing `/` and `-` in the package name with `_`, and from then on takes it from here: install, update and remove never derive it again. Two packages with the same module are refused. |
| `dist.type` | string | Distribution type: `zip` or `git`. |
| `dist.url` | string | Download URL (zip) or clone URL (git), absolute and exactly as the registry returned it. It may point at another host than `index_url`, such as a CDN or a git host. A token from `ODOOPACK_AUTH` is chosen by the host of this URL, so another host only gets one when `ODOOPACK_AUTH` has a token for it, and a token is only sent over `https`. A zip URL must use `https`, with or without a token, so that nobody on the way sees which packages a project installs; plain `http` is allowed for localhost only, for a registry under development. A git URL must use `https` or `ssh`. The URL must be stable and free of secrets, since the lockfile is committed: user info (a token or password), a query and a fragment are refused, so a registry cannot put a signed, expiring URL into it. Only an ssh user such as `git@` is allowed. The content is checked against `dist.digest` or `dist.reference`, wherever it comes from. |
| `dist.reference` | string | Git reference (commit SHA) the distribution was built from. |
| `dist.digest` | string | `sha256:` digest of the zip. Verified on install; a mismatch aborts the install. |
| `direct` | boolean | `true` when `odoopack.json` requires the package, `false` when it is only a dependency of another package. |
| `depends` | array | Packages this package depends on. They are locked too. |
| `external_modules` | array | Module names this package depends on that no index provides, such as the modules that ship with Odoo. |
| `index_url` | string | URL of the index the package was resolved from, exactly as the manifest writes it. Later resolutions ask this index only, as long as the manifest has an index with this URL, whatever its name. A package without it is resolved as if it were new. |

## Unknown keys

odoopack writes the lockfile indented by two spaces, with a trailing newline and through a temporary file it renames. Keys are matched exactly, and a key that appears twice in one object is an error.

A key odoopack does not know in a package or in its `dist` stays as long as odoopack keeps the package as it is, for example through `update` of another package or `remove`. So a newer odoopack can add a key such as a signature or a size, which an older one could not compute again, and the older one does not erase it in a team that runs both. When a package is resolved again, its entry is written from scratch and the key is gone, since it described the old entry. Such a key never makes odoopack install a package again.

A key odoopack does not know at the top level is dropped: it may describe the whole lockfile, which every write changes. A key of an index in the manifest that odoopack does not know is not part of `content_hash`.

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

## Content hash

`content_hash` covers the manifest only, so that any tool can compute it from `odoopack.json` without odoopack:

1. Take this JSON object from the manifest:
   - `odoo_series`: the string as written.
   - `require`: the object as written, `{}` when the manifest has none.
   - `indexes`: an array with one object `{"type": ..., "url": ...}` per index, the URL exactly as the manifest writes it, sorted by `url` and then by `type`, `[]` when the manifest has none. The manifest only takes the [canonical form](manifest.md#indexes) of a URL, so there is nothing to normalize.
2. Serialize it in the canonical form of [RFC 8785](https://www.rfc-editor.org/rfc/rfc8785) (JSON Canonicalization Scheme).
3. Take the SHA-256 of those bytes and write it as `sha256:` followed by the lowercase hex digest.

For example, a manifest with `"odoo_series": "19.0"`, `"require": {"acme/health-check": "19.0"}` and the one index `https://registry.example.com` hashes the bytes

```json
{"indexes":[{"type":"registry","url":"https://registry.example.com"}],"odoo_series":"19.0","require":{"acme/health-check":"19.0"}}
```

The name and position of an index, keys odoopack does not know, `name`, `addons_dir`, `extra` and `format_version` are not part of it: none of them changes what a package resolves to, since a package is locked to the URL of its index. The locked packages are not part of it either; `dist.digest` and `dist.reference` pin their content.

Since the position of an index is not part of it, moving a private index in front of a public one keeps the lockfile fresh and changes only packages that are not locked yet. A locked package stays with its `index_url` until it is resolved again, for example by `odoopack update`.

### How the hash may change

A key that a later release makes part of resolution, such as `require_dev`, joins the hashed object only when the manifest has it. A manifest without the key keeps its hash, so upgrading odoopack does not make its lockfile stale. Any other change to these steps raises the `format_version` of the lockfile, so that an older odoopack refuses the lockfile instead of computing another hash and calling it stale.

## Staleness

When you change `require`, the URL or type of an index, or `odoo_series` in the manifest, the lockfile's `content_hash` no longer matches. A lockfile that lacks a package `require` names is stale as well, whatever its hash. [`install`](../commands/install.md) detects this, re-resolves against the registry, and rewrites the lockfile before installing, or fails with `--frozen-lockfile`. [`update`](../commands/update.md) always re-resolves.
