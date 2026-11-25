# Configuration

odoopack reads configuration from a config file and from environment variables, on top of built-in defaults.

## Config file

By default the CLI looks for `$HOME/.odoopack.yaml`. Override the path with the global `--config` flag.

## Keys and defaults

| Key | Default | Description |
| --- | --- | --- |
| `default_index_url` | `https://odoopack.dev` | Built-in default registry, which [`init`](../commands/init.md) adds to a new manifest as the index `default`. Resolution searches only the indexes of the manifest. See [index](../commands/index-command.md#resolution-order). |
| `addons_dir` | `addons/odoopack` | Directory [`init`](../commands/init.md) writes into a new manifest as `addons_dir`. A project then uses the one in its manifest. |
| `manifest` | `odoopack.json` | Manifest filename. |
| `lock` | `odoopack.lock` | Lockfile filename. |
| `auth` | (unset) | Registry tokens. Set through the `ODOOPACK_AUTH` environment variable. |
| `cache_dir` | `$XDG_CACHE_HOME/odoopack`, else `~/.cache/odoopack` | Directory of the shared [package store](../commands/cache.md). |
| `no_cache` | `false` | Install without the package store. Also the `--no-cache` flag. |

## Environment variables

Every key can be set through the environment as `ODOOPACK_` followed by its name in upper case. Variables without the prefix, such as `MANIFEST` or `LOCK`, are ignored, so that a generic variable of a CI job does not change what odoopack reads.

| Variable | Key |
| --- | --- |
| `ODOOPACK_AUTH` | `auth` |
| `ODOOPACK_CACHE_DIR` | `cache_dir` |
| `ODOOPACK_NO_CACHE` | `no_cache` |
| `ODOOPACK_FROZEN_LOCKFILE` | `frozen_lockfile` |
| `ODOOPACK_DEFAULT_INDEX_URL` | `default_index_url` |
| `ODOOPACK_ADDONS_DIR` | `addons_dir` |
| `ODOOPACK_MANIFEST` | `manifest` |
| `ODOOPACK_LOCK` | `lock` |

`ODOOPACK_AUTH` holds a JSON object of bearer tokens keyed by registry host. See [Private registries](../guides/private-registries.md).
