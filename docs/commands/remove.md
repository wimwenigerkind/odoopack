# remove

Remove an addon from the requirements, the lockfile, and the addons path.

```console
$ odoopack remove acme/health-check
removed acme/health-check@19.0.1.0.0 (2 package(s) removed, 0 installed)
```

## Usage

```
odoopack remove <addon>
```

## Arguments

- `<addon>`: the addon name to remove. Any `@version` suffix is ignored; removal is by name.

## Behavior

1. Loads `odoopack.json`. If the addon is not a current requirement, it errors.
2. Locks the remaining requirements, keeping every locked package as it is. It asks an index only for a package whose lockfile entry cannot be kept, for example one without an `index`.
3. Refuses while another locked package depends on the addon, and names those packages:

   ```console
   $ odoopack remove acme/base-tools
   cannot remove acme/base-tools: acme/health-check depend(s) on it
   ```

4. Like [install](install.md#the-odoopack-marker), it refuses an invalid `addons_dir` or a non-empty one without the `.odoopack` marker.
5. Removes the requirement from `odoopack.json`, and the addon and every dependency that no remaining package needs from `odoopack.lock`, and recomputes the content hash.
6. Deletes the directories of the removed packages from the addons path.
