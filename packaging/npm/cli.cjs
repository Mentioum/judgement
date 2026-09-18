#!/usr/bin/env node
'use strict';

const { spawn } = require('node:child_process');
const { join } = require('node:path');

function fail(message) {
  process.stderr.write(JSON.stringify({ error: { code: 'launcher_error', message } }) + '\n');
  process.exitCode = 1;
}

if (!['darwin', 'linux', 'win32'].includes(process.platform) ||
    !['x64', 'arm64'].includes(process.arch)) {
  fail(`Unsupported platform: ${process.platform}/${process.arch}`);
} else {
  const binary = join(__dirname, 'native', `${process.platform}-${process.arch}`,
    process.platform === 'win32' ? 'judgement.exe' : 'judgement');
  const child = spawn(binary, process.argv.slice(2), { stdio: 'inherit' });
  const signals = ['SIGINT', 'SIGTERM'];
  const handlers = signals.map(signal => () => child.kill(signal));
  signals.forEach((signal, i) => process.on(signal, handlers[i]));
  child.on('error', error => fail(`Could not start judgement: ${error.message}`));
  child.on('close', (code, signal) => {
    signals.forEach((name, i) => process.removeListener(name, handlers[i]));
    if (signal) process.kill(process.pid, signal);
    else if (code !== null) process.exitCode = code;
  });
}
