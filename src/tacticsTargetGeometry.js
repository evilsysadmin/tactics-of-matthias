// Pure Tactics target geometry. No map, renderer, React, API or MM3 imports.
// Tile and enemy snapshots are injected by the current game's runtime.
export function tacticsAttackDistance(origin, position, profile) {
  const dx = Math.abs(position.x - origin.x);
  const dy = Math.abs(position.y - origin.y);
  if (profile.attackPattern === 'adjacent') return dx + dy === 1 ? 1 : null;
  if (profile.attackPattern === 'orthogonal') {
    if (dx !== 0 && dy !== 0) return null;
    return dx + dy;
  }
  if (profile.attackPattern === 'diagonal') return dx === dy && dx > 0 ? dx : null;
  if (profile.attackPattern === 'line') {
    if (!(dx === 0 || dy === 0 || dx === dy)) return null;
    return Math.max(dx, dy);
  }
  return null;
}


export function tacticsLineIsClear(from, to, { tileAt, positionedEnemies = [], ignoredEnemyId = null }) {
  const spanX = Math.abs(to.x - from.x);
  const spanY = Math.abs(to.y - from.y);
  const straight = spanX === 0 || spanY === 0;
  const diagonal = spanX === spanY;
  if (!straight && !diagonal) return false;

  const dx = Math.sign(to.x - from.x);
  const dy = Math.sign(to.y - from.y);
  let x = from.x + dx;
  let y = from.y + dy;
  while (x !== to.x || y !== to.y) {
    if (tileAt(x, y) === '#') return false;
    const blocked = positionedEnemies.some(({ enemy, position }) => (
      enemy.id !== ignoredEnemyId && position.x === x && position.y === y
    ));
    if (blocked) return false;
    x += dx;
    y += dy;
  }
  return true;
}
