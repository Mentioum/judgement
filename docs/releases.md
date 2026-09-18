# Packaging and releases

Judgement ships one CLI and a Go module. GoReleaser OSS builds the binaries and
source archive. `packaging/prepare.mjs` prepares npm and Homebrew files from that
build; GoReleaser generates AUR metadata. No provider-specific packaging or
runtime dependencies are added to the Go binary.

## Channels

| Channel | Package | Installation behavior |
| --- | --- | --- |
| GitHub Releases | Six binary archives | Extract and add to PATH; no Go needed |
| npm | `@mentioum/judgement` | Node.js 22+ launcher chooses the bundled binary |
| Homebrew | `mentioum/tap/judgement` | Source formula builds with Homebrew's Go on macOS/Linux |
| AUR | `judgement-bin` | Installs the Linux release binary on x86_64/aarch64 |
| Go | `github.com/Mentioum/judgement/cmd/judgement` | `go install` with Go 1.23+ |

**npm, the Homebrew tap, and the AUR package have not been published yet.** Their
account setup must be completed before advertising working installation commands.

The npm tarball bundles all six binaries (about 16 MB compressed), with no
dependencies, lifecycle scripts, or runtime downloads. This trades a larger
download for one package to maintain and works with `--ignore-scripts`. The
launcher preserves stdin/stdout/stderr, environment, exit codes, and cancellation.
It is a CLI distribution, not a JavaScript SDK. Supported combinations are
macOS, Linux, and Windows on x64 and arm64.

The Homebrew formula follows the
[source-build approach for open-source CLI tools](https://docs.brew.sh/Adding-Software-to-Homebrew).
Homebrew installs the build dependency automatically. Unlike the earlier cask
proposal, this approach also supports Linux and does not need Apple signing
credentials. It does not disable quarantine or other operating-system checks.

## Local package checks

Use GoReleaser OSS 2.18.2 and Node.js 24/npm 11 (the CI versions):

```sh
goreleaser check
goreleaser release --snapshot --clean
node packaging/prepare.mjs
npm pack ./dist/packages/npm --pack-destination ./dist/packages --ignore-scripts
node packaging/test-npm.mjs dist/packages/*.tgz
```

Snapshot mode writes to `dist/` without publishing or registry credentials.
`--clean` replaces that directory. Snapshot URLs in the Homebrew/AUR definitions
are only previews, not downloadable releases. CI supplies the local archives to
test these definitions before a public release exists.

CI checks every archive checksum, installs the actual npm tarball on all three
operating systems, builds/tests the Homebrew formula on macOS, and runs `makepkg`
in an Arch Linux container. It also checks generated `.SRCINFO` against
`makepkg --printsrcinfo`. Native arm64 Windows/Linux installation and a Homebrew
Linux installation have not been exercised; their binaries are cross-compiled.

The archive names are `judgement_<version>_<os>_<arch>.<format>`, with the binary
at the root. Unix uses tar.gz; Windows uses zip. `checksums.txt` covers these and
the source archive. `package-checksums.txt` separately covers the npm tarball,
formula, `PKGBUILD`, and `.SRCINFO`. Checksums detect corruption, not publisher
identity. Source and packaged CLI versions come from the release tag; keep the
default `Version` in `internal/cli/cli.go` current for `go install` too.

## Release and publication

1. Verify live API behavior with a TypeSafe key and synthetic input before
   claiming live-service verification. This remains outstanding.
2. Update the source version and changelog, merge reviewed changes, and wait
   for CI to pass. Run the snapshot checks above.
3. Push the reviewed `vX.Y.Z` tag. This makes the Go module version public
   immediately. The release workflow creates a **draft** with binary/source
   archives, the npm tarball, formula, AUR files, and both checksum files.
4. Check that the entire workflow succeeded, inspect the draft assets, and
   smoke-test downloaded packages. Then publish the draft.
5. Run **Publish packages** (`publish-packages.yml`) from `main`, supplying the
   published tag and one channel: `npm`, `homebrew`, or `aur`. Repeat per channel.

The publication workflow rejects drafts, prereleases, and non-stable tag names,
verifies package checksums and the npm package version, and publishes the exact
reviewed files. It does not rebuild them. No registry is updated by a PR,
snapshot, or draft release. A failed channel can be retried independently;
npm versions are immutable, so an already-published version must not be republished.

## One-time publisher setup

### npm

Confirm ownership of the `@mentioum` scope. For the first stable release, log in
with `npm login`, download the reviewed tarball and `package-checksums.txt`,
verify the tarball against its entry, and publish it with:

```sh
npm publish ./mentioum-judgement-X.Y.Z.tgz --access public
```

In the package's npm settings, add a
[trusted publisher](https://docs.npmjs.com/trusted-publishers/) for GitHub owner
`Mentioum`, repository `judgement`, workflow `publish-packages.yml`, allowing
direct publishing. Subsequent releases use OIDC; no npm token is stored in GitHub.
The first publish may require interactive 2FA. Do not paste tokens into issues,
chat, or configuration files.

### Homebrew

Create the public `Mentioum/homebrew-tap` repository. Configure
`HOMEBREW_TAP_TOKEN` in judgement's Actions secrets with contents-write permission
for that tap only. The workflow commits `Formula/judgement.rb` after the release
is public. Users then run `brew install mentioum/tap/judgement` and receive updates
through `brew upgrade`. A normal `GITHUB_TOKEN` cannot push to another repository.

### AUR

Confirm the `judgement-bin` package is available, or that the intended AUR
account maintains it. Register a dedicated SSH public key with that account and
put its private key in the `AUR_SSH_PRIVATE_KEY` Actions secret. Set the
`AUR_KNOWN_HOSTS` repository variable to the host-key entry verified against
Arch's published SSH fingerprints. Host-key checking stays enabled.

The workflow pushes only `PKGBUILD` and `.SRCINFO` to
`ssh://aur@aur.archlinux.org/judgement-bin.git`, using AUR's `master` branch.
The definition includes architecture-specific SHA-256 checksums, certificate
dependencies, and the MIT license file. Users may use `yay -S judgement-bin`,
another AUR helper, or clone the AUR repository and run `makepkg -si`.
