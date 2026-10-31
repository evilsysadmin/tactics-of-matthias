import { tacticsRemoteMapTopology } from '../adapters/tacticsWorldScene.js';

const RUN_ID = /^[A-Za-z0-9_-]{1,128}$/;
const MAX_RESPONSE_BYTES = 1024 * 1024;

export class TacticsApiError extends Error {
  constructor(code, status = null) {
    super('Tactics API request could not be completed');
    this.name = 'TacticsApiError';
    this.code = code;
    this.status = status;
  }
}

function approvedEndpoint(baseURL, approvedOrigin) {
  let endpoint;
  try { endpoint = new URL(baseURL); } catch { throw new TypeError('Invalid Tactics API origin'); }
  const local = endpoint.protocol === 'http:'
    && ['localhost', '127.0.0.1', '[::1]'].includes(endpoint.hostname);
  if (!['https:', ...(local ? ['http:'] : [])].includes(endpoint.protocol)
    || endpoint.username || endpoint.password || endpoint.search || endpoint.hash
    || endpoint.pathname !== '/' || endpoint.origin !== approvedOrigin) {
    throw new TypeError('Tactics API endpoint must match an explicitly approved origin');
  }
  return endpoint.origin;
}

function validateWorld(world, runId) {
  if (!world || typeof world !== 'object' || Array.isArray(world)
    || world.source !== 'remote' || world.runId !== runId
    || world.runStatus !== 'active'
    || !Number.isSafeInteger(world.worldVersion) || world.worldVersion < 0
    || !world.state || typeof world.state !== 'object' || Array.isArray(world.state)) {
    throw new TacticsApiError('invalid-world');
  }
  try { tacticsRemoteMapTopology(world); }
  catch { throw new TacticsApiError('invalid-world'); }
  return world;
}

async function validatedToken(getAccessToken, signal) {
  const token = await getAccessToken({ audience: 'tactics', signal });
  if (typeof token !== 'string' || !token || token.length > 8192
    || /\s/.test(token)) throw new TacticsApiError('authentication');
  return token;
}

function validatedAction(action, memberId) {
  if (typeof memberId !== 'string' || !RUN_ID.test(memberId)) {
    throw new TacticsApiError('invalid-action');
  }
  if (action?.kind === 'move'
    && Number.isSafeInteger(action.x) && action.x >= 0 && action.x < 4096
    && Number.isSafeInteger(action.y) && action.y >= 0 && action.y < 4096) {
    return { kind: 'move', memberId, x: action.x, y: action.y };
  }
  if (action?.kind === 'attack' && RUN_ID.test(action.enemyId || '')) {
    return { kind: 'attack', memberId, enemyId: action.enemyId };
  }
  throw new TacticsApiError('invalid-action');
}

// Safe HTTP transport: called only after an approved deployment config supplies
// its API origin and a genuine ShadowOps TACTICS-audience token exchange.
// This is NOT an identity provider and cannot create runs or local save states.
export function createTacticsRunApi({
  baseURL, approvedOrigin, runId, getAccessToken,
  fetchImpl = globalThis.fetch,
} = {}) {
  const origin = approvedEndpoint(baseURL, approvedOrigin);
  if (!RUN_ID.test(runId || '') || typeof getAccessToken !== 'function'
    || typeof fetchImpl !== 'function') {
    throw new TypeError('Tactics API needs run ID, real SSO token source and fetch');
  }

  const request = async (method, route, body, signal) => {
    const token = await validatedToken(getAccessToken, signal);
    const options = {
      method, signal, cache: 'no-store', credentials: 'omit',
      redirect: 'error', referrerPolicy: 'no-referrer',
      headers: {
        Authorization: 'Bearer ' + token,
        Accept: 'application/json',
        ...(body ? { 'Content-Type': 'application/json' } : {}),
      },
      ...(body ? { body: JSON.stringify(body) } : {}),
    };
    let response;
    try { response = await fetchImpl(origin + route, options); }
    catch { throw new TacticsApiError('network'); }
    if (!response?.ok) {
      throw new TacticsApiError('http', Number.isInteger(response?.status) ? response.status : null);
    }
    if (!response.headers?.get('content-type')?.toLowerCase().includes('application/json')) {
      throw new TacticsApiError('invalid-response');
    }
    const length = Number(response.headers.get('content-length'));
    if (Number.isFinite(length) && length > MAX_RESPONSE_BYTES) {
      throw new TacticsApiError('invalid-response');
    }
    let text;
    try { text = await response.text(); }
    catch { throw new TacticsApiError('invalid-response'); }
    if (text.length > MAX_RESPONSE_BYTES) throw new TacticsApiError('invalid-response');
    let result;
    try { result = JSON.parse(text); }
    catch { throw new TacticsApiError('invalid-response'); }
    return validateWorld(result, runId);
  };

  const route = '/api/tactics/runs/' + encodeURIComponent(runId);
  return Object.freeze({
    loadAuthorizedWorld({ signal } = {}) {
      return request('GET', route + '/bootstrap', null, signal);
    },
    async commitTransition({ previous, action, actorMemberId, signal } = {}) {
      if (!previous || previous.runId !== runId
        || !Number.isSafeInteger(previous.worldVersion) || previous.worldVersion < 0) {
        throw new TacticsApiError('invalid-run');
      }
      const memberId = actorMemberId || previous.frame?.selectedMemberId;
      const intent = validatedAction(action, memberId);
      const result = await request('POST', route + '/actions', {
        expectedWorldVersion: previous.worldVersion, action: intent,
      }, signal);
      if (result.worldVersion !== previous.worldVersion + 1) {
        throw new TacticsApiError('version-conflict', 409);
      }
      return result;
    },
  });
}
