# Private registries

Registries that serve private addons require a bearer token. odoopack sends this token both when looking up an addon and when downloading its zip distribution. Every request also carries `User-Agent: odoopack/<version>`, so a registry can tell which odoopack release is asking.

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

When `ODOOPACK_AUTH` is set but is not a JSON object of this form, for example because of a typo in `bearer`, odoopack stops with an error instead of resolving without a token.

## Pointing a project at a private registry

Add the registry as an index in the manifest, then require addons as usual:

```console
$ odoopack index add company https://registry.example.com
$ odoopack require acme/health-check@19.0.1.0.0
```

`index add` puts the new index first, so it is searched before the public default registry that [`init`](../commands/init.md) added. The default registry is only asked for addons your indexes answer with "not found" (HTTP 404), with or without a token. Any other answer stops the resolution with an error instead: a 401 or 403, for example from an expired token, a 5xx, a timeout or an unavailable private registry. To resolve from your own registries only, remove it:

```console
$ odoopack index remove default
```

See the [`index`](../commands/index-command.md) command and the [Configuration](../reference/configuration.md) reference.

## Security

- Never commit `ODOOPACK_AUTH` or raw tokens to the repository.
- In CI, store the token as a secret and pass it through the environment. See [CI with GitHub Actions](ci-github-actions.md).
- A token is only sent over `https`. Plain `http` is allowed for `localhost` and loopback addresses, for a registry under development; for any other `http://` index that has a token, odoopack stops with an error that names the URL.
- When a registry redirects to another scheme, host or port, the token is not sent along, and a redirect from `https` to `http` is refused.
