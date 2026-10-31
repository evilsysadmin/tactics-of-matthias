import assert from 'node:assert/strict';
import test from 'node:test';
import {
  CHRONICLES_ISO_PARTY_FACING,
  CHRONICLES_ISO_PARTY_LAYOUT,
  CHRONICLES_ISO_EXPLORATION_PARTY_LAYOUT,
  CHRONICLES_ISO_MARKER_STYLE,
} from '../src/render/tacticsPartyPresentation.js';
import {
  chroniclesIsoTorchPlacements,
  chroniclesIsoUsesLegacyDressing,
} from '../src/render/tacticsLighting.js';

test('preserves all four independent tactical character slots and exploration formation', () => {
  const ids = ['matthias', 'rook', 'bishop', 'knight'];
  for (const layout of [CHRONICLES_ISO_PARTY_LAYOUT, CHRONICLES_ISO_EXPLORATION_PARTY_LAYOUT]) {
    assert.deepEqual(Object.keys(layout).sort(), ids.slice().sort());
    assert.equal(new Set(ids.map(id => `${layout[id].x}:${layout[id].z}`)).size, 4);
    for (const id of ids) {
      assert.ok(Number.isFinite(layout[id].x));
      assert.ok(Number.isFinite(layout[id].z));
      assert.ok(layout[id].scale > 0);
      assert.ok(Object.isFrozen(layout[id]));
    }
    assert.ok(Object.isFrozen(layout));
  }
  assert.equal(CHRONICLES_ISO_PARTY_FACING, Math.PI);
  assert.equal(CHRONICLES_ISO_MARKER_STYLE.shape, 'square');
});

test('legacy crypt keeps six authored torches, other empty maps invent none', () => {
  const crypt = { sceneStyle: { dressing: 'crypt-legacy' } };
  assert.equal(chroniclesIsoUsesLegacyDressing(crypt), true);
  assert.equal(chroniclesIsoTorchPlacements(crypt).length, 6);
  assert.deepEqual(chroniclesIsoTorchPlacements({ wallFaces: [] }), []);
  assert.deepEqual(chroniclesIsoTorchPlacements({}), []);
  assert.equal(chroniclesIsoUsesLegacyDressing({}), false);
});

test('wall-face torches deduplicate cells, orient onto wall and cap mobile density', () => {
  const wallFaces = Array.from({ length: 24 }, (_, i) => ({ x: i, y: 2, side: ['north', 'east', 'south', 'west'][i % 4] }));
  wallFaces.push({ x: 0, y: 2, side: 'south' });
  const plan = { wallFaces };
  const desktop = chroniclesIsoTorchPlacements(plan);
  const touch = chroniclesIsoTorchPlacements(plan, { coarsePointer: true });
  assert.equal(desktop.length, 8);
  assert.equal(touch.length, 5);
  assert.equal(desktop[0].oz, -0.94);
  assert.equal(desktop[1].ox, -0.94);
  assert.ok(desktop.every(t => Number.isFinite(t.ox) && Number.isFinite(t.oz)));
  assert.equal(new Set(desktop.map(t => `${t.x}:${t.y}`)).size, desktop.length);
});
