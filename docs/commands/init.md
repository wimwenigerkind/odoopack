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
| `-n`, `--name` | `odoopack` | Project name written to the manifest. |

## Result

The generated manifest contains the project name, an empty `require` map, an empty `indexes` map, and the default `addons_path` (`addons/odoopack`). See [Manifest](../reference/manifest.md).
