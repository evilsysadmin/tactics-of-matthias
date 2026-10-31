// Pure 3D positions for the standalone Tactics renderer. Coordinates and
// proportions match Chronicles' published isometric dungeon plan; no Three.js,
// DOM, fake world, material choices or game rules live here.
export const TACTICS_ISOMETRIC_CELL_SIZE = 2.45;

function checkedFrame(frame) {
  if (frame?.version !== 1 || !frame?.map
    || !Number.isSafeInteger(frame.map.width) || frame.map.width < 1
    || !Number.isSafeInteger(frame.map.height) || frame.map.height < 1
    || !Number.isFinite(frame.map.center?.x) || !Number.isFinite(frame.map.center?.y)
    || !Array.isArray(frame.map.floors) || !Array.isArray(frame.map.walls)
    || !Array.isArray(frame.party) || !Array.isArray(frame.enemies)) {
    throw new TypeError('Invalid Tactics renderer frame for spatial projection');
  }
  return frame.map;
}

function normalizedSize(value) {
  const number = Number(value);
  return Number.isFinite(number) && number > 0 ? number : TACTICS_ISOMETRIC_CELL_SIZE;
}

export function tacticsCellToWorld(frame, x, y, cellSize = TACTICS_ISOMETRIC_CELL_SIZE) {
  const map = checkedFrame(frame);
  if (!Number.isSafeInteger(x) || !Number.isSafeInteger(y)
    || x < 0 || x >= map.width || y < 0 || y >= map.height) {
    throw new TypeError('Invalid Tactics cell for spatial projection');
  }
  const size = normalizedSize(cellSize);
  return Object.freeze({
    x: (x - map.center.x) * size,
    y: 0,
    z: (y - map.center.y) * size,
  });
}

function placed(frame, cells, cellSize) {
  return Object.freeze(cells.map(entry => Object.freeze({
    ...entry,
    world: tacticsCellToWorld(frame, entry.x, entry.y, cellSize),
  })));
}

function placedActor(frame, actor, cellSize) {
  return Object.freeze({
    id: actor.id,
    visible: actor.visible,
    world: actor.visible && actor.cell
      ? tacticsCellToWorld(frame, actor.cell.x, actor.cell.y, cellSize)
      : null,
  });
}

export function tacticsSpatialRenderPlan(frame, { cellSize = TACTICS_ISOMETRIC_CELL_SIZE } = {}) {
  const map = checkedFrame(frame);
  const size = normalizedSize(cellSize);
  const interaction = frame.interaction;
  return Object.freeze({
    version: 1,
    mapId: map.id,
    cellSize: size,
    foundation: Object.freeze({
      width: (map.width + 0.25) * size,
      depth: (map.height + 0.25) * size,
    }),
    floors: placed(frame, map.floors, size),
    walls: placed(frame, map.walls, size),
    wallFaces: placed(frame, map.wallFaces || [], size),
    party: Object.freeze(frame.party.map(actor => placedActor(frame, actor, size))),
    enemies: Object.freeze(frame.enemies.map(actor => placedActor(frame, actor, size))),
    focus: tacticsCellToWorld(frame, frame.focusCell.x, frame.focusCell.y, size),
    markers: Object.freeze({
      moves: placed(frame, interaction?.legalMoves || [], size),
      targets: placed(frame, interaction?.legalTargets || [], size),
    }),
    // Torch source offsets are renderer-authored world-unit values, not
    // measured in grid cells; preserve their precise authored displacement.
    torches: Object.freeze((frame.torches || []).map(torch => Object.freeze({
      x: torch.x,
      y: torch.y,
      world: Object.freeze({
        ...tacticsCellToWorld(frame, torch.x, torch.y, size),
        x: tacticsCellToWorld(frame, torch.x, torch.y, size).x + torch.ox,
        z: tacticsCellToWorld(frame, torch.x, torch.y, size).z + torch.oz,
      }),
    }))),
  });
}
