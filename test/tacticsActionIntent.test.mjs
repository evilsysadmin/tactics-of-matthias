import assert from 'node:assert/strict';
import test from 'node:test';
import { tacticsSceneFrame } from '../src/adapters/tacticsSceneFrame.js';
import { tacticsIntentFromFrame, TACTICS_ACTION_INTENT_VERSION } from '../src/adapters/tacticsActionIntent.js';

function frame(mode = 'hybrid') {
  return tacticsSceneFrame({
    version: 1, mapId: 'crypt-eight-squares',
    scenePlan: {
      mapId: 'crypt-eight-squares', title: 'Cripta',
      width: 4, height: 4, center: { x: 1.5, y: 1.5 },
      floors: [
        { x: 1, y: 1, tile: '.' }, { x: 2, y: 1, tile: '.' },
        { x: 3, y: 1, tile: '.' },
      ],
      walls: [], wallFaces: [],
    },
    focusCell: { x: 1, y: 1 }, selectedMemberId: 'matthias',
    partyFormation: 'combat-grid',
    party: [{ id: 'matthias', visible: true, hpRatio: 1, cell: { x: 1, y: 1 } }],
    enemies: [{ id: 'pawn', visible: true, cell: { x: 3, y: 1 }, visualType: 'pawn' }],
    interaction: mode ? {
      mode, legalMoves: [{ x: 2, y: 1 }],
      legalTargets: [{ x: 3, y: 1, enemyId: 'pawn' }],
    } : null,
  });
}

test('renderer clicks only map to real legal moves and targets from scene contract', () => {
  const source = frame();
  const move = tacticsIntentFromFrame(source, { kind: 'move', x: 2, y: 1 });
  assert.deepEqual(move, {
    version: TACTICS_ACTION_INTENT_VERSION, mapId: 'crypt-eight-squares',
    memberId: 'matthias', kind: 'move', destination: { x: 2, y: 1 },
  });
  assert.ok(Object.isFrozen(move) && Object.isFrozen(move.destination));
  assert.deepEqual(tacticsIntentFromFrame(source, { kind: 'attack', enemyId: 'pawn' }), {
    version: 1, mapId: 'crypt-eight-squares',
    memberId: 'matthias', kind: 'attack', enemyId: 'pawn',
  });
});

test('rejects invalid clicks, fake enemy ids, unsupported actions and malformed frames', () => {
  const source = frame();
  for (const request of [
    { kind: 'move', x: 1, y: 2 }, { kind: 'move', x: 2.5, y: 1 },
    { kind: 'attack', enemyId: 'ghost' }, { kind: 'attack', enemyId: '' },
    { kind: 'wait' }, { kind: 'ability' }, { kind: 'use' }, null,
  ]) {
    assert.equal(tacticsIntentFromFrame(source, request), null);
  }
  assert.throws(() => tacticsIntentFromFrame({ version: 2 }, { kind: 'move', x: 2, y: 1 }), /frame/);
});

test('respects attack-only, move-only and no-active-action frames', () => {
  assert.equal(tacticsIntentFromFrame(frame('move'), { kind: 'attack', enemyId: 'pawn' }), null);
  assert.equal(tacticsIntentFromFrame(frame('attack'), { kind: 'move', x: 2, y: 1 }), null);
  assert.equal(tacticsIntentFromFrame(frame(null), { kind: 'attack', enemyId: 'pawn' }), null);
  const down = { ...frame(), party: [{ id: 'matthias', visible: false, cell: null, hpRatio: 0 }] };
  assert.equal(tacticsIntentFromFrame(down, { kind: 'attack', enemyId: 'pawn' }), null);
});
