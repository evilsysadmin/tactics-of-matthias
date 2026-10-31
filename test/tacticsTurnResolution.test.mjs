import assert from 'node:assert/strict';
import test from 'node:test';
import { tacticsResolvePlayerAction } from '../src/tacticsTurnResolution.js';

function harness(override = {}) {
  const calls = { advance: 0, start: [] };
  const ports = {
    prepareExplorationSpawn: state => state,
    advanceCombatInitiative: state => { calls.advance += 1; return { ...state, advanced: true }; },
    activeEnemies: state => state.enemies || [],
    combatActive: state => Boolean(state.engaged),
    advanceExplorationEnemies: (_previous, next) => ({ ...next, enemiesStepped: true }),
    startInitiativeCombat: (state, enemies, options) => {
      calls.start.push({ state, enemies, options });
      return { ...state, phase: 'combat', message: 'Iniciativa.', forcedIds: options.forceEnemyIds };
    },
    ...override,
  };
  return { ports, calls };
}

test('terminal campaigns and unchanged actions cannot consume an initiative slot', () => {
  const { ports, calls } = harness();
  const defeated = { phase: 'defeated' };
  assert.equal(tacticsResolvePlayerAction(defeated, { phase: 'explore' }, { ports }), defeated);
  assert.equal(tacticsResolvePlayerAction(defeated, null, { ports }), defeated);
  const current = { phase: 'combat', initiative: { order: [{ kind: 'party', id: 'matthias' }] } };
  assert.equal(tacticsResolvePlayerAction(current, current, { ports }), current);
  assert.equal(calls.advance, 0);
  assert.equal(tacticsResolvePlayerAction(current, { ...current, turns: 1 }, { ports }).advanced, true);
  assert.equal(calls.advance, 1);
});

test('exploration enemy approach starts the genuine initiative boundary', () => {
  const { ports, calls } = harness({
    advanceExplorationEnemies: (_previous, next) => ({ ...next, engaged: true, message: 'Contacto.' }),
  });
  const before = { phase: 'explore', engaged: false, enemies: [{ id: 'pawn' }], turns: 0 };
  const result = tacticsResolvePlayerAction(before, { ...before, turns: 1 }, {
    ports, random: () => 0,
  });
  assert.equal(result.phase, 'combat');
  assert.equal(result.message, 'Contacto. Iniciativa.');
  assert.equal(calls.start.length, 1);
  assert.deepEqual(calls.start[0].options.forceEnemyIds, ['pawn']);
});

test('forced ranged attacks initiate combat before committing damage', () => {
  const { ports } = harness();
  const before = { phase: 'explore', engaged: false, enemies: [{ id: 'pawn' }], hp: 9, turns: 0 };
  const result = tacticsResolvePlayerAction(before, { ...before, hp: 2, turns: 1 }, {
    forceCombat: true, forceEnemyIds: ['pawn', 'pawn', 'bishop'], ports,
  });
  assert.equal(result.hp, 9);
  assert.equal(result.turns, 0);
  assert.deepEqual(result.forcedIds, ['pawn', 'bishop']);
});

test('map transition resets exploration cadence and asks real world port to prepare spawn', () => {
  let relocated = 0;
  const { ports, calls } = harness({
    prepareExplorationSpawn: state => { relocated += 1; return { ...state, relocated: true }; },
  });
  const before = { mapId: 'crypt', explorationEnemySteps: 7, phase: 'explore' };
  const result = tacticsResolvePlayerAction(before, { ...before, mapId: 'town' }, { ports });
  assert.equal(result.mapId, 'town');
  assert.equal(result.explorationEnemySteps, 0);
  assert.equal(result.relocated, true);
  assert.equal(relocated, 1);
  assert.equal(calls.start.length, 0);
});

test('room-scale engagement schedules all living enemies even when only one caused contact', () => {
  const { ports, calls } = harness();
  const before = { phase: 'explore', engaged: true,
    enemies: [{ id: 'pawn' }, { id: 'spectre' }], turns: 0 };
  const result = tacticsResolvePlayerAction(before, { ...before, turns: 1 }, {
    forceEnemyIds: ['pawn'], ports,
  });
  assert.deepEqual(result.forcedIds, ['pawn', 'spectre']);
  assert.equal(calls.start[0].state, before);
});
