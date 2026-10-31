import assert from 'node:assert/strict';
import test from 'node:test';
import { tacticsSceneFrame } from '../src/adapters/tacticsSceneFrame.js';
import { tacticsRemoteMapTopology, tacticsSceneModelFromRemoteWorld } from '../src/adapters/tacticsWorldScene.js';
import { tacticsCellToWorld, tacticsSpatialRenderPlan, TACTICS_ISOMETRIC_CELL_SIZE } from '../src/render/tacticsSpatialRenderPlan.js';

const cryptGrid = [
  '#######', '#..X..#', '#.###.#', '#...#.#',
  '#.#S#.#', '#P.E..#', '#######',
];
function frame() {
  const world = {
    source: 'remote', runStatus: 'active', runId: 'real-server-run',
    currentMapId: 'crypt-eight-squares',
    map: { id: 'crypt-eight-squares', title: 'Cripta', grid: cryptGrid },
  };
  const state = { mapId: world.currentMapId, phase: 'combat',
    initiative: { order: [{ kind: 'party', id: 'matthias' }] } };
  const model = tacticsSceneModelFromRemoteWorld(world, state, {
    selectedMemberId: 'matthias',
    focusCell: { x: 3, y: 3 },
    party: [
      { id: 'matthias', visible: true, hpRatio: 1, cell: { x: 1, y: 5 } },
      { id: 'rook', visible: false, hpRatio: 0, cell: null },
    ],
    enemies: [{
      id: 'corrupted-pawn', visible: true,
      visualType: 'corrupted-pawn', cell: { x: 3, y: 5 },
    }],
    interaction: {
      mode: 'hybrid', legalMoves: [{ x: 1, y: 4 }],
      legalTargets: [{ enemyId: 'corrupted-pawn', x: 3, y: 5 }],
    },
  });
  return tacticsSceneFrame(model);
}

test('projects canonical isometric scale and the real crypt footprint', () => {
  const input = frame();
  const plan = tacticsSpatialRenderPlan(input);
  assert.equal(TACTICS_ISOMETRIC_CELL_SIZE, 2.45);
  assert.equal(plan.cellSize, 2.45);
  assert.equal(plan.mapId, 'crypt-eight-squares');
  assert.equal(plan.foundation.width, 7.25 * 2.45);
  assert.equal(plan.foundation.depth, 7.25 * 2.45);
  assert.equal(plan.floors.length, 19);
  assert.equal(plan.walls.length, 16);
  assert.equal(plan.wallFaces.length, 36);
  assert.deepEqual(plan.focus, { x: 0, y: 0, z: 0 });
  assert.deepEqual(plan.floors.find(cell => cell.x === 3 && cell.y === 4).world,
    { x: 0, y: 0, z: 2.45 });
  assert.ok(Object.isFrozen(plan.floors[0].world));
  assert.ok(Object.isFrozen(plan));
});

test('positions living party and enemy actors plus legal move and attack markers', () => {
  const plan = tacticsSpatialRenderPlan(frame());
  assert.deepEqual(plan.party[0].world, { x: -4.9, y: 0, z: 4.9 });
  assert.equal(plan.party[1].world, null);
  assert.deepEqual(plan.enemies[0].world, { x: 0, y: 0, z: 4.9 });
  assert.deepEqual(plan.markers.moves[0].world, { x: -4.9, y: 0, z: 2.45 });
  assert.deepEqual(plan.markers.targets[0].world, { x: 0, y: 0, z: 4.9 });
  assert.equal(plan.markers.targets[0].enemyId, 'corrupted-pawn');
});

test('deterministic center, scale changes, torches and malformed coordinates', () => {
  const input = frame();
  assert.deepEqual(tacticsCellToWorld(input, 3, 3), { x: 0, y: 0, z: 0 });
  assert.deepEqual(tacticsCellToWorld(input, 4, 2, 2), { x: 2, y: 0, z: -2 });
  assert.throws(() => tacticsCellToWorld(input, 7, 1), /Tactics/i);
  assert.throws(() => tacticsSpatialRenderPlan({ version: 99 }), /Tactics/i);
  assert.deepEqual(tacticsSpatialRenderPlan(input).torches, [
    { x: 1, y: 5, world: { x: -4.9 - 0.98, y: 0, z: 4.9 - 0.78 } },
    { x: 5, y: 5, world: { x: 4.9 + 0.94, y: 0, z: 4.9 - 0.7 } },
    { x: 1, y: 3, world: { x: -4.9 - 0.88, y: 0, z: 0 - 0.82 } },
    { x: 5, y: 3, world: { x: 4.9 + 0.92, y: 0, z: 0 - 0.72 } },
    { x: 2, y: 1, world: { x: -2.45 - 0.72, y: 0, z: -4.9 - 0.92 } },
    { x: 5, y: 1, world: { x: 4.9 + 0.72, y: 0, z: -4.9 - 0.92 } },
  ]);
});

test('the spatial renderer is data-only and never touches the authoritative world', () => {
  const input = frame();
  const before = JSON.stringify(input);
  const plan = tacticsSpatialRenderPlan(input);
  assert.equal(JSON.stringify(input), before);
  assert.equal(plan.enemies.length, 1);
  assert.equal(plan.party.length, 2);
  assert.equal(tacticsRemoteMapTopology({
    source: 'remote', runStatus: 'active', runId: 'real-run',
    currentMapId: 'crypt-eight-squares',
    map: { id: 'crypt-eight-squares', grid: cryptGrid },
  }).floors.length, plan.floors.length);
});
