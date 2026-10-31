import assert from 'node:assert/strict';
import test from 'node:test';
import { tacticsAttackDistance, tacticsLineIsClear } from '../src/tacticsTargetGeometry.js';

const from = { x: 3, y: 3 };
const dist = (x, y, attackPattern) => tacticsAttackDistance(from, { x, y }, { attackPattern });

test('adjacent, orthogonal, diagonal and ranged tactical patterns remain distinct', () => {
  assert.equal(dist(4, 3, 'adjacent'), 1);
  assert.equal(dist(4, 4, 'adjacent'), null);
  assert.equal(dist(5, 3, 'adjacent'), null);
  assert.equal(dist(3, 5, 'orthogonal'), 2);
  assert.equal(dist(4, 4, 'orthogonal'), null);
  assert.equal(dist(3, 3, 'orthogonal'), 0);
  assert.equal(dist(5, 5, 'diagonal'), 2);
  assert.equal(dist(3, 3, 'diagonal'), null);
  assert.equal(dist(6, 5, 'diagonal'), null);
  assert.equal(dist(3, 6, 'line'), 3);
  assert.equal(dist(5, 5, 'line'), 2);
  assert.equal(dist(6, 5, 'line'), null);
  assert.equal(dist(4, 3, 'not-a-class'), null);
});

test('line of sight stops at intermediate walls or living blockers, not the target square', () => {
  const to = { x: 3, y: 6 };
  const clear = { tileAt: () => '.', positionedEnemies: [] };
  assert.equal(tacticsLineIsClear(from, to, clear), true);
  assert.equal(tacticsLineIsClear(from, to, {
    ...clear,
    tileAt: (x, y) => (x === 3 && y === 5 ? '#' : '.'),
  }), false);
  assert.equal(tacticsLineIsClear(from, to, {
    ...clear,
    positionedEnemies: [{ enemy: { id: 'blocker' }, position: { x: 3, y: 4 } }],
  }), false);
  assert.equal(tacticsLineIsClear(from, to, {
    ...clear,
    positionedEnemies: [{ enemy: { id: 'target' }, position: to }],
  }), true);
});

test('ignored enemy remains transparent for an intended attack; non-line targets are invalid', () => {
  const enemy = { enemy: { id: 'spectre' }, position: { x: 4, y: 4 } };
  const opts = { tileAt: () => '.', positionedEnemies: [enemy] };
  assert.equal(tacticsLineIsClear(from, { x: 5, y: 5 }, opts), false);
  assert.equal(tacticsLineIsClear(from, { x: 5, y: 5 }, { ...opts, ignoredEnemyId: 'spectre' }), true);
  assert.equal(tacticsLineIsClear(from, { x: 5, y: 4 }, opts), false);
});
