// The renderer may suggest an intent but can never perform a game action.
// The authoritative tactical turn reducer MUST revalidate every returned intent.
export const TACTICS_ACTION_INTENT_VERSION = 1;

export function tacticsIntentFromFrame(frame, request) {
  if (frame?.version !== 1 || typeof frame?.map?.id !== 'string'
    || typeof frame?.selectedMemberId !== 'string' || !Array.isArray(frame?.party)) {
    throw new TypeError('Invalid Tactics renderer frame');
  }
  if (!request || typeof request !== 'object' || !frame.interaction) return null;
  const { mode, legalMoves, legalTargets } = frame.interaction;
  const memberId = frame.selectedMemberId;
  const member = frame.party.find(entry => entry.id === memberId);
  if (!member?.visible) return null;

  if (request.kind === 'move' && ['move', 'hybrid'].includes(mode)) {
    if (!Number.isSafeInteger(request.x) || !Number.isSafeInteger(request.y)) return null;
    const allowed = legalMoves?.find(cell => cell.x === request.x && cell.y === request.y);
    if (!allowed) return null;
    return Object.freeze({
      version: TACTICS_ACTION_INTENT_VERSION,
      mapId: frame.map.id,
      memberId,
      kind: 'move',
      destination: Object.freeze({ x: allowed.x, y: allowed.y }),
    });
  }

  if (request.kind === 'attack' && ['attack', 'hybrid'].includes(mode)) {
    if (typeof request.enemyId !== 'string' || !request.enemyId) return null;
    const target = legalTargets?.find(entry => entry.enemyId === request.enemyId);
    const enemy = frame.enemies?.find(entry => entry.id === request.enemyId && entry.visible);
    if (!target || !enemy) return null;
    return Object.freeze({
      version: TACTICS_ACTION_INTENT_VERSION,
      mapId: frame.map.id,
      memberId,
      kind: 'attack',
      enemyId: target.enemyId,
    });
  }

  // Do not invent "wait", "use" or "ability" legality: those need turn
  // metadata that the current v1 scene model deliberately does not contain.
  return null;
}
