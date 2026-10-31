import assert from 'node:assert/strict';
import test from 'node:test';
import { tacticsCreateSession, tacticsApplySessionRequest, tacticsReconcileServerSession } from '../src/tacticsSession.js';

function remote() {
  return {
    source: 'remote', runId: 'authorized-run-123',
    currentMapId: 'crypt-eight-squares', worldVersion: 8, runStatus: 'active',
  };
}

function ports() {
  let projections = 0;
  const controls = {
    legalMove: true,
    calls: [],
  };
  const makeFrame = state => {
    projections += 1;
    return {
      version: 1, mapId: state.mapId,
      scenePlan: {
        mapId: state.mapId, title: 'Cripta', width: 4, height: 4,
        center: { x: 1.5, y: 1.5 },
        floors: [
          { x: 1, y: 1, tile: '.' }, { x: 2, y: 1, tile: '.' },
          { x: 3, y: 1, tile: '.' },
        ],
        walls: [], wallFaces: [],
      },
      focusCell: { x: state.x, y: state.y },
      selectedMemberId: 'rook', partyFormation: 'explore-compact',
      party: [{ id: 'rook', visible: true, hpRatio: 1, cell: { x: state.x, y: state.y } }],
      enemies: [],
      interaction: {
        mode: 'move',
        legalMoves: [{ x: 2, y: 1 }], legalTargets: [],
      },
    };
  };
  const harness = {
    hydrateState: world => ({
      mapId: world.currentMapId, phase: 'explore', x: 1, y: 1, turns: 0,
    }),
    projectScene: makeFrame,
    actionPorts: {
      partyCanAct: state => state.phase === 'explore',
      currentActor: () => null,
      memberPosition: () => ({ x: 1, y: 1 }),
      legalMoves: () => controls.legalMove ? [{ x: 2, y: 1 }] : [],
      move: (state, cell) => {
        controls.calls.push('move');
        return { ...state, x: cell.x, y: cell.y };
      },
      targets: () => [],
      attack: () => { throw new Error('Cannot attack without real target'); },
      resolveAction: (_previous, next) => next,
    },
  };
  return { harness, controls, projectionCount: () => projections };
}

test('requires remote validated run identity and never constructs a local stand-in', () => {
  const { harness } = ports();
  assert.throws(() => tacticsCreateSession(null, harness), /remote/i);
  assert.throws(() => tacticsCreateSession({ ...remote(), source: 'local' }, harness), /remote/i);
  assert.throws(() => tacticsCreateSession({ ...remote(), runStatus: 'defeated' }, harness), /remote/i);
  assert.throws(() => tacticsCreateSession({ ...remote(), worldVersion: -1 }, harness), /remote/i);
  assert.throws(() => tacticsCreateSession(remote(), { ...harness,
    hydrateState: () => ({ mapId: 'other' }),
  }), /map/i);
  assert.throws(() => tacticsCreateSession(remote(), { ...harness,
    projectScene: () => ({ version: 2 }),
  }), /version/i);
});

test('a valid turn evolves session revision and reprojects scene from new runtime state', () => {
  const { harness, controls, projectionCount } = ports();
  const before = tacticsCreateSession(remote(), harness);
  assert.equal(before.runId, 'authorized-run-123');
  assert.equal(before.worldVersion, 8);
  assert.equal(before.revision, 0);
  assert.equal(before.frame.focusCell.x, 1);
  assert.ok(Object.isFrozen(before));
  const performed = tacticsApplySessionRequest(before,
    { revision: 0, kind: 'move', x: 2, y: 1 }, harness);
  assert.deepEqual(controls.calls, ['move']);
  assert.equal(performed.actionKind, 'move');
  assert.equal(performed.session.revision, 1);
  assert.equal(performed.session.frame.focusCell.x, 2);
  assert.equal(performed.session.state.x, 2);
  assert.equal(before.state.x, 1);
  assert.equal(before.revision, 0);
  assert.equal(projectionCount(), 2);
  assert.equal(performed.session.runId, before.runId);
});

test('stale event revision and newly illegal actions cannot mutate session', () => {
  const { harness, controls, projectionCount } = ports();
  const session = tacticsCreateSession(remote(), harness);
  assert.equal(tacticsApplySessionRequest(session,
    { revision: 2, kind: 'move', x: 2, y: 1 }, harness), null);
  controls.legalMove = false;
  assert.equal(tacticsApplySessionRequest(session,
    { revision: 0, kind: 'move', x: 2, y: 1 }, harness), null);
  assert.equal(tacticsApplySessionRequest(session,
    { revision: 0, kind: 'attack', enemyId: 'ghost' }, harness), null);
  assert.equal(projectionCount(), 1);
  assert.deepEqual(controls.calls, []);
});

test('a projection error fails closed instead of committing the action', () => {
  const { harness } = ports();
  const session = tacticsCreateSession(remote(), harness);
  assert.throws(() => tacticsApplySessionRequest(session,
    { revision: 0, kind: 'move', x: 2, y: 1 },
    { ...harness, projectScene: () => ({ version: 99 }) }), /version/i);
  assert.equal(session.revision, 0);
  assert.equal(session.state.x, 1);
});

test('server reconciliation requires exact run identity and next remote world version', () => {
  const { harness } = ports();
  const current = tacticsCreateSession(remote(), harness);
  const updated = { ...remote(), worldVersion: 9 };
  const confirmed = tacticsReconcileServerSession(current, updated, harness);
  assert.equal(confirmed.worldVersion, 9);
  assert.equal(confirmed.revision, 1);
  assert.ok(Object.isFrozen(confirmed));
  for (const invalid of [
    { ...updated, source: 'local' },
    { ...updated, runId: 'wrong' },
    { ...updated, worldVersion: 8 },
    { ...updated, worldVersion: 10 },
    { ...updated, runStatus: 'defeated' },
  ]) assert.throws(() => tacticsReconcileServerSession(current, invalid, harness), /Tactics/i);
});
