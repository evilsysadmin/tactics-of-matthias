// Pure tactical turn boundary, extracted from the live Tactics mode.
// The owning game's real world/AI/initiative functions are passed as ports.
// Do not invent or import a substitute world: the caller owns authority.
export function tacticsResolvePlayerAction(
  previous,
  next,
  {
    forceCombat = false,
    forceEnemyIds = [],
    random = Math.random,
    ports,
  } = {},
) {
  if (!previous || !next) return previous || next;
  if (previous?.mapId && next?.mapId && previous.mapId !== next.mapId) {
    return ports.prepareExplorationSpawn({
      ...next,
      explorationEnemySteps: 0,
    });
  }
  if (previous.phase === 'escaped' || previous.phase === 'defeated') return previous;

  if (previous.initiative?.order?.length) {
    if (next === previous) return previous;
    return ports.advanceCombatInitiative(next, ports.activeEnemies(next));
  }

  const alreadyEngaged = ports.combatActive(previous);
  const movedIntoEngagement = ports.combatActive(next);
  const explorationNext = (!forceCombat && !alreadyEngaged && !movedIntoEngagement)
    ? ports.advanceExplorationEnemies(previous, next)
    : next;
  const shouldStartCombat = forceCombat
    || alreadyEngaged
    || ports.combatActive(explorationNext);
  if (!shouldStartCombat) return explorationNext;

  // Entering combat is a boundary, not a free attack. A move that newly enters
  // engagement completes before initiative starts; an already-engaged legacy
  // state or an explicit ranged attack rolls before committing the action.
  // Enemies may take one exploration step first; if that step closes contact,
  // initiative starts immediately and no exploration damage is dealt.
  const entryState = (forceCombat || alreadyEngaged) ? previous : explorationNext;
  const activeEnemies = ports.activeEnemies(entryState);
  const roomEnemyIds = activeEnemies.map((enemy) => enemy.id);
  const started = ports.startInitiativeCombat(
    entryState,
    activeEnemies,
    {
      // Tactics is a room-scale battle board. Once contact freezes exploration,
      // every living enemy on that board must belong to the same scheduler;
      // otherwise a distant patrol could wander into range without ever owning
      // an initiative turn.
      forceEnemyIds: [...new Set([...roomEnemyIds, ...forceEnemyIds])],
      random,
    },
  );
  if (
    started !== entryState
    && entryState === explorationNext
    && typeof explorationNext.message === 'string'
    && explorationNext.message
  ) {
    return {
      ...started,
      message: `${explorationNext.message} ${started.message}`,
    };
  }
  return started;
}
