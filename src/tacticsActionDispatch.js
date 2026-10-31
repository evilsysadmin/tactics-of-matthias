// State transition adapter used by the live Tactics UI.
// All game authority comes from the caller's *real* world/rules/initiative ports.
// No React, renderer, networking, saved runs or imitation game rules.
export function tacticsExecuteMove(state, dx, dy, ports) {
  if (!ports.partyCanAct(state)) return null;
  const actor = ports.currentActor(state);
  const memberId = actor?.kind === 'party' ? actor.id : null;
  const origin = memberId
    ? ports.memberPosition(state, memberId)
    : { x: state.x, y: state.y };
  const legal = ports.legalMoves(state, memberId).find((move) => (
    move.x === origin.x + dx && move.y === origin.y + dy
  ));
  if (!legal) return null;
  const moved = ports.move(state, legal, memberId);
  const next = ports.resolveAction(state, moved);
  return { next, actorMemberId: memberId, actionKind: 'move' };
}

export function tacticsExecuteAttack(state, enemyId, selectedMemberId, ports) {
  if (!ports.partyCanAct(state)) return null;
  const actor = ports.currentActor(state);
  const memberId = actor?.kind === 'party' ? actor.id : selectedMemberId;
  const targets = ports.targets(state, memberId);
  const target = enemyId
    ? targets.find((candidate) => candidate.enemyId === enemyId)
    : targets[0];
  if (!target) return null;
  const attacked = ports.attack(state, memberId, target.enemyId);
  const next = ports.resolveAction(state, attacked, {
    forceCombat: !state.initiative,
    forceEnemyIds: [target.enemyId],
  });
  return { next, actorMemberId: memberId, actionKind: 'attack' };
}
