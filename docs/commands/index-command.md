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

When resolving an addon, the indexes declared in the manifest are tried in alphabetical order by name. odoopack moves on to the next index only when an index answers that it does not have the addon (HTTP 404). Any other answer stops the resolution with an error that names the index: a 401 or 403 (for example an expired token), a 5xx, a timeout, a network error or an invalid response. A transient failure of a private registry therefore never installs a public addon with the same name.

After them, the built-in default registry (`default_index_url`, see [Configuration](../reference/configuration.md)) is tried last, under the name `default`, unless the manifest already has an index named `default` or one with the same URL. It is a fallback like any other index: only a 404 from the indexes before it reaches it, so a private registry that fails never hands a private addon name over to the public registry.

The index each package was resolved from is recorded in [`odoopack.lock`](../reference/lockfile.md) by its URL, not its name. Later resolutions of a locked package, including its transitive dependencies, ask that index only, so an index can be renamed without changing the lockfile. If the manifest no longer has an index with that URL, the package is resolved as if it were new.
