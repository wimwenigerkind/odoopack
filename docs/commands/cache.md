# cache

Manage the shared package store that installs go through.

## Usage

```
odoopack cache dir
odoopack cache prune [--all]
```

## The package store

Every zip distribution with a `sha256` checksum in the lockfile is extracted once into the store, at `<cache dir>/store/<sha256>/`. Projects get the addon as hard links to the files in the store, so the same addon version takes disk space once, however many projects install it, and a second install downloads nothing.

- **Read-only files.** Files in the store are read-only. Because projects hard link them, the installed addon files are read-only too: an edit in one project would otherwise change the store and every other project. Patch an addon in its own repository, or install without the store.
- **Other volumes.** A hard link cannot cross volumes. When the addons path is on another volume than the cache directory, for example a Docker volume, files are copied instead and the store only saves the download.
- **What is left out.** Git distributions and zip distributions without a checksum are installed directly, as before.

To install without the store, pass `--no-cache` to [`install`](install.md), [`update`](update.md) or [`require`](require.md), or set `ODOOPACK_NO_CACHE=1`. The addon is then downloaded and extracted straight into the project, with writable files.

## Subcommands

### `cache dir`

Print the cache directory.

```console
$ odoopack cache dir
/home/me/.cache/odoopack
```

The directory is `ODOOPACK_CACHE_DIR` when set, otherwise `$XDG_CACHE_HOME/odoopack`, otherwise `~/.cache/odoopack`. See [Configuration](../reference/configuration.md).

### `cache prune`

Remove the packages no project uses any more.

```console
$ odoopack cache prune
removed 3 package(s), freed 1.2 MiB
```

A package counts as used while a project still links to its files, so a package becomes unused once every project that installed it has removed it, switched to another version, or reinstalled it with `--no-cache`. No list of projects is kept.

`prune` also removes what an interrupted install left behind, once it is older than an hour.

| Flag | Default | Description |
| --- | --- | --- |
| `--all` | `false` | Remove every package, including those still in use. Projects keep their files; the next install downloads the packages again. |

Do not run `prune` while an install is running: it may remove a package the install is about to link, and the install then fails. Running it again fixes that.
