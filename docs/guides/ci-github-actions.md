# CI with GitHub Actions

The [setup-odoopack](https://github.com/wimwenigerkind/setup-odoopack) action installs the odoopack CLI on a runner, adds it to the `PATH`, and can configure registry authentication.

## Public registry

```yaml
jobs:
  install:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: wimwenigerkind/setup-odoopack@v0.1.0
        with:
          version: latest
      - run: odoopack install
```

## Private registry

Provide the registry URL and a token stored as a repository secret. The action exports `ODOOPACK_AUTH` scoped to that host for the following steps.

```yaml
jobs:
  install:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: wimwenigerkind/setup-odoopack@v0.1.0
        with:
          version: latest
          registry-url: https://registry.example.com
          token: ${{ secrets.ODOOPACK_TOKEN }}
      - run: odoopack install
```

## Inputs

| Input | Default | Description |
| --- | --- | --- |
| `version` | `latest` | Release tag to install (`1.2.0`, `v1.2.0`, or `latest`). |
| `token` | `''` | API token for a private registry. Requires `registry-url`. |
| `registry-url` | `''` | Base URL the token authenticates against. |

Commit `odoopack.json` and `odoopack.lock` so that `odoopack install` in CI reproduces the exact locked versions.
