import assert from 'node:assert/strict';
import test from 'node:test';
import { tacticsSceneFrame } from '../src/adapters/tacticsSceneFrame.js';

// Shape of the real v1 Chronicles scene projection. Contract data, not a
// fallback playable map or invented server response.
function projectedScene() {
  return {
    version: 1, mapId: 'crypt-eight-squares',
    scenePlan: {
      mapId: 'crypt-eight-squares', title: 'Cripta de las Ocho Casillas',
      width: 5, height: 5, center: { x: 2, y: 2 },
      floors: [
        { x: 1, y: 1, tile: '.' }, { x: 2, y: 1, tile: '.' },
        { x: 3, y: 1, tile: '.' }, { x: 2, y: 2, tile: 'P' },
      ],
      walls: [{ x: 0, y: 1 }],
      wallFaces: [{ x: 0, y: 1, side: 'east' }],
    },
    focusCell: { x: 1, y: 1 }, partyFormation: 'explore-compact',
    selectedMemberId: 'matthias',
    party: ['matthias', 'rook', 'bishop', 'knight'].map(id => ({
      id, visible: true, hpRatio: 1, cell: { x: 1, y: 1 },
    })),
    enemies: [{ id: 'corrupted-pawn', visible: true,
      cell: { x: 3, y: 1 }, visualType: 'corrupted-pawn' }],
    interaction: { mode: 'hybrid', legalMoves: [{ x: 2, y: 1 }],
      legalTargets: [{ x: 3, y: 1, enemyId: 'corrupted-pawn', damage: 999 }] },
    content: [{ id: 'secret-story', body: 'do not leak' }],
    worldFlags: { completed: true },
    rpgModifiers: { matthias: { attackDamageBonus: 999 } },
  };
}

test('translates the real v1 projected scene shape to a detached renderer frame', () => {
  const source = projectedScene();
  const frame = tacticsSceneFrame(source);
  assert.equal(frame.version, 1);
  assert.equal(frame.map.id, source.mapId);
  assert.deepEqual(frame.map.floors[0], { x: 1, y: 1, tile: '.' });
  assert.equal(frame.party.length, 4);
  assert.equal(frame.partyFormation, 'explore-compact');
  assert.deepEqual(frame.interaction.legalTargets, [{ x: 3, y: 1, enemyId: 'corrupted-pawn' }]);
  assert.deepEqual(frame.torches, [{ x: 0, y: 1, ox: 0.94, oz: 0 }]);
  assert.ok(Object.isFrozen(frame) && Object.isFrozen(frame.map.floors[0]));
  assert.ok(Object.isFrozen(frame.party[0]) && Object.isFrozen(frame.interaction.legalTargets[0]));
  source.party[0].cell.x = 4;
  source.scenePlan.floors[0].tile = 'X';
  source.interaction.legalTargets[0].enemyId = 'changed';
  assert.deepEqual(frame.party[0].cell, { x: 1, y: 1 });
  assert.equal(frame.map.floors[0].tile, '.');
  assert.equal(frame.interaction.legalTargets[0].enemyId, 'corrupted-pawn');
});

test('exposes no raw narrative, RPG modifiers or authoritative save fields', () => {
  const json = JSON.stringify(tacticsSceneFrame(projectedScene()));
  for (const secret of ['secret-story', 'attackDamageBonus', 'worldFlags', '999']) {
    assert.equal(json.includes(secret), false, secret);
  }
});

test('fails closed for unsupported versions, mismatched maps and bad geometry', () => {
  const valid = projectedScene();
  const cases = [
    { ...valid, version: 2 },
    { ...valid, scenePlan: { ...valid.scenePlan, mapId: 'different' } },
    { ...valid, scenePlan: { ...valid.scenePlan, center: { x: NaN, y: 1 } } },
    { ...valid, scenePlan: { ...valid.scenePlan, floors: [valid.scenePlan.floors[0], valid.scenePlan.floors[0]] } },
    { ...valid, scenePlan: { ...valid.scenePlan, walls: [{ x: 1, y: 1 }] } },
    { ...valid, scenePlan: { ...valid.scenePlan, floors: [{ x: -1, y: 1, tile: '.' }] } },
    { ...valid, party: [...valid.party, valid.party[0]] },
    { ...valid, focusCell: { x: NaN, y: 1 } },
    { ...valid, selectedMemberId: 'stranger' },
    { ...valid, partyFormation: 'free-camera' },
    { ...valid, interaction: { ...valid.interaction,
      legalTargets: [{ x: 3, y: 1, enemyId: 'ghost' }] } },
  ];
  for (const invalid of cases) assert.throws(() => tacticsSceneFrame(invalid), /Tactics/i);
});

test('allows a combat-grid with fallen actors, no active actions and no new world', () => {
  const scene = projectedScene();
  scene.partyFormation = 'combat-grid';
  scene.party[1].visible = false;
  scene.party[1].cell = null;
  scene.enemies[0].visible = false;
  scene.enemies[0].cell = null;
  scene.interaction = null;
  const frame = tacticsSceneFrame(scene, { coarsePointer: true });
  assert.equal(frame.partyFormation, 'combat-grid');
  assert.equal(frame.party[1].cell, null);
  assert.equal(frame.enemies[0].cell, null);
  assert.equal(frame.interaction, null);
});
