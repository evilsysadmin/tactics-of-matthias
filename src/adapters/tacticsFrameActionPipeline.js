import { tacticsIntentFromFrame } from './tacticsActionIntent.js';
import { tacticsExecuteAttack, tacticsExecuteMove } from './tacticsActionDispatch.js';

// Receives an immutable v1 renderer frame and a raw user click; executes through
// the live game's rule ports, not through the frame's possibly stale permissions.
// No mutable state is committed here. The UI/runtime owns progression + saves.
export function tacticsDispatchFrameAction(state, frame, request, ports) {
  if (!state || typeof state !== 'object') return null;
  const intent = tacticsIntentFromFrame(frame, request);
  if (!intent || intent.mapId !== state.mapId) return null;

  // The selected party actor may have changed after the frame was projected.
  // Never let a stale click spend another character's initiative slot.
  const actor = ports.currentActor(state);
  if (actor && (actor.kind !== 'party' || actor.id !== intent.memberId)) return null;

  if (intent.kind === 'attack') {
    return tacticsExecuteAttack(state, intent.enemyId, intent.memberId, ports);
  }

  if (intent.kind === 'move') {
    const origin = actor?.kind === 'party'
      ? ports.memberPosition(state, actor.id)
      : { x: state.x, y: state.y };
    if (!origin || !Number.isSafeInteger(origin.x) || !Number.isSafeInteger(origin.y)) return null;
    return tacticsExecuteMove(state,
      intent.destination.x - origin.x,
      intent.destination.y - origin.y,
      ports);
  }

  return null;
}
