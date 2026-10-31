import assert from 'node:assert/strict';
import test from 'node:test';
import { createTacticsRunApi, TacticsApiError } from '../src/browser/tacticsRunApi.js';

function world(overrides = {}) {
  return {
    source: 'remote', runStatus: 'active', runId: 'run-1',
    currentMapId: 'crypt', worldVersion: 8,
    map: { id: 'crypt', title: 'Cripta', grid: ['####', '#..#', '####'] },
    state: { mapId: 'crypt', x: 1, y: 1, phase: 'explore' },
    ...overrides,
  };
}
function fixture({ reply = world(), httpStatus = 200, token = 'signed-tactics-token' } = {}) {
  const requests = [];
  const audience = [];
  const api = createTacticsRunApi({
    baseURL: 'https://api-tactics.example.test/',
    approvedOrigin: 'https://api-tactics.example.test',
    runId: 'run-1',
    getAccessToken: async ({ audience: aud }) => { audience.push(aud); return token; },
    fetchImpl: async (url, options) => {
      requests.push({ url, options });
      return {
        ok: httpStatus === 200,
        status: httpStatus,
        headers: { get(name) {
          if (name === 'content-type') return 'application/json; charset=utf-8';
          if (name === 'content-length') return null;
          return null;
        } },
        text: async () => JSON.stringify(reply),
      };
    },
  });
  return { api, requests, audience };
}

test('bootstrap uses only explicit origin, Tactics-audience bearer and no credentials', async () => {
  const { api, requests, audience } = fixture();
  const got = await api.loadAuthorizedWorld();
  assert.equal(got.runId, 'run-1');
  assert.equal(got.worldVersion, 8);
  assert.deepEqual(audience, ['tactics']);
  assert.deepEqual(requests.map(r => r.url), [
    'https://api-tactics.example.test/api/tactics/runs/run-1/bootstrap',
  ]);
  assert.equal(requests[0].options.method, 'GET');
  assert.equal(requests[0].options.headers.Authorization, 'Bearer signed-tactics-token');
  assert.equal(requests[0].options.credentials, 'omit');
  assert.equal(requests[0].options.cache, 'no-store');
  assert.equal(requests[0].options.redirect, 'error');
  assert.equal(requests[0].options.referrerPolicy, 'no-referrer');
});

test('only legal action properties and current world version reach POST', async () => {
  const { api, requests } = fixture({ reply: world({ worldVersion: 9, state: { x: 2 } }) });
  const previous = { runId: 'run-1', worldVersion: 8,
    frame: { selectedMemberId: 'rook' } };
  const got = await api.commitTransition({
    previous, action: { kind: 'move', x: 2, y: 1, fakeHp: 9999 },
  });
  assert.equal(got.worldVersion, 9);
  assert.equal(requests[0].options.method, 'POST');
  assert.deepEqual(JSON.parse(requests[0].options.body), {
    expectedWorldVersion: 8,
    action: { kind: 'move', memberId: 'rook', x: 2, y: 1 },
  });
});

test('attack uses initiative actor but cannot send arbitrary client world state', async () => {
  const { api, requests } = fixture({ reply: world({ worldVersion: 9 }) });
  await api.commitTransition({
    previous: { runId: 'run-1', worldVersion: 8,
      frame: { selectedMemberId: 'matthias' } },
    actorMemberId: 'rook',
    action: { kind: 'attack', enemyId: 'pawn', state: { hp: 999 } },
  });
  assert.deepEqual(JSON.parse(requests[0].options.body).action,
    { kind: 'attack', memberId: 'rook', enemyId: 'pawn' });
});

test('origin, remote HTTP and token source are pinned before network calls', async () => {
  for (const baseURL of [
    'http://api-tactics.example.test/', 'https://evil.example/',
    'https://alice:pw@api-tactics.example.test/',
    'https://api-tactics.example.test/path',
    'https://api-tactics.example.test/?token=secret',
  ]) assert.throws(() => createTacticsRunApi({
    baseURL, approvedOrigin: 'https://api-tactics.example.test',
    runId: 'run-1', getAccessToken: async () => 'token', fetchImpl: async () => ({}),
  }), /origin/i);
  const { api, requests } = fixture({ token: 'bad token' });
  await assert.rejects(api.loadAuthorizedWorld(), error =>
    error instanceof TacticsApiError && error.code === 'authentication');
  assert.equal(requests.length, 0);
});

test('foreign runs, malformed maps or stale server version fail without trusting response', async () => {
  for (const reply of [
    world({ runId: 'foreign-run' }),
    world({ source: 'local' }),
    world({ map: { id: 'wrong', grid: ['###','#.#','###'] } }),
    world({ state: null }),
    world({ worldVersion: -1 }),
  ]) {
    const { api } = fixture({ reply });
    await assert.rejects(api.loadAuthorizedWorld(), error =>
      error instanceof TacticsApiError && error.code === 'invalid-world');
  }
  const { api } = fixture({ reply: world({ worldVersion: 8 }) });
  await assert.rejects(api.commitTransition({
    previous: { runId: 'run-1', worldVersion: 8, frame: { selectedMemberId: 'rook' } },
    action: { kind: 'move', x: 2, y: 1 },
  }), error => error.code === 'version-conflict');
});

test('HTTP errors do not leak response content or access token', async () => {
  const { api } = fixture({ httpStatus: 403 });
  await assert.rejects(api.loadAuthorizedWorld(), error => {
    assert.equal(error.status, 403);
    assert.equal(error.code, 'http');
    assert.doesNotMatch(error.message, /signed-tactics-token|run-1|server/i);
    return true;
  });
});

test('invalid action or mismatched run never causes network traffic', async () => {
  const { api, requests } = fixture();
  await assert.rejects(api.commitTransition({
    previous: { runId: 'wrong', worldVersion: 8 },
    action: { kind: 'move', x: 2, y: 1 },
  }), /Tactics/i);
  await assert.rejects(api.commitTransition({
    previous: { runId: 'run-1', worldVersion: 8, frame: { selectedMemberId: 'rook' } },
    action: { kind: 'attack', enemyId: 'bad/id' },
  }), /Tactics/i);
  assert.equal(requests.length, 0);
});
