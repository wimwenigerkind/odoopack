# index

Manage the registry indexes that addons are resolved from. Indexes are stored in `odoopack.json`.

Aliases: `idx`.

## Usage

```
odoopack index add <name> <url> [--type registry] [--append]
odoopack index remove <name>
odoopack index list
```

## Subcommands

### `index add <name> <url>`

Add or replace an index. The name takes lowercase letters, digits, `_` and `-`. The URL is `https://` or `http://` with a host and an optional path; a URL with credentials, a query or a fragment is refused, since the manifest is committed and tokens belong in `ODOOPACK_AUTH`. `index add` writes the URL in its canonical form, so `https://Registry.Example.com:443/` is stored as `https://registry.example.com`. A new index is searched first, before the indexes already there, so that a private registry wins over the public one; with `--append` it is searched last. An index with the same name is replaced at its position.

```console
$ odoopack index add company https://registry.example.com
added index company https://registry.example.com (registry)
```

| Flag | Default | Description |
| --- | --- | --- |
| `--type` | `registry` | Index type. `registry` is currently the only supported type. |
| `--append` | `false` | Search the new index after the others instead of first. |

### `index remove <name>`

Remove a configured index. Aliases: `rm`, `delete`.

```console
$ odoopack index remove company
removed index company
```

### `index list`

List configured indexes in the order they are searched. Aliases: `ls`.

```console
$ odoopack index list
```

## Resolution order

When resolving an addon, the indexes declared in the manifest are tried in their order. odoopack moves on to the next index only when an index answers that it does not have the addon (HTTP 404). Any other answer stops the resolution with an error that names the index: a 401 or 403 (for example an expired token), a 5xx, a timeout, a network error or an invalid response. A transient failure of a private registry therefore never installs a public addon with the same name.

Only the indexes of the manifest are searched. [`init`](init.md) adds the built-in default registry (`default_index_url`, see [Configuration](../reference/configuration.md)) as the index `default`; remove it with `odoopack index remove default` to resolve from your own registries only. While it is listed after your indexes, only a 404 from them reaches it, so a private registry that fails never hands a private addon name over to the public registry.

The index each package was resolved from is recorded in [`odoopack.lock`](../reference/lockfile.md) by its URL, not its name. Later resolutions of a locked package, including its transitive dependencies, ask that index only, so an index can be renamed without changing the lockfile. If the manifest no longer has an index with that URL, the package is resolved as if it were new.
