#!/usr/bin/env bash
# CI-only: use an isolated tap and local source archive to test an unpublished build.
set -euo pipefail
export HOMEBREW_NO_AUTO_UPDATE=1 HOMEBREW_NO_INSTALL_CLEANUP=1
brew tap-new judgement-test/local
tap=$(brew --repository judgement-test/local)
cp dist/packages/judgement.rb "$tap/Formula/judgement.rb"
source_archive=$(find "$PWD/dist" -maxdepth 1 -name 'judgement_*_source.tar.gz' -print)
ruby -e 'p = ARGV[0]; s = File.read(p); s.sub!(/url "[^"]+"/, "url \"file://#{ARGV[1]}\""); File.write(p, s)' \
  "$tap/Formula/judgement.rb" "$source_archive"
brew install --build-from-source judgement-test/local/judgement
brew test judgement-test/local/judgement
