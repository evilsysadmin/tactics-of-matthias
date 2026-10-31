import assert from 'node:assert/strict';
import test from 'node:test';
import { createTacticsGameHost } from '../src/browser/tacticsGameHost.js';
import { tacticsSceneModelFromRemoteWorld } from '../src/adapters/tacticsWorldScene.js';

const WORLD = {
  source: 'remote', runStatus: 'active', runId: 'server-run',
  worldVersion: 3, currentMapId: 'room',
  map: { id: 'room', title: 'Server room', grid: ['####', '#..#', '####'] },
};
function setup({ loader = async () => WORLD,
  commit = async ({ next }) => ({ ...WORLD, worldVersion: WORLD.worldVersion + 1, state: next.state }),
  renderFailure = false, projectionFailure = false } = {}) {
  const views = [];
  const statuses = [];
  const ports = {
    hydrateState: world => ({ mapId: world.currentMapId, x: world.state?.x ?? 1, y: world.state?.y ?? 1, phase: 'explore' }),
    projectScene: state => {
      if (projectionFailure && state.x === 2) throw new Error('invalid projected scene');
      return tacticsSceneModelFromRemoteWorld(WORLD, state, {
      selectedMemberId: 'rook', focusCell: { x: state.x, y: state.y },
      party: [{ id: 'rook', visible: true, hpRatio: 1, cell: { x: state.x, y: state.y } }],
      enemies: [], interaction: { mode: 'move',
        legalMoves: state.x === 1 ? [{ x: 2, y: 1 }] : [], legalTargets: [] },
      });
    },
    actionPorts: {
      partyCanAct: () => true, currentActor: () => null,
      memberPosition: state => ({ x: state.x, y: state.y }),
      legalMoves: state => state.x === 1 ? [{ x: 2, y: 1 }] : [],
      move: (state, target) => ({ ...state, x: target.x, y: target.y }),
      targets: () => [], attack: () => { throw Error('attack not enabled'); },
      resolveAction: (_before, next) => next,
    },
  };
  const host = createTacticsGameHost({
    element: {}, loadAuthorizedWorld: loader, commitTransition: commit, ports,
    viewportFactory: ({ frame, onAction }) => {
      const view = { frame, onAction, disposed: false, updates: 0,
        updateFrame(next) {
          if (renderFailure) throw Error('WebGL lost');
          this.frame = next;
          this.updates++;
        },
        dispose() { this.disposed = true; } };
      views.push(view);
      return view;
    },
    onStatus: event => statuses.push(event),
  });
  return { host, views, statuses };
}

test('auth-controlled loader gates real scene creation and teardown', async () => {
  const h = setup();
  assert.equal(await h.host.dispatchAction({ kind: 'move', x: 2, y: 1 }), false);
  assert.equal(h.views.length, 0);
  assert.equal(await h.host.start(), true);
  assert.equal(h.host.status, 'ready');
  assert.equal(h.host.session.frame.map.floors.length, 2);
  assert.equal(h.views.length, 1);
  h.host.dispose();
  h.host.dispose();
  assert.equal(h.host.status, 'disposed');
  assert.equal(h.host.session, null);
  assert.equal(h.views[0].disposed, true);
});

test('only acknowledged, legal actions may reveal a new scene', async () => {
  let commits = 0;
  const h = setup({ commit: async ({ next }) => { commits++; return { ...WORLD, worldVersion: 4, state: next.state }; } });
  await h.host.start();
  assert.equal(await h.host.dispatchAction({ kind: 'attack', enemyId: 'ghost' }), false);
  assert.equal(await h.host.dispatchAction({ kind: 'member', memberId: 'rook' }), false);
  assert.equal(await h.host.dispatchAction({ kind: 'move', x: 2, y: 1 }), true);
  assert.equal(commits, 1);
  assert.equal(h.host.session.revision, 1);
  assert.equal(h.views[0].updates, 1);
  assert.equal(h.host.session.frame.focusCell.x, 2);
  assert.equal(await h.host.dispatchAction({ kind: 'move', x: 2, y: 1 }), false);
  h.host.dispose();
});

test('checkpoint rejection blocks further input without altering current frame', async () => {
  const h = setup({ commit: async () => false });
  await h.host.start();
  assert.equal(await h.host.dispatchAction({ kind: 'move', x: 2, y: 1 }), false);
  assert.equal(h.host.status, 'error');
  assert.equal(h.host.session.revision, 0);
  assert.equal(h.views[0].updates, 0);
  assert.equal(await h.host.dispatchAction({ kind: 'move', x: 2, y: 1 }), false);
  h.host.dispose();
});

test('stale async bootstrap cannot mount a superseded world', async () => {
  let finish;
  let calls = 0;
  const h = setup({ loader: () => {
    calls++;
    return calls === 1 ? new Promise(resolve => { finish = resolve; }) : Promise.resolve(WORLD);
  } });
  const first = h.host.start();
  const second = h.host.start();
  finish(WORLD);
  assert.equal(await first, false);
  assert.equal(await second, true);
  assert.equal(h.views.length, 1);
  h.host.dispose();
});

test('rejects untrusted loader results and cannot render a fallback', async () => {
  const h = setup({ loader: async () => ({ ...WORLD, source: 'local' }) });
  assert.equal(await h.host.start(), false);
  assert.equal(h.host.status, 'error');
  assert.equal(h.views.length, 0);
  h.host.dispose();
});

test('server acknowledgment followed by render error requires fresh bootstrap', async () => {
  const h = setup({ renderFailure: true });
  await h.host.start();
  assert.equal(await h.host.dispatchAction({ kind: 'move', x: 2, y: 1 }), false);
  assert.equal(h.host.status, 'error');
  assert.equal(h.host.session.revision, 0);
  assert.equal(await h.host.start(), true);
  assert.equal(h.views[0].disposed, true);
  h.host.dispose();
});

test('simultaneous clicks cannot commit the same turn twice', async () => {
  let done;
  const h = setup({ commit: () => new Promise(resolve => { done = resolve; }) });
  await h.host.start();
  const first = h.host.dispatchAction({ kind: 'move', x: 2, y: 1 });
  assert.equal(await h.host.dispatchAction({ kind: 'move', x: 2, y: 1 }), false);
  done({ ...WORLD, worldVersion: 4, state: { x: 2, y: 1 } });
  assert.equal(await first, true);
  assert.equal(h.host.session.revision, 1);
  h.host.dispose();
});

test('projection failure during input is handled without commit or unhandled pointer rejection', async () => {
  let committed = false;
  const h = setup({ projectionFailure: true, commit: async () => {
    committed = true;
    return { ...WORLD, worldVersion: 4 };
  } });
  await h.host.start();
  assert.equal(await h.host.dispatchAction({ kind: 'move', x: 2, y: 1 }), false);
  assert.equal(committed, false);
  assert.equal(h.host.status, 'error');
  assert.equal(h.host.session.revision, 0);
  h.host.dispose();
});

test('server outcome, not a locally predicted move, defines the displayed world', async () => {
  const h = setup({ commit: async () => ({
    ...WORLD, worldVersion: 4, state: { x: 1, y: 1 },
  }) });
  await h.host.start();
  assert.equal(await h.host.dispatchAction({ kind: 'move', x: 2, y: 1 }), true);
  assert.equal(h.host.session.worldVersion, 4);
  assert.equal(h.host.session.state.x, 1);
  assert.equal(h.views[0].frame.focusCell.x, 1);
  h.host.dispose();
});

test('mismatched run/version response stops gameplay without displaying prediction', async () => {
  for (const response of [
    true,
    { ...WORLD, worldVersion: 3, state: { x: 2, y: 1 } },
    { ...WORLD, runId: 'someone-elses-run', worldVersion: 4, state: { x: 2, y: 1 } },
    { ...WORLD, worldVersion: 4, source: 'local', state: { x: 2, y: 1 } },
  ]) {
    const h = setup({ commit: async () => response });
    await h.host.start();
    assert.equal(await h.host.dispatchAction({ kind: 'move', x: 2, y: 1 }), false);
    assert.equal(h.host.status, 'error');
    assert.equal(h.host.session.revision, 0);
    assert.equal(h.views[0].updates, 0);
    h.host.dispose();
  }
});
