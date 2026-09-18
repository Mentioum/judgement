# Packaging and releases

Judgement ships one standalone CLI and a Go module. Packaging does not depend on
the provider: additional provider adapters will ship inside the same binary.
The release setup uses GoReleaser OSS; the CLI gains no runtime dependencies.

## Installation paths

| Audience | Initial path | Follow-up when needed |
| --- | --- | --- |
| macOS | GitHub Release `.tar.gz`, arm64 or amd64 | Homebrew cask after signing and tap setup |
| Linux | GitHub Release `.tar.gz`, arm64 or amd64 | Distribution packages if users need them |
| Windows | GitHub Release `.zip`, arm64 or amd64 | Scoop if users need automatic updates |
| Go developers | `go install github.com/Mentioum/judgement/cmd/judgement@latest` | Pin a tag for reproducible installs |

Homebrew and Scoop installation are not available yet. Avoid advertising those
commands until they work. Agents can use the same binaries as people; a separate
installer, daemon, package registry, or plugin system is unnecessary.

## Local packaging checks

Install [GoReleaser OSS](https://goreleaser.com/getting-started/install/oss/).
CI pins v2.18.2. From a checkout with Git tags:

```sh
goreleaser check
goreleaser release --snapshot --clean
```

Snapshot mode writes to `dist/` without publishing or requiring GitHub
credentials. `--clean` replaces previous contents of that directory. The config
builds six targets with CGO disabled and includes the license, README, changelog,
and CLI/agent guides. Unix archives are tar.gz; Windows archives are zip.

New archives use `judgement_<version>_<os>_<arch>.<format>` and place the binary
at the archive root. Older releases have different archive names/layouts. Each
archive is listed in the SHA-256 `checksums.txt`. On macOS, for example:

```sh
cd dist
shasum -a 256 -c checksums.txt
```

This checks every archive; when downloading just one, verify its matching entry.
Checksums detect corruption but do not replace publisher signing. CI verifies
all checksums and extracts/runs the Linux amd64 archive. Local smoke testing
should also run the extracted binary's `version`, `describe`, and `validate`
commands without API credentials.

## Publishing a version

1. Check the API behavior with a real TypeSafe key and synthetic input before
   claiming live-service verification. This has not yet been done.
2. Update the source `Version` and changelog, and ensure CI passes. Preview the
   release with the snapshot commands above. Do not commit `dist/`.
3. Tag the reviewed commit, for example `v0.2.0`. Pushing a `v*` tag runs tests
   and GoReleaser, which creates a **draft** GitHub Release.
4. Inspect the draft assets, checksum file, version output, and release notes.
   Smoke-test downloaded archives on the platforms being advertised.
5. Publish the reviewed draft. Drafting does not gate the Go module: pushing a
   public tag makes that version available to `go install` already, so review
   the code before tagging.

The workflow uses the repository's `GITHUB_TOKEN`. It neither publishes a
package-manager definition nor creates a new repository. No release is made by
PR or snapshot builds.

## Homebrew next

For convenient macOS installation, use GoReleaser's current
[`homebrew_casks` support](https://goreleaser.com/customization/publish/homebrew_casks/).
Its older `brews` configuration is deprecated. Add the cask only after:

* A macOS signing/notarization approach has been verified on a clean machine.
  Do not build the install flow around removing quarantine checks.
* A public Homebrew tap exists, with narrowly scoped publishing credentials.
  The current repository's workflow token cannot write to a separate tap.
* The cask's URLs and checksums point to a published release. Update the tap
  after publication so users cannot install references to draft assets.

Keep package-manager work separate from the draft-building workflow. The
[official action documentation](https://goreleaser.com/customization/ci/actions/)
describes the token requirements. Add Linux packages or Scoop when there is a
concrete need; do not maintain multiple distribution systems preemptively.
