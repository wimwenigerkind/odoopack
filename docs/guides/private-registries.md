# Private registries

Registries that serve private addons require a bearer token. odoopack sends this token both when looking up an addon and when downloading its zip distribution.

## Providing a token

Tokens are configured through the `ODOOPACK_AUTH` environment variable, which holds a JSON object mapping registry hosts to tokens:

```json
{
  "bearer": {
    "registry.example.com": "<token>",
    "https://registry.example.com": "<token>"
  }
}
```

```console
$ export ODOOPACK_AUTH='{"bearer":{"registry.example.com":"<token>"}}'
$ odoopack install
```

A token is matched against a registry by host. Both the bare host (`registry.example.com`) and the scheme-qualified form (`https://registry.example.com`) are accepted as keys, and either matches a registry configured with that URL.

Because the value is keyed by host, a single `ODOOPACK_AUTH` can hold tokens for several registries at once.

## Pointing a project at a private registry

Add the registry as an index in the manifest, then require addons as usual:

```console
$ odoopack index add company https://registry.example.com
$ odoopack require acme/health-check@19.0.1.0.0
```

See the [`index`](../commands/index-command.md) command and the [Configuration](../reference/configuration.md) reference.

## Security

- Never commit `ODOOPACK_AUTH` or raw tokens to the repository.
- In CI, store the token as a secret and pass it through the environment. See [CI with GitHub Actions](ci-github-actions.md).
