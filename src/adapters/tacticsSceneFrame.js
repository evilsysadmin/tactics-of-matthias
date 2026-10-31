import { chroniclesIsoTorchPlacements } from '../render/tacticsLighting.js';

// Consume the existing v1 chroniclesProjectSceneModel output, not raw RPG state.
// This boundary will let Tactics own its renderer without importing MM3.
export const TACTICS_SCENE_FRAME_VERSION = 1;

function record(value, name) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    throw new TypeError('Tactics scene requires ' + name);
  }
  return value;
}

function id(value, name) {
  if (typeof value !== 'string' || !value.trim()) {
    throw new TypeError('Invalid Tactics scene ' + name);
  }
  return value;
}

function cell(value, name, width, height) {
  record(value, name);
  if (!Number.isSafeInteger(value.x) || !Number.isSafeInteger(value.y)
    || value.x < 0 || value.y < 0 || value.x >= width || value.y >= height) {
    throw new TypeError('Invalid Tactics scene cell: ' + name);
  }
  return Object.freeze({ x: value.x, y: value.y });
}

function points(values, name, width, height, kind = '') {
  if (!Array.isArray(values)) throw new TypeError('Invalid Tactics scene array: ' + name);
  return Object.freeze(values.map((value, index) => {
    const location = cell(value, name + '[' + index + ']', width, height);
    if (kind === 'floor' && (typeof value.tile !== 'string' || !value.tile || value.tile === '#')) {
      throw new TypeError('Invalid Tactics scene floor tile');
    }
    if (kind === 'face' && !['north', 'east', 'south', 'west'].includes(value.side)) {
      throw new TypeError('Invalid Tactics scene wall face');
    }
    return Object.freeze({
      ...location,
      ...(kind === 'floor' ? { tile: value.tile } : {}),
      ...(kind === 'face' ? { side: value.side } : {}),
    });
  }));
}

function actors(values, name, width, height, party = false) {
  if (!Array.isArray(values)) throw new TypeError('Invalid Tactics scene ' + name);
  const ids = values.map(value => id(value?.id, name + ' id'));
  if (new Set(ids).size !== ids.length) throw new TypeError('Duplicate Tactics scene ' + name + ' id');
  return Object.freeze(values.map(value => {
    if (typeof value.visible !== 'boolean') throw new TypeError('Invalid Tactics scene visibility');
    if (value.visible && !value.cell) throw new TypeError('Invalid Tactics scene visible actor cell');
    if (party && (!Number.isFinite(value.hpRatio) || value.hpRatio < 0 || value.hpRatio > 1)) {
      throw new TypeError('Invalid Tactics scene hpRatio');
    }
    return Object.freeze({
      id: value.id,
      visible: value.visible,
      cell: value.cell ? cell(value.cell, name + ' cell', width, height) : null,
      ...(party ? { hpRatio: value.hpRatio } : {
        visualType: typeof value.visualType === 'string' ? value.visualType : '',
      }),
    });
  }));
}

export function tacticsSceneFrame(scene, { coarsePointer = false } = {}) {
  record(scene, 'model');
  if (scene.version !== 1) throw new TypeError('Unsupported Tactics scene version');
  const mapId = id(scene.mapId, 'mapId');
  const plan = record(scene.scenePlan, 'scenePlan');
  if (plan.mapId !== mapId) throw new TypeError('Tactics scene map mismatch');
  const { width, height } = plan;
  if (!Number.isSafeInteger(width) || width < 1 || width > 2048
    || !Number.isSafeInteger(height) || height < 1 || height > 2048
    || width * height > 1_000_000) {
    throw new TypeError('Invalid Tactics scene dimensions');
  }
  if (!Number.isFinite(plan.center?.x) || !Number.isFinite(plan.center?.y)) {
    throw new TypeError('Invalid Tactics scene center');
  }
  const floors = points(plan.floors, 'floors', width, height, 'floor');
  const walls = points(plan.walls, 'walls', width, height);
  const wallFaces = points(plan.wallFaces, 'wallFaces', width, height, 'face');
  const floorKeys = floors.map(p => p.x + ':' + p.y);
  const wallKeys = walls.map(p => p.x + ':' + p.y);
  const floorSet = new Set(floorKeys);
  if (!floors.length || floorSet.size !== floors.length
    || new Set(wallKeys).size !== walls.length
    || wallKeys.some(key => floorSet.has(key))) {
    throw new TypeError('Invalid Tactics scene terrain overlap or duplicate');
  }
  const party = actors(scene.party, 'party', width, height, true);
  const enemies = actors(scene.enemies, 'enemies', width, height);
  const selectedMemberId = id(scene.selectedMemberId, 'selectedMemberId');
  if (!party.some(member => member.id === selectedMemberId)) {
    throw new TypeError('Tactics scene selection not in party');
  }
  if (!['explore-compact', 'combat-grid'].includes(scene.partyFormation)) {
    throw new TypeError('Unsupported Tactics scene formation');
  }
  let interaction = null;
  if (scene.interaction !== undefined && scene.interaction !== null) {
    const source = record(scene.interaction, 'interaction');
    if (!['move', 'attack', 'hybrid'].includes(source.mode)) {
      throw new TypeError('Invalid Tactics scene interaction mode');
    }
    const legalMoves = points(source.legalMoves || [], 'legalMoves', width, height);
    const targets = source.legalTargets || [];
    const legalTargets = points(targets, 'legalTargets', width, height);
    if (targets.some(target => !enemies.some(enemy => enemy.id === target?.enemyId))) {
      throw new TypeError('Invalid Tactics scene target id');
    }
    interaction = Object.freeze({
      mode: source.mode,
      legalMoves,
      legalTargets: Object.freeze(legalTargets.map((p, index) =>
        Object.freeze({ ...p, enemyId: targets[index].enemyId }))),
    });
  }

  // Explicit allowlist: campaign flags, equipment, character progression and
  // narrative must not cross from gameplay state into a renderer frame.
  return Object.freeze({
    version: TACTICS_SCENE_FRAME_VERSION,
    map: Object.freeze({
      id: mapId,
      title: typeof plan.title === 'string' ? plan.title : '',
      width, height,
      center: Object.freeze({ x: plan.center.x, y: plan.center.y }),
      floors, walls, wallFaces,
    }),
    focusCell: cell(scene.focusCell, 'focusCell', width, height),
    partyFormation: scene.partyFormation,
    selectedMemberId,
    party, enemies, interaction,
    torches: Object.freeze(chroniclesIsoTorchPlacements(plan, { coarsePointer })
      .map(torch => Object.freeze({ ...torch }))),
  });
}
