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
| `odoo_series` | string | Required. Odoo series the project targets, two numbers without leading zeros such as `19.0`. Written by [`init`](../commands/init.md) from `--odoo`. Every requirement and dependency is resolved for this series. |
| `require` | object | Map of package name to constraint. A package name is `vendor/name` or `name` of lowercase letters, digits, `_` and `-`. The constraint takes one of the forms in [Constraints](#constraints). |
| `indexes` | array | Registries to resolve addons from, in the order they are searched. Only these are searched: there is no implicit registry. [`init`](../commands/init.md) adds the built-in default registry as the index `default`. |
| `addons_dir` | string | Required. Directory addons are installed into; [`init`](../commands/init.md) writes `addons/odoopack`. A relative path of letters, digits, `.`, `_` and `-`, with `/` as separator also on Windows, and without a `.` or `..` part, a leading or trailing `/`, a backslash or a colon, so that it means the same directory on every system. It must also resolve, through symlinks, to a directory strictly inside the project. odoopack owns this directory: it writes a `.odoopack` marker into it and refuses to clear a non-empty directory without the marker. See [install](../commands/install.md#the-odoopack-marker). |
| `extra` | object | Optional. Free-form data for other tools, such as deployment settings. odoopack keeps it as it is and never warns about it. |

### `indexes[]`

| Field | Type | Description |
| --- | --- | --- |
| `name` | string | Unique name of the index, 1 to 64 lowercase letters, digits, `_` and `-`, starting with a letter or digit. Used in messages and by the `index` command. Resolution does not depend on it: a package is locked to the URL of its index. `default` has no special meaning. |
| `url` | string | Registry base URL: `https://` or `http://`, a host and an optional path, in its one canonical form: scheme and host in lower case, no default port (`:443`, `:80`), no trailing `/` and no percent escapes. Two indexes must not have the same URL. Credentials, a query and a fragment are refused; tokens belong in `ODOOPACK_AUTH`, see [Private registries](../guides/private-registries.md). |
| `type` | string | Index type. `registry` is currently the only type. An unknown type is refused with a request to upgrade odoopack, instead of skipping the index and resolving the addon elsewhere. |

Indexes are tried in their order during resolution, and only an index that does not have the addon passes it on to the next one. Put a private registry before a public one, so that a name both have resolves to the private addon. See [index](../commands/index-command.md#resolution-order).

## Constraints

A constraint in `require` takes exactly one of three forms. Each keeps its meaning in every release of format version 1.

| Form | Example | Meaning |
| --- | --- | --- |
| Release | `19.0.1.0.0` | Exactly this release. Five numbers separated by `.`, without leading zeros; the first two are the Odoo series and must match `odoo_series`. |
| Series | `19.0` | Must be `odoo_series` itself. The newest release of the series, resolved when the lockfile is written and pinned there until the package is resolved again, by `odoopack update` or by `odoopack install` on a stale lockfile. `require` writes this form when no version is given. |
| Branch build | `dev-19.0` | The head of a branch, pinned in the lockfile by its commit. The branch name takes `A-Z`, `a-z`, `0-9`, `.`, `_`, `/` and `-`, must not contain `..`, start with `-` or `/`, or end with `/` or `.lock`. A branch named after a series, such as `dev-18.0`, must match `odoo_series`; any other branch name, such as `dev-feature/x`, is not checked against it. |

Anything else is refused, such as `latest`, `^19.0`, `>=19.0.1.0.0`, `19.0.*` or `19.0.1`. The characters `^`, `~`, `<`, `>`, `=`, `*`, `|`, `,` and whitespace are reserved for a range syntax a later format version may add.

A dependency of a locked package is met by any release of `odoo_series` and by any branch build, since Odoo depends on a module, not on a version of it. See [Resolution](lockfile.md#resolution).

## Reserved keys

These top-level keys are reserved for later releases. odoopack does not use them yet and keeps them like any other unknown key, but they may get a meaning, so do not use them for data of your own; use `extra` instead.

| Key | Intended meaning |
| --- | --- |
| `require_dev` | Packages for development and tests only, like `require` |
| `conflict` | Packages that must not be installed together with this project |
| `replace` | Packages this project provides itself |
| `scripts` | Commands to run on install or update |
| `config` | Settings of odoopack for this project |
| `description` | A description of the project |
| `license` | The license of the project as an SPDX expression |

## Validation

Every command checks the whole manifest when it loads it and stops with every problem it finds, before it resolves or installs anything. Keys are matched exactly, so `"Require"` is an unknown key, not `require`. A key that appears twice in one object is an error, instead of the last one silently winning. A command also refuses to write a manifest that would not pass, so a mistake in its arguments never leaves an invalid manifest behind. `odoopack doctor` additionally checks what depends on the project directory and the lockfile.

## Unknown keys

odoopack keeps every key it does not know, at the top level and in an index, and writes it back unchanged when a command such as `require` or `index add` rewrites the manifest. A key that a newer odoopack added therefore survives a write by an older one. Each command warns about every unknown key, since it is usually a typo:

```console
! odoopack.json: unknown key "requires" is kept but ignored by this odoopack
```

Put data of your own under `extra` instead, which odoopack never warns about.

odoopack writes the known keys in the order of the table above and the unknown keys after them, sorted by name, indented by two spaces and with a trailing newline. It writes to a temporary file and renames it, so the manifest is never left half written. A new optional key does not raise `format_version`; only a change an older odoopack would misread does.
