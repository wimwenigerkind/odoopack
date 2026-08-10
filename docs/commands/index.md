# Commands

Every command operates on the manifest (`odoopack.json`) and lockfile (`odoopack.lock`) in the current directory.

| Command | Aliases | Description |
| --- | --- | --- |
| [`init`](init.md) | | Initialize a new odoopack project. |
| [`require`](require.md) | `add`, `req` | Add an addon dependency and install it. |
| [`install`](install.md) | `i` | Install all required addons from the lockfile. |
| [`update`](update.md) | `up` | Re-resolve requirements against the registry and reinstall. |
| [`remove`](remove.md) | | Remove an addon from requirements. |
| [`list`](list.md) | `l` | List required addons and their install status. |
| [`index`](index-command.md) | `idx` | Manage registry indexes. |

## Global flags

| Flag | Default | Description |
| --- | --- | --- |
| `--config` | `$HOME/.odoopack.yaml` | Path to the config file. |

See [Configuration](../reference/configuration.md) for config keys and environment variables that apply to all commands.
