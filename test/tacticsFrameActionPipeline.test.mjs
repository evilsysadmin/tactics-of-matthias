import assert from 'node:assert/strict';
import test from 'node:test';
import { tacticsSceneFrame } from '../src/adapters/tacticsSceneFrame.js';
import { tacticsDispatchFrameAction } from '../src/adapters/tacticsFrameActionPipeline.js';
import { tacticsResolvePlayerAction } from '../src/tacticsTurnResolution.js';

// Real scene *schema*, not a fallback game world. Engine rules come from ports.
function frame() {
  return tacticsSceneFrame({
    version: 1, mapId: 'crypt-eight-squares',
    scenePlan: {
      mapId: 'crypt-eight-squares', title: 'Cripta',
      width: 5, height: 5, center: { x: 2, y: 2 },
      floors: [
        { x: 1, y: 1, tile: '.' },
        { x: 2, y: 1, tile: '.' },
        { x: 3, y: 1, tile: '.' },
      ],
      walls: [], wallFaces: [],
    },
    focusCell: { x: 1, y: 1 }, selectedMemberId: 'rook',
    partyFormation: 'explore-compact',
    party: [{ id: 'rook', cell: { x: 1, y: 1 }, hpRatio: 1, visible: true }],
    enemies: [{ id: 'pawn', cell: { x: 3, y: 1 }, visible: true, visualType: 'pawn' }],
    interaction: {
      mode: 'hybrid',
      legalMoves: [{ x: 2, y: 1 }],
      legalTargets: [{ enemyId: 'pawn', x: 3, y: 1 }],
    },
  });
}

function world() {
  return {
    mapId: 'crypt-eight-squares', phase: 'explore', turnPhase: 'party',
    x: 1, y: 1, initiative: null, turns: 0, hp: 5,
  };
}

function harness({ legalMove = true, legalAttack = true, actor = null } = {}) {
  const events = [];
  const runtime = {
    prepareExplorationSpawn: state => state,
    advanceCombatInitiative: state => ({ ...state, initiativeAdvanced: true }),
    activeEnemies: () => [{ id: 'pawn' }],
    combatActive: () => false,
    advanceExplorationEnemies: (_previous, next) => next,
    startInitiativeCombat: (state, enemies) =>
      ({ ...state, phase: 'combat', initiative: { order: enemies }, forced: true }),
  };
  const ports = {
    partyCanAct: state => state.phase !== 'defeated' && state.turnPhase !== 'enemy',
    currentActor: () => actor,
    memberPosition: () => ({ x: 1, y: 1 }),
    legalMoves: () => legalMove ? [{ x: 2, y: 1 }] : [],
    move: (state, destination) => {
      events.push('move');
      return { ...state, x: destination.x, y: destination.y };
    },
    targets: () => legalAttack ? [{ enemyId: 'pawn' }] : [],
    attack: (state, _memberId, enemyId) => {
      events.push('attack:' + enemyId);
      return { ...state, hp: 3 };
    },
    resolveAction: (previous, next, options) => tacticsResolvePlayerAction(
      previous, next, { ...options, ports: runtime },
    ),
  };
  return { ports, events };
}

test('a legal map click reaches the current move reducer and resolves a turn', () => {
  const { ports, events } = harness();
  const result = tacticsDispatchFrameAction(world(), frame(), { kind: 'move', x: 2, y: 1 }, ports);
  assert.deepEqual(events, ['move']);
  assert.deepEqual(result, {
    next: { ...world(), x: 2, y: 1 },
    actorMemberId: null, actionKind: 'move',
  });
});

test('an attack uses the real forced-combat boundary before attempting damage', () => {
  const { ports, events } = harness();
  const result = tacticsDispatchFrameAction(world(), frame(), { kind: 'attack', enemyId: 'pawn' }, ports);
  assert.deepEqual(events, ['attack:pawn']);
  assert.equal(result.actorMemberId, 'rook');
  assert.equal(result.actionKind, 'attack');
  assert.equal(result.next.phase, 'combat');
  assert.equal(result.next.hp, 5);
  assert.equal(result.next.forced, true);
});

test('stale frames, new initiative actors and changed engine legality fail closed', () => {
  const noMove = harness({ legalMove: false });
  assert.equal(tacticsDispatchFrameAction(world(), frame(),
    { kind: 'move', x: 2, y: 1 }, noMove.ports), null);
  assert.deepEqual(noMove.events, []);
  const noAttack = harness({ legalAttack: false });
  assert.equal(tacticsDispatchFrameAction(world(), frame(),
    { kind: 'attack', enemyId: 'pawn' }, noAttack.ports), null);
  assert.deepEqual(noAttack.events, []);
  const changed = harness({ actor: { kind: 'party', id: 'matthias' } });
  assert.equal(tacticsDispatchFrameAction(world(), frame(),
    { kind: 'attack', enemyId: 'pawn' }, changed.ports), null);
  assert.deepEqual(changed.events, []);
  const enemyTurn = harness({ actor: { kind: 'enemy', id: 'pawn' } });
  assert.equal(tacticsDispatchFrameAction(world(), frame(),
    { kind: 'move', x: 2, y: 1 }, enemyTurn.ports), null);
  assert.equal(tacticsDispatchFrameAction({ ...world(), mapId: 'town' }, frame(),
    { kind: 'move', x: 2, y: 1 }, harness().ports), null);
  assert.equal(tacticsDispatchFrameAction(world(), frame(),
    { kind: 'move', x: 4, y: 4 }, harness().ports), null);
});

test('an active party member uses their initiative position, not the old party anchor', () => {
  const { ports } = harness({ actor: { kind: 'party', id: 'rook' } });
  const state = { ...world(), phase: 'combat',
    initiative: { order: [{ kind: 'party', id: 'rook' }], cursor: 0 } };
  const result = tacticsDispatchFrameAction(state, frame(),
    { kind: 'move', x: 2, y: 1 }, ports);
  assert.equal(result.actorMemberId, 'rook');
  assert.equal(result.next.initiativeAdvanced, true);
});
