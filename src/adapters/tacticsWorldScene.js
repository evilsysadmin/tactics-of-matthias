// Reproduces the topology projection used by Chronicles' isometric scene plan,
// but only from an already server-validated *current* map manifest. No local
// maps, imports, network calls, invented actors or fallback terrain.
export const TACTICS_WORLD_SCENE_VERSION = 1;

function validatedMap(world) {
  if (world?.source !== 'remote' || world?.runStatus !== 'active'
    || typeof world.runId !== 'string' || !world.runId
    || typeof world.currentMapId !== 'string' || !world.currentMapId
    || world.map?.id !== world.currentMapId) {
    throw new TypeError('Tactics requires an active, validated remote world map');
  }
  const grid = world.map.grid;
  if (!Array.isArray(grid) || !grid.length || grid.length > 1024
    || typeof grid[0] !== 'string' || grid[0].length < 1 || grid[0].length > 1024
    || grid.length * grid[0].length > 1_000_000
    || grid.some(row => typeof row !== 'string' || row.length !== grid[0].length
      || [...row].length !== row.length)) {
    throw new TypeError('Invalid Tactics remote map grid');
  }
  return world.map;
}

export function tacticsRemoteMapTopology(world) {
  const map = validatedMap(world);
  const grid = map.grid;
  const height = grid.length;
  const width = grid[0].length;
  const walkable = (x, y) => Boolean(grid[y]?.[x] && grid[y][x] !== '#');
  const floors = [];
  const walls = [];
  const wallFaces = [];
  const directions = [
    { dx: 0, dy: -1, side: 'north' },
    { dx: 1, dy: 0, side: 'east' },
    { dx: 0, dy: 1, side: 'south' },
    { dx: -1, dy: 0, side: 'west' },
  ];

  grid.forEach((row, y) => {
    [...row].forEach((tile, x) => {
      if (tile !== '#') {
        floors.push(Object.freeze({ x, y, tile }));
        return;
      }
      // Retain the source renderer's deliberate suppression of the final
      // outer row/column for wall *volumes*, but not for visible wall faces.
      if (x < width - 1 && y < height - 1
        && (walkable(x - 1, y) || walkable(x + 1, y)
          || walkable(x, y - 1) || walkable(x, y + 1))) {
        walls.push(Object.freeze({ x, y }));
      }
      directions.forEach(({ dx, dy, side }) => {
        if (walkable(x + dx, y + dy)) {
          wallFaces.push(Object.freeze({ x, y, side }));
        }
      });
    });
  });

  if (!floors.length) throw new TypeError('Tactics remote map has no walkable cells');
  return Object.freeze({
    mapId: map.id,
    title: typeof map.title === 'string' ? map.title : '',
    width,
    height,
    center: Object.freeze({ x: (width - 1) / 2, y: (height - 1) / 2 }),
    floors: Object.freeze(floors),
    walls: Object.freeze(walls),
    wallFaces: Object.freeze(wallFaces),
  });
}

// Compose the *real* world topology with actors/actions supplied by the host's
// current game state. The renderer-frame validator checks every supplied cell.
// Projectors MUST not fabricate enemies, combat legality or a missing world.
export function tacticsSceneModelFromRemoteWorld(world, state, {
  selectedMemberId,
  focusCell,
  party,
  enemies,
  interaction = null,
} = {}) {
  if (!state || state.mapId !== world?.currentMapId) {
    throw new TypeError('Tactics state and authoritative world map differ');
  }
  const scenePlan = tacticsRemoteMapTopology(world);
  const combatFormation = Boolean(state?.initiative?.order?.length || state?.phase === 'combat');
  if (!Array.isArray(party) || !Array.isArray(enemies)
    || !focusCell || !selectedMemberId) {
    throw new TypeError('Tactics requires live actor and selection projections');
  }
  return Object.freeze({
    version: TACTICS_WORLD_SCENE_VERSION,
    mapId: scenePlan.mapId,
    scenePlan,
    focusCell,
    selectedMemberId,
    partyFormation: combatFormation ? 'combat-grid' : 'explore-compact',
    party,
    enemies,
    interaction,
  });
}
