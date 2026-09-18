// Run after GoReleaser. Keep packaging separate from publishing draft assets.
import { copyFileSync, cpSync, mkdirSync, readFileSync, writeFileSync, chmodSync } from 'node:fs';
import { createHash } from 'node:crypto';
import { join } from 'node:path';

const dist = process.argv[2] || 'dist';
const readJSON = path => JSON.parse(readFileSync(path, 'utf8'));
const metadata = readJSON(join(dist, 'metadata.json'));
const artifacts = readJSON(join(dist, 'artifacts.json'));
const version = metadata.version;
if (!/^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$/.test(version)) {
  throw new Error(`Unsupported package version: ${version}`);
}
const output = join(dist, 'packages');
const npm = join(output, 'npm');
const platforms = { darwin: 'darwin', linux: 'linux', windows: 'win32' };
const arches = { amd64: 'x64', arm64: 'arm64' };
for (const [goos, platform] of Object.entries(platforms)) {
  for (const [goarch, arch] of Object.entries(arches)) {
    const matches = artifacts.filter(a => a.type === 'Binary' && a.extra?.ID === 'judgement' &&
      a.goos === goos && a.goarch === goarch);
    if (matches.length !== 1) throw new Error(`Expected one binary for ${goos}/${goarch}`);
    const directory = join(npm, 'native', `${platform}-${arch}`);
    mkdirSync(directory, { recursive: true });
    const target = join(directory, goos === 'windows' ? 'judgement.exe' : 'judgement');
    copyFileSync(matches[0].path, target);
    chmodSync(target, 0o755);
  }
}
const manifest = readJSON('packaging/npm/package.json');
manifest.version = version;
writeFileSync(join(npm, 'package.json'), JSON.stringify(manifest, null, 2) + '\n');
copyFileSync('packaging/npm/cli.cjs', join(npm, 'cli.cjs'));
chmodSync(join(npm, 'cli.cjs'), 0o755);
for (const file of ['LICENSE', 'README.md']) copyFileSync(file, join(npm, file));
cpSync('docs', join(npm, 'docs'), { recursive: true });

const source = artifacts.find(a => a.type === 'Source');
if (!source) throw new Error('GoReleaser source archive is missing');
const checksum = createHash('sha256').update(readFileSync(source.path)).digest('hex');
const url = `https://github.com/Mentioum/judgement/releases/download/${metadata.tag}/${source.name}`;
let formula = readFileSync('packaging/homebrew.rb.tmpl', 'utf8');
for (const [key, value] of Object.entries({ URL: url, VERSION: version, SHA256: checksum })) {
  formula = formula.replaceAll(`@@${key}@@`, value);
}
writeFileSync(join(output, 'judgement.rb'), formula);
copyFileSync(join(dist, 'aur/judgement-bin.pkgbuild'), join(output, 'PKGBUILD'));
copyFileSync(join(dist, 'aur/judgement-bin.srcinfo'), join(output, '.SRCINFO'));
console.log(`Prepared npm, Homebrew, and AUR packages for ${version} in ${output}`);
