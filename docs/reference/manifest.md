# Manifest (`odoopack.json`)

The manifest describes your project: which addons it requires and which registries to resolve them from. It is created by [`init`](../commands/init.md) and edited by the `require`, `remove`, and `index` commands. Commit it to version control.

## Example

```json
{
  "name": "my-project",
  "require": {
    "acme/health-check": "19.0.1.0.0"
  },
  "indexes": {
    "company": {
      "url": "https://registry.example.com",
      "type": "registry"
    }
  },
  "addons_path": "addons/odoopack"
}
```

## Fields

| Field | Type | Description |
| --- | --- | --- |
| `name` | string | Project name. |
| `require` | object | Map of addon name to pinned version. The version is an exact string, not a range. |
| `indexes` | object | Map of index name to `{ url, type }`. Registries to resolve addons from. |
| `addons_path` | string | Directory addons are installed into. Defaults to `addons/odoopack`. |

### `indexes.<name>`

| Field | Type | Description |
| --- | --- | --- |
| `url` | string | Registry base URL, including scheme and host. |
| `type` | string | Index type. `registry` is currently the only supported type. |

Indexes are tried in alphabetical order by name during resolution. If none match, the built-in default registry is tried last. See [Configuration](configuration.md).
