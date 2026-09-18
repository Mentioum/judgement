#!/usr/bin/env bash
# Run inside the official Arch base-devel container with the checkout at /workspace.
set -euo pipefail
work=$(mktemp -d)
cp /workspace/dist/packages/PKGBUILD "$work/PKGBUILD"
cd "$work"
source ./PKGBUILD
# Seed makepkg's download cache with the exact local archive. It still verifies
# the generated checksum; snapshot URLs intentionally do not exist publicly.
cp /workspace/dist/judgement_*_linux_amd64.tar.gz "${source_x86_64[0]%%::*}"
useradd --create-home builder
chown -R builder:builder "$work"
runuser -u builder -- makepkg --nodeps --noconfirm
runuser -u builder -- makepkg --printsrcinfo > actual.srcinfo
diff <(sed '/^[[:space:]]*$/d; s/^[[:space:]]*//' actual.srcinfo | sort) \
     <(sed '/^[[:space:]]*$/d; s/^[[:space:]]*//' /workspace/dist/packages/.SRCINFO | sort)
mkdir extracted
bsdtar -xf "judgement-bin-${pkgver}-${pkgrel}-x86_64.pkg.tar.zst" -C extracted
test -f extracted/usr/share/licenses/judgement-bin/LICENSE
./extracted/usr/bin/judgement version
./extracted/usr/bin/judgement validate --input /workspace/examples/request.json
