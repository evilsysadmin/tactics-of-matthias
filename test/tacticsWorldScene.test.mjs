import assert from 'node:assert/strict';
import test from 'node:test';
import { tacticsRemoteMapTopology, tacticsSceneModelFromRemoteWorld } from '../src/adapters/tacticsWorldScene.js';
import { tacticsSceneFrame } from '../src/adapters/tacticsSceneFrame.js';
import { tacticsCreateSession, tacticsApplySessionRequest } from '../src/tacticsSession.js';

// The actual 7×7 crypt grid from Chronicles' versioned authored manifest.
// Test-only reference: never bundled as a fallback playable world.
const CRYPT = Object.freeze([
  '#######', '#..X..#', '#.###.#', '#...#.#',
  '#.#S#.#', '#P.E..#', '#######',
]);
function world() {
  return {
    source: 'remote', runStatus: 'active',
    runId: 'remote-crypt-run', currentMapId: 'crypt-eight-squares', worldVersion: 1,
    map: { id: 'crypt-eight-squares', title: 'Cripta de las Ocho Casillas', grid: [...CRYPT] },
  };
}
function actors(state) {
  return {
    selectedMemberId: 'matthias',
    focusCell: { x: state.x, y: state.y },
    party: [{ id: 'matthias', visible: true, hpRatio: 1, cell: { x: state.x, y: state.y } }],
    enemies: [{ id: 'corrupted-pawn', visible: true, visualType: 'corrupted-pawn', cell: { x: 3, y: 5 } }],
    interaction: {
      mode: 'hybrid',
      legalMoves: [{ x: 1, y: 4 }],
      legalTargets: [{ enemyId: 'corrupted-pawn', x: 3, y: 5 }],
    },
  };
}

test('projects exact real crypt topology, wall exposure, center and authored tiles', () => {
  const topology = tacticsRemoteMapTopology(world());
  assert.equal(topology.mapId, 'crypt-eight-squares');
  assert.equal(topology.width, 7);
  assert.equal(topology.height, 7);
  assert.deepEqual(topology.center, { x: 3, y: 3 });
  assert.equal(topology.floors.length, 19);
  assert.equal(topology.walls.length, 16);
  assert.equal(topology.wallFaces.length, 36);
  assert.ok(topology.floors.some(cell => cell.x === 3 && cell.y === 4 && cell.tile === 'S'));
  assert.ok(topology.wallFaces.some(face => face.x === 2 && face.y === 6 && face.side === 'north'));
  assert.ok(Object.isFrozen(topology.floors[0]));
  assert.ok(Object.isFrozen(topology.wallFaces[0]));
});

test('fails closed for local worlds, map mismatches, empty or malformed terrain', () => {
  const base = world();
  for (const bad of [
    { ...base, source: 'local' },
    { ...base, runStatus: 'defeated' },
    { ...base, map: { ...base.map, id: 'other' } },
    { ...base, map: { ...base.map, grid: [] } },
    { ...base, map: { ...base.map, grid: ['##', '#'] } },
    { ...base, map: { ...base.map, grid: ['###', '###'] } },
    { ...base, map: { ...base.map, grid: ['#😊#'] } },
  ]) assert.throws(() => tacticsRemoteMapTopology(bad), /Tactics/i);
});

test('derives current scene from authoritative topology and real projected actors', () => {
  const state = { mapId: 'crypt-eight-squares', x: 1, y: 5, phase: 'explore' };
  const projected = tacticsSceneModelFromRemoteWorld(world(), state, actors(state));
  const frame = tacticsSceneFrame(projected);
  assert.equal(frame.map.floors.length, 19);
  assert.equal(frame.map.wallFaces.length, 36);
  assert.deepEqual(frame.focusCell, { x: 1, y: 5 });
  assert.equal(frame.partyFormation, 'explore-compact');
  assert.deepEqual(frame.interaction.legalTargets, [{
    enemyId: 'corrupted-pawn', x: 3, y: 5,
  }]);
  assert.equal(frame.map.id, state.mapId);
  assert.throws(() => tacticsSceneModelFromRemoteWorld(world(),
    { ...state, mapId: 'unknown' }, actors(state)), /map/i);
  assert.throws(() => tacticsSceneModelFromRemoteWorld(world(), state),
    /live actor/i);
});

test('the headless session now accepts *authoritative map terrain*, not preassembled scene geometry', () => {
  const remote = world();
  const ports = {
    hydrateState: () => ({ mapId: remote.currentMapId, phase: 'explore', x: 1, y: 5, turns: 0 }),
    projectScene: state => tacticsSceneModelFromRemoteWorld(remote, state, actors(state)),
    actionPorts: {
      currentActor: () => null,
      partyCanAct: () => true,
      legalMoves: () => [{ x: 1, y: 4 }],
      memberPosition: state => ({ x: state.x, y: state.y }),
      move: (state, destination) => ({ ...state, x: destination.x, y: destination.y }),
      targets: () => [],
      attack: () => { throw new Error('No attacks without real enemy rules'); },
      resolveAction: (_previous, next) => next,
    },
  };
  const first = tacticsCreateSession(remote, ports);
  assert.equal(first.frame.map.floors.length, 19);
  assert.equal(first.frame.map.id, remote.currentMapId);
  const next = tacticsApplySessionRequest(first,
    { kind: 'move', x: 1, y: 4, revision: first.revision }, ports);
  assert.ok(next);
  assert.equal(next.session.frame.focusCell.y, 4);
  assert.equal(next.session.revision, 1);
  assert.equal(first.state.y, 5);
});
