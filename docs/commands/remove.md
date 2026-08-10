# remove

Remove an addon from the requirements, the lockfile, and the addons path.

```console
$ odoopack remove acme/health-check
removed acme/health-check@19.0.1.0.0
```

## Usage

```
odoopack remove <addon>
```

## Arguments

- `<addon>`: the addon name to remove. Any `@version` suffix is ignored; removal is by name.

## Behavior

1. Loads `odoopack.json`. If the addon is not a current requirement, it errors.
2. Removes the requirement from `odoopack.json`.
3. Deletes the addon from `odoopack.lock` and recomputes the content hash.
4. Deletes the addon's directory from the addons path.
