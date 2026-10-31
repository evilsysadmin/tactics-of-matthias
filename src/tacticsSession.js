import { tacticsSceneFrame } from './adapters/tacticsSceneFrame.js';
import { tacticsDispatchFrameAction } from './adapters/tacticsFrameActionPipeline.js';

function requirePorts(ports) {
  if (typeof ports?.hydrateState !== 'function'
    || typeof ports?.projectScene !== 'function'
    || !ports?.actionPorts) {
    throw new TypeError('Tactics session requires real world, scene and action ports');
  }
}

function project(state, ports) {
  if (!state || typeof state !== 'object' || typeof state.mapId !== 'string') {
    throw new TypeError('Invalid Tactics runtime state');
  }
  const frame = tacticsSceneFrame(ports.projectScene(state));
  if (frame.map.id !== state.mapId) {
    throw new TypeError('Tactics runtime scene and state map mismatch');
  }
  return frame;
}

// A session is permitted only after the hosting game validates a *remote*
// authoritative run. A string marker is a contract, not authentication:
// SSO and backend authority must be enforced by the upstream bootstrap.
export function tacticsCreateSession(world, ports) {
  requirePorts(ports);
  if (world?.source !== 'remote'
    || typeof world.runId !== 'string' || !world.runId
    || typeof world.currentMapId !== 'string' || !world.currentMapId
    || !Number.isSafeInteger(world.worldVersion) || world.worldVersion < 0
    || world.runStatus !== 'active') {
    throw new TypeError('Tactics requires a validated active remote run');
  }
  const state = ports.hydrateState(world);
  if (state?.mapId !== world.currentMapId) {
    throw new TypeError('Tactics runtime map does not match remote run');
  }
  return Object.freeze({
    runId: world.runId,
    worldVersion: world.worldVersion,
    revision: 0,
    state,
    frame: project(state, ports),
  });
}

// No writes, timers, local fixtures or fake world fallback. The owning UI
// commits progression/checkpoints after confirming this state transition.
export function tacticsApplySessionRequest(session, request, ports) {
  requirePorts(ports);
  if (!session || !Number.isSafeInteger(session.revision) || session.revision < 0
    || !request || request.revision !== session.revision) return null;
  const outcome = tacticsDispatchFrameAction(
    session.state, session.frame, request, ports.actionPorts,
  );
  if (!outcome || !outcome.next || outcome.next === session.state) return null;
  const nextSession = Object.freeze({
    ...session,
    revision: session.revision + 1,
    state: outcome.next,
    frame: project(outcome.next, ports),
  });
  return Object.freeze({
    session: nextSession,
    actionKind: outcome.actionKind,
    actorMemberId: outcome.actorMemberId,
  });
}


// The browser must display the SERVER'S resolved state, not a local prediction
// merely because the API returned 2xx. This also handles server-side RNG,
// enemy reactions and cross-map movement without trusting client checkpoints.
export function tacticsReconcileServerSession(previous, world, ports) {
  requirePorts(ports);
  if (!previous || !Number.isSafeInteger(previous.revision)
    || world?.source !== 'remote' || world?.runStatus !== 'active'
    || world?.runId !== previous.runId
    || !Number.isSafeInteger(world?.worldVersion)
    || world.worldVersion !== previous.worldVersion + 1
    || typeof world?.currentMapId !== 'string' || !world.currentMapId) {
    throw new TypeError('Tactics server response has incompatible run identity or world version');
  }
  const state = ports.hydrateState(world);
  if (state?.mapId !== world.currentMapId) {
    throw new TypeError('Tactics authoritative state map mismatch');
  }
  return Object.freeze({
    runId: previous.runId,
    worldVersion: world.worldVersion,
    revision: previous.revision + 1,
    state,
    frame: project(state, ports),
  });
}

/**
 * Read-only terminal records are NOT active game sessions. Never hydrate an
 * actionable scene or synthesize a local world from a terminal server run.
 */
export function tacticsReadOnlyTerminal(world, previous = null) {
  const terminal = world?.runStatus === 'defeated' || world?.runStatus === 'completed';
  const phase = world?.state?.phase;
  const matchingPhase = world?.runStatus === 'defeated'
    ? phase === 'defeated'
    : phase === 'completed' || phase === 'escaped';
  const party = world?.state?.party;
  const partyValid = Array.isArray(party) && party.length > 0
    && party.length <= 16 && party.every(member =>
      typeof member?.id === 'string' && member.id.length > 0
      && Number.isSafeInteger(member.hp) && member.hp >= 0);
  if (!terminal || world.source !== 'remote'
    || typeof world.runId !== 'string' || !world.runId
    || typeof world.currentMapId !== 'string' || !world.currentMapId
    || world.map?.id !== world.currentMapId
    || world.state?.mapId !== world.currentMapId
    || !partyValid || !matchingPhase
    || (world.runStatus === 'defeated' && party.some(member => member.hp !== 0))
    || !Number.isSafeInteger(world.worldVersion) || world.worldVersion < 0
    || (previous && (world.runId !== previous.runId
      || world.worldVersion !== previous.worldVersion + 1))) {
    throw new TypeError('Tactics terminal outcome is not a valid authorized run');
  }
  return Object.freeze({
    runId: world.runId,
    worldVersion: world.worldVersion,
    revision: previous ? previous.revision + 1 : 0,
    runStatus: world.runStatus,
    state: null,
    frame: null,
  });
}
