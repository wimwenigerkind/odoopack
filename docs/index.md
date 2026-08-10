# odoopack

odoopack is a package manager for Odoo addons. It resolves addon dependencies from one or more registries, records exact versions in a lockfile, and installs the addons into your project's addons path.

## How it works

- A project is described by a manifest, `odoopack.json`, which lists the addons you require and the registries to resolve them from.
- Resolved versions are pinned in a lockfile, `odoopack.lock`, so installs are reproducible.
- Addons are materialized into an addons path (default `addons/odoopack/`) that you point Odoo at.

```console
$ odoopack init
$ odoopack require acme/health-check@19.0.1.0.0
$ odoopack install
```

## Next steps

- [Getting started](getting-started/index.md): install the CLI and create your first project.
- [Commands](commands/index.md): reference for every command.
- [Guides](guides/index.md): private registries and CI.
- [Reference](reference/index.md): manifest, lockfile, and configuration formats.
