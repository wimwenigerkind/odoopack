# Manifest (`odoopack.json`)

The manifest describes your project: which addons it requires and which registries to resolve them from. It is created by [`init`](../commands/init.md) and edited by the `require`, `remove`, and `index` commands. Commit it to version control.

## Example

```json
{
  "format_version": 1,
  "name": "my-project",
  "odoo_series": "19.0",
  "require": {
    "acme/health-check": "19.0.1.0.0"
  },
  "indexes": [
    {
      "name": "company",
      "url": "https://registry.example.com",
      "type": "registry"
    },
    {
      "name": "default",
      "url": "https://odoopack.dev",
      "type": "registry"
    }
  ],
  "addons_dir": "addons/odoopack"
}
```

## Fields

| Field | Type | Description |
| --- | --- | --- |
| `$schema` | string | Optional. URL of a JSON Schema for editor completion and validation. odoopack keeps it and does not read it. |
| `format_version` | integer | Version of the manifest format, currently `1`. A manifest without it is version 1. odoopack refuses a manifest with a higher version than it knows and asks you to upgrade odoopack; it always writes the current version. Adding an optional field does not raise it. Not part of the lockfile's `content_hash`. |
| `name` | string | Project name. |
| `odoo_series` | string | Required. Odoo series the project targets, two numbers such as `19.0`. Written by [`init`](../commands/init.md) from `--odoo`. Every requirement and dependency is resolved for this series. |
| `require` | object | Map of package name to constraint. A package name is `vendor/name` or `name` of lowercase letters, digits, `_` and `-`. A constraint is an exact release (`19.0.1.0.0`), a series (`19.0`, the newest release of the series) or a branch build (`dev-19.0`), without surrounding spaces. |
| `indexes` | array | Registries to resolve addons from, in the order they are searched. Only these are searched: there is no implicit registry. [`init`](../commands/init.md) adds the built-in default registry as the index `default`. |
| `addons_dir` | string | Directory addons are installed into. Defaults to `addons/odoopack`. Must be a relative path that resolves, also through symlinks, to a directory strictly inside the project, not the project directory itself. odoopack owns this directory: it writes a `.odoopack` marker into it and refuses to clear a non-empty directory without the marker. See [install](../commands/install.md#the-odoopack-marker). |
| `extra` | object | Optional. Free-form data for other tools, such as deployment settings. odoopack keeps it as it is and never warns about it. |

### `indexes[]`

| Field | Type | Description |
| --- | --- | --- |
| `name` | string | Unique name of the index, 1 to 64 lowercase letters, digits, `_` and `-`, starting with a letter or digit. Used in messages and by the `index` command. Resolution does not depend on it: a package is locked to the URL of its index. `default` has no special meaning. |
| `url` | string | Registry base URL: `https://` or `http://`, a host and an optional path. Credentials, a query and a fragment are refused; tokens belong in `ODOOPACK_AUTH`, see [Private registries](../guides/private-registries.md). |
| `type` | string | Index type. `registry` is currently the only type. An unknown type is refused with a request to upgrade odoopack, instead of skipping the index and resolving the addon elsewhere. |

Indexes are tried in their order during resolution, and only an index that does not have the addon passes it on to the next one. Put a private registry before a public one, so that a name both have resolves to the private addon. See [index](../commands/index-command.md#resolution-order).

## Validation

Every command checks the whole manifest when it loads it and stops with every problem it finds, before it resolves or installs anything. A command also refuses to write a manifest that would not pass, so a mistake in its arguments never leaves an invalid manifest behind. `odoopack doctor` additionally checks what depends on the project directory and the lockfile.

## Unknown keys

odoopack keeps every key it does not know, at the top level and in an index, and writes it back unchanged when a command such as `require` or `index add` rewrites the manifest. A key that a newer odoopack added therefore survives a write by an older one. Each command warns about every unknown key, since it is usually a typo:

```console
! odoopack.json: unknown key "requires" is kept but ignored by this odoopack
```

Put data of your own under `extra` instead, which odoopack never warns about.

odoopack writes the known keys in the order of the table above and the unknown keys after them, sorted by name. A new optional key does not raise `format_version`; only a change an older odoopack would misread does.

## Renamed keys

Releases before format version 1 was frozen wrote `odoo` instead of `odoo_series` and `addons_path` instead of `addons_dir`. odoopack still reads both and writes the new keys the next time it writes the manifest.

## Indexes as an object

Releases before format version 1 was frozen wrote `indexes` as an object of indexes by name, searched them in alphabetical order, and then searched the built-in default registry (`default_index_url`) unless an index was named `default` or had its URL. odoopack still reads that form: it lists the indexes in alphabetical order and adds the default registry as the last index named `default` where those releases searched it, so resolution stays the same. The next write stores the array.

Since the default registry then becomes part of `content_hash`, a lockfile written before is stale once. `odoopack install --frozen-lockfile` fails until the lockfile is written again. `odoopack install` writes it, but resolves every package again from the index it is locked to, so a requirement on a series such as `19.0` can move to a newer release; review the lockfile diff before you commit it.
