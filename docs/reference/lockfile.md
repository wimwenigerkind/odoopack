# Lockfile (`odoopack.lock`)

The lockfile pins the exact distribution of every requirement so that installs are reproducible. It is generated and maintained by the CLI. Do not edit it by hand, but do commit it to version control.

## Example

```json
{
  "content_hash": "sha256:1976680cba9b714f50256eb050427a4b9d0a7ab12e5ed82fe104d6d797ca2f35",
  "packages": {
    "acme/health-check": {
      "version": "19.0.1.0.0",
      "dist": {
        "type": "zip",
        "url": "https://registry.example.com/registry/v1/zipball/<id>/<reference>",
        "reference": "<git-sha>",
        "shasum": "sha256:<hex>"
      }
    }
  }
}
```

## Fields

| Field | Type | Description |
| --- | --- | --- |
| `content_hash` | string | `sha256:` hash of the requirements, indexes, and locked packages. Used to detect when the lockfile is stale relative to the manifest. |
| `packages` | object | Map of addon name to the locked package. |

### `packages.<name>`

| Field | Type | Description |
| --- | --- | --- |
| `version` | string | The resolved version. |
| `dist.type` | string | Distribution type: `zip` or `git`. |
| `dist.url` | string | Download URL (zip) or clone URL (git). |
| `dist.reference` | string | Git reference (commit SHA) the distribution was built from. |
| `dist.shasum` | string | `sha256:` checksum of the zip. Verified on install; a mismatch aborts the install. |

## Staleness

When you change `require` or `indexes` in the manifest, the lockfile's `content_hash` no longer matches. [`install`](../commands/install.md) detects this, re-resolves against the registry, and rewrites the lockfile before installing. [`update`](../commands/update.md) always re-resolves.
