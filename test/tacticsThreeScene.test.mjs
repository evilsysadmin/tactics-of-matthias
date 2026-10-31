import assert from 'node:assert/strict';
import test from 'node:test';
import * as THREE from 'three';
import { tacticsSceneFrame } from '../src/adapters/tacticsSceneFrame.js';
import { tacticsSceneModelFromRemoteWorld } from '../src/adapters/tacticsWorldScene.js';
import { createTacticsThreeScene } from '../src/render/tacticsThreeScene.js';

const cryptGrid = [
  '#######', '#..X..#', '#.###.#', '#...#.#',
  '#.#S#.#', '#P.E..#', '#######',
];
function frame() {
  const world = {
    source: 'remote', runStatus: 'active', runId: 'server-validated-id',
    currentMapId: 'crypt-eight-squares',
    map: { id: 'crypt-eight-squares', title: 'Cripta', grid: cryptGrid },
  };
  const state = {
    mapId: world.currentMapId, phase: 'combat',
    initiative: { order: [{ kind: 'party', id: 'matthias' }] },
  };
  return tacticsSceneFrame(tacticsSceneModelFromRemoteWorld(world, state, {
    selectedMemberId: 'matthias', focusCell: { x: 3, y: 3 },
    party: [
      { id: 'matthias', visible: true, hpRatio: 1, cell: { x: 1, y: 5 } },
      { id: 'rook', visible: false, hpRatio: 0, cell: null },
    ],
    enemies: [{
      id: 'corrupted-pawn', visible: true,
      visualType: 'corrupted-pawn', cell: { x: 3, y: 5 },
    }],
    interaction: {
      mode: 'hybrid', legalMoves: [{ x: 1, y: 4 }],
      legalTargets: [{ enemyId: 'corrupted-pawn', x: 3, y: 5 }],
    },
  }));
}

test('constructs native Three scene graph, real instanced crypt geometry and camera', () => {
  const rendered = createTacticsThreeScene(frame());
  assert.ok(rendered.scene instanceof THREE.Scene);
  assert.ok(rendered.camera instanceof THREE.PerspectiveCamera);
  assert.equal(rendered.scene.getObjectByName('tactics-floors').count, 19);
  assert.equal(rendered.scene.getObjectByName('tactics-walls').count, 16);
  assert.equal(rendered.camera.fov, 38);
  assert.deepEqual(rendered.camera.position.toArray(), [0, 8.15, 8.55]);
  const floor = rendered.scene.getObjectByName('tactics-floors');
  const matrix = new THREE.Matrix4();
  floor.getMatrixAt(0, matrix);
  const position = new THREE.Vector3().setFromMatrixPosition(matrix);
  assert.ok(Math.abs(position.y + 0.095) < 1e-6);
  assert.ok(rendered.scene.getObjectByName('tactics-party-matthias'));
  assert.equal(rendered.scene.getObjectByName('tactics-party-rook'), undefined);
  assert.ok(rendered.scene.getObjectByName('tactics-enemy-corrupted-pawn'));
  rendered.dispose();
  rendered.dispose();
  assert.equal(rendered.scene.children.length, 0);
});

test('picking is limited to legal markers, actor selection and live scene-owned meshes', () => {
  const rendered = createTacticsThreeScene(frame());
  const move = rendered.root.getObjectByName('tactics-move-1-4');
  const attack = rendered.root.getObjectByName('tactics-attack-corrupted-pawn');
  const member = rendered.root.getObjectByName('tactics-party-matthias');
  assert.deepEqual(rendered.pick([{ object: move }]), { kind: 'move', x: 1, y: 4 });
  assert.deepEqual(rendered.pick([{ object: attack }]), {
    kind: 'attack', enemyId: 'corrupted-pawn',
  });
  assert.deepEqual(rendered.pick([{ object: member }]), {
    kind: 'member', memberId: 'matthias',
  });
  assert.equal(rendered.pick([{ object: rendered.root.getObjectByName('tactics-floors') }]), null);
  const foreign = new THREE.Mesh();
  foreign.userData.tacticsAction = { kind: 'attack', enemyId: 'ghost' };
  assert.equal(rendered.pick([{ object: foreign }]), null);
  rendered.dispose();
  assert.equal(rendered.pick([{ object: move }]), null);
});

test('actor visuals are opt-in factory objects and remain caller-owned', () => {
  const supplied = [];
  const rendered = createTacticsThreeScene(frame(), {
    createActorVisual(meta) {
      const visual = new THREE.Group();
      supplied.push({ visual, meta });
      return visual;
    },
  });
  assert.deepEqual(supplied.map(x => x.meta.id), ['matthias', 'corrupted-pawn']);
  assert.deepEqual(supplied[0].visual.position.toArray(), [-4.9, 0, 4.9]);
  assert.equal(supplied[0].visual.parent, rendered.root);
  rendered.dispose();
  assert.equal(supplied[0].visual.parent, null);
});

test('invalid frames or factories fail without creating fake terrain or assets', () => {
  assert.throws(() => createTacticsThreeScene({ version: 999 }), /Tactics/);
  assert.throws(() => createTacticsThreeScene(frame(), {
    createActorVisual: () => ({ nonsense: true }),
  }), /Three.Object3D/);
  assert.throws(() => createTacticsThreeScene(frame(), { createActorVisual: 'wrong' }), /factory/);
});
