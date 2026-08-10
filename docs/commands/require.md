# require

Add an addon dependency, pin it in the lockfile, and install it.

```console
$ odoopack require acme/health-check@19.0.1.0.0
Added acme/health-check@19.0.1.0.0
```

## Usage

```
odoopack require <addon>@<version>
```

Aliases: `add`, `req`.

## Arguments

- `<addon>@<version>`: the addon name and an explicit version, separated by `@`. The version is **required**; there is no default and no version-range syntax. The value must exactly match a version advertised by the registry, for example a release version (`19.0.1.0.0`) or a branch build (`dev-19.0`).

## Behavior

1. Loads `odoopack.json` (run [`init`](init.md) first if it is missing).
2. Resolves the addon through the configured [indexes](index-command.md).
3. Adds the requirement to `odoopack.json`.
4. Writes the resolved version, distribution type, URL, reference, and checksum to `odoopack.lock` and recomputes its content hash.
5. Installs the single addon into the addons path.

To add many addons at once, run `require` per addon, or edit `odoopack.json` and run [`install`](install.md).
