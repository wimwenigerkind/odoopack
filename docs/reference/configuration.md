# Configuration

odoopack reads configuration from a config file and from environment variables, on top of built-in defaults.

## Config file

By default the CLI looks for `$HOME/.odoopack.yaml`. Override the path with the global `--config` flag.

## Keys and defaults

| Key | Default | Description |
| --- | --- | --- |
| `default_index_url` | `https://odoopack.dev` | Built-in registry tried last during resolution when no configured index matches. |
| `addons_path` | `addons/odoopack` | Directory addons are installed into. Also stored per project in the manifest. |
| `manifest` | `odoopack.json` | Manifest filename. |
| `lock` | `odoopack.lock` | Lockfile filename. |
| `auth` | (unset) | Registry tokens. Set through the `ODOOPACK_AUTH` environment variable. |
| `cache_dir` | `$XDG_CACHE_HOME/odoopack`, else `~/.cache/odoopack` | Directory of the shared [package store](../commands/cache.md). |
| `no_cache` | `false` | Install without the package store. Also the `--no-cache` flag. |

## Environment variables

Every key can be set through the environment by upper-casing its name:

| Variable | Key |
| --- | --- |
| `ODOOPACK_AUTH` | `auth` |
| `ODOOPACK_CACHE_DIR` | `cache_dir` |
| `ODOOPACK_NO_CACHE` | `no_cache` |
| `DEFAULT_INDEX_URL` | `default_index_url` |
| `ADDONS_PATH` | `addons_path` |
| `MANIFEST` | `manifest` |
| `LOCK` | `lock` |

`ODOOPACK_AUTH` holds a JSON object of bearer tokens keyed by registry host. See [Private registries](../guides/private-registries.md).
