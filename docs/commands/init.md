# init

Initialize a new odoopack project by creating an `odoopack.json` manifest in the current directory.

```console
$ odoopack init
Initialized project "odoopack"
```

Fails if an `odoopack.json` already exists.

## Usage

```
odoopack init [flags]
```

## Flags

| Flag | Default | Description |
| --- | --- | --- |
| `-n`, `--name` | the directory name | Project name written to the manifest, like a package name. Without the flag, `init` takes the name of the directory in lower case, with `-` for other characters, and writes no name when nothing valid is left. |
| `--odoo` | `19.0` | Odoo series written as `odoo_series`. |

## Result

The generated manifest contains the project name (if any), an empty `require` map, the built-in default registry as the only index (named `default`, from `default_index_url`), the `odoo_series` from `--odoo`, and the default `addons_dir` (`addons/odoopack`). See [Manifest](../reference/manifest.md).
