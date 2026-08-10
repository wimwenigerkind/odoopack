# index

Manage the registry indexes that addons are resolved from. Indexes are stored in `odoopack.json`.

Aliases: `idx`.

## Usage

```
odoopack index add <name> <url> [--type registry]
odoopack index remove <name>
odoopack index list
```

## Subcommands

### `index add <name> <url>`

Add or replace an index. The URL must include a scheme and host.

```console
$ odoopack index add company https://registry.example.com
added index company https://registry.example.com (registry)
```

| Flag | Default | Description |
| --- | --- | --- |
| `--type` | `registry` | Index type. `registry` is currently the only supported type. |

### `index remove <name>`

Remove a configured index. Aliases: `rm`, `delete`.

```console
$ odoopack index remove company
removed index company
```

### `index list`

List configured indexes. Aliases: `ls`.

```console
$ odoopack index list
```

If no index points at the built-in default registry, it is shown as an implicit `default` entry.

## Resolution order

When resolving an addon, indexes declared in the manifest are tried in alphabetical order by name. If none match and no manifest index already points at it, the built-in default registry (`default_index_url`) is tried last. See [Configuration](../reference/configuration.md).
