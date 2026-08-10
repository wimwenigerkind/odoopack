# list

List the required addons and whether each is currently installed in the addons path.

```console
$ odoopack list
┌───────────────────┬─────────────┬───────────┐
│ Name              │ Version     │ Installed │
├───────────────────┼─────────────┼───────────┤
│ acme/health-check │ 19.0.1.0.0  │ yes       │
└───────────────────┴─────────────┴───────────┘
```

Aliases: `l`.

## Usage

```
odoopack list
```

## Behavior

Reads requirements from `odoopack.json` and reports, for each, the pinned version and whether its directory exists in the addons path. It does not contact any registry.
