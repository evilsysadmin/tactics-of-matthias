import assert from 'node:assert/strict';
import test from 'node:test';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

const file = relative => readFileSync(fileURLToPath(new URL('../' + relative, import.meta.url)), 'utf8');

test('standalone page mounts Tactics shell, no local game or fake identity', () => {
  const html = file('index.html');
  const main = file('src/browser/main.js');
  assert.match(html, /<html lang="es">/);
  assert.match(html, /id="app"/);
  assert.match(html, /src="\/src\/browser\/main.js"/);
  assert.match(main, /mountTacticsAppShell\(\{ root \}\)/);
  assert.doesNotMatch(main, /loadAuthorizedWorld|fakeWorld|localStorage|chroniclesCreateRun|demoMap/);
  assert.match(main, /pagehide/);
});

test('build scripts use pinned Vite and three without hosted/CDN fallbacks', () => {
  const pkg = JSON.parse(file('package.json'));
  const lock = JSON.parse(file('package-lock.json'));
  assert.equal(pkg.dependencies.three, '0.186.1');
  assert.equal(pkg.devDependencies.vite, '6.4.3');
  assert.equal(lock.packages['node_modules/vite'].version, pkg.devDependencies.vite);
  assert.equal(lock.packages['node_modules/three'].version, pkg.dependencies.three);
  assert.match(pkg.scripts.build, /vite build/);
  assert.match(file('.github/workflows/ci.yml'), /npm run build/);
});
