// Exercise an actual npm-packed install with lifecycle scripts disabled.
import assert from 'node:assert/strict';
import { mkdtempSync, readFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { spawn, spawnSync } from 'node:child_process';
import { once } from 'node:events';

const archive = resolve(process.argv[2]);
const prefix = mkdtempSync(join(tmpdir(), 'judgement-npm-'));
try {
  const npm = process.platform === 'win32' ? 'npm.cmd' : 'npm';
  const install = spawnSync(npm, ['install', '--global', '--prefix', prefix,
    '--cache', join(prefix, 'cache'), '--ignore-scripts', '--no-audit', '--no-fund', '--offline', archive],
  { encoding: 'utf8', shell: process.platform === 'win32' });
  assert.equal(install.status, 0, install.stderr);
  const root = process.platform === 'win32' ? prefix : join(prefix, 'lib');
  const pkg = join(root, 'node_modules', '@mentioum', 'judgement');
  const version = JSON.parse(readFileSync(join(pkg, 'package.json'))).version;
  const cli = process.platform === 'win32' ? join(prefix, 'judgement.cmd') : join(prefix, 'bin/judgement');
  const run = (args, input) => spawnSync(cli, args, {
    encoding: 'utf8', input, shell: process.platform === 'win32',
  });
  let result = run(['version']);
  assert.equal(result.status, 0, result.stderr);
  assert.equal(JSON.parse(result.stdout).version, version);
  assert.equal(result.stderr, '');
  result = run(['validate'], readFileSync('examples/request.json'));
  assert.equal(result.status, 0, result.stderr);
  assert.equal(JSON.parse(result.stdout).valid, true);
  result = run(['validate'], '{broken');
  assert.notEqual(result.status, 0);
  assert.equal(result.stdout, '');
  assert.ok(JSON.parse(result.stderr).error);
  if (process.platform !== 'win32') {
    const child = spawn(process.execPath, [join(pkg, 'cli.cjs'), 'validate'], { stdio: 'pipe' });
    const closed = once(child, 'close');
    await new Promise(resolve => setTimeout(resolve, 500));
    child.kill('SIGINT');
    const timeout = setTimeout(() => child.kill('SIGKILL'), 5000);
    const [code, signal] = await closed;
    clearTimeout(timeout);
    assert.ok(code === 130 || signal === 'SIGINT', `Unexpected interrupt result: ${code}/${signal}`);
  }
  console.log(`npm install and CLI checks passed on ${process.platform}/${process.arch}`);
} finally {
  rmSync(prefix, { recursive: true, force: true });
}
