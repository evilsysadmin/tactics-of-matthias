import * as THREE from 'three';
import { tacticsSpatialRenderPlan } from './tacticsSpatialRenderPlan.js';

// Builds a REAL Three.js scene graph from an already validated v1 gameplay
// frame. The owning browser host supplies WebGLRenderer, assets, SSO and saves.
// No bundled map fixtures or RPG/AI rules are imported.
const MAX_RENDER_CELLS = 12000;

function addInstanced(root, name, entries, geometry, material, height, owned) {
  if (!entries.length) return null;
  const mesh = new THREE.InstancedMesh(geometry, material, entries.length);
  mesh.name = name;
  const dummy = new THREE.Object3D();
  entries.forEach((entry, index) => {
    dummy.position.set(entry.world.x, height, entry.world.z);
    dummy.updateMatrix();
    mesh.setMatrixAt(index, dummy.matrix);
  });
  mesh.instanceMatrix.needsUpdate = true;
  root.add(mesh);
  owned.add(geometry);
  owned.add(material);
  return mesh;
}

function ring(root, name, position, color, action, interactive, owned, radius) {
  const geometry = new THREE.RingGeometry(radius * 0.78, radius, 32);
  const material = new THREE.MeshBasicMaterial({
    color, side: THREE.DoubleSide, transparent: true, opacity: 0.88,
    depthWrite: false,
  });
  const mesh = new THREE.Mesh(geometry, material);
  mesh.rotation.x = -Math.PI / 2;
  mesh.position.set(position.x, 0.12, position.z);
  mesh.name = name;
  if (action) {
    mesh.userData.tacticsAction = Object.freeze({ ...action });
    interactive.add(mesh);
  }
  root.add(mesh);
  owned.add(geometry);
  owned.add(material);
  return mesh;
}

export function createTacticsThreeScene(frame, { createActorVisual = null } = {}) {
  const plan = tacticsSpatialRenderPlan(frame);
  if (plan.floors.length + plan.walls.length > MAX_RENDER_CELLS) {
    throw new RangeError('Tactics scene exceeds the 3D cell budget');
  }
  if (createActorVisual !== null && typeof createActorVisual !== 'function') {
    throw new TypeError('Tactics actor visual factory must be a function');
  }

  const owned = new Set();
  const interactive = new Set();
  const externalVisuals = [];
  const scene = new THREE.Scene();
  scene.background = new THREE.Color(0x100f13);
  scene.fog = new THREE.Fog(0x100f13, 17, 75);
  const root = new THREE.Group();
  root.name = 'tactics-world';
  scene.add(root);

  let disposed = false;
  const dispose = () => {
    if (disposed) return;
    disposed = true;
    scene.remove(root);
    root.clear();
    for (const visual of externalVisuals) visual.removeFromParent();
    for (const resource of owned) resource.dispose();
    owned.clear();
    interactive.clear();
  };

  try {
    root.add(new THREE.AmbientLight(0xb6c4de, 1.2));
    const moon = new THREE.DirectionalLight(0xb7d4ff, 2.1);
    moon.position.set(-8, 16, 7);
    root.add(moon);

    const size = plan.cellSize;
    const floorGeometry = new THREE.BoxGeometry(size * 0.994, 0.17, size * 0.994);
    const floorMaterial = new THREE.MeshStandardMaterial({
      color: 0x44555e, roughness: 0.92, metalness: 0.05,
    });
    addInstanced(root, 'tactics-floors', plan.floors,
      floorGeometry, floorMaterial, -0.095, owned);

    const wallGeometry = new THREE.BoxGeometry(size, 2.65, size);
    const wallMaterial = new THREE.MeshStandardMaterial({
      color: 0x33373c, roughness: 0.96, metalness: 0.03,
    });
    addInstanced(root, 'tactics-walls', plan.walls,
      wallGeometry, wallMaterial, 1.23, owned);

    plan.torches.forEach((torch, index) => {
      const glow = new THREE.PointLight(0xffae61, 1.55, 7.5, 2);
      glow.name = 'tactics-torch-' + index;
      glow.position.set(torch.world.x, 1.8, torch.world.z);
      root.add(glow);
    });

    plan.markers.moves.forEach((move) => {
      ring(root, 'tactics-move-' + move.x + '-' + move.y,
        move.world, 0x8bd4bc,
        { kind: 'move', x: move.x, y: move.y }, interactive, owned, size * 0.33);
    });
    plan.markers.targets.forEach((target) => {
      ring(root, 'tactics-attack-' + target.enemyId,
        target.world, 0xf69a8f,
        { kind: 'attack', enemyId: target.enemyId }, interactive, owned, size * 0.38);
    });

    for (const [kind, actors] of [['party', plan.party], ['enemy', plan.enemies]]) {
      for (const actor of actors) {
        if (!actor.visible || !actor.world) continue;
        // Diegetic identity rings are locators, not fabricated character
        // sprites. Premium real visuals are supplied by a future asset factory.
        ring(root, 'tactics-' + kind + '-' + actor.id,
          actor.world, kind === 'party' ? 0xe1c58a : 0xc07575,
          kind === 'party' ? { kind: 'member', memberId: actor.id } : null,
          interactive, owned, size * 0.22);
        if (!createActorVisual) continue;
        const visual = createActorVisual({
          kind, id: actor.id, world: actor.world, mapId: plan.mapId,
        });
        if (!(visual instanceof THREE.Object3D)) {
          throw new TypeError('Tactics actor visual factory must return Three.Object3D');
        }
        visual.position.set(actor.world.x, 0, actor.world.z);
        visual.name ||= 'tactics-visual-' + kind + '-' + actor.id;
        root.add(visual);
        externalVisuals.push(visual);
        // Asset ownership stays with the caller: never dispose shared R2
        // textures/materials owned by an injected model/sprite factory.
      }
    }

    const camera = new THREE.PerspectiveCamera(38, 1.6, 0.1, 160);
    camera.position.set(plan.focus.x, 8.15, plan.focus.z + 8.55);
    camera.lookAt(plan.focus.x, 0.78, plan.focus.z - 2.65);
    camera.name = 'tactics-camera';

    return Object.freeze({
      scene, camera, root, plan,
      pick(intersections) {
        if (disposed || !Array.isArray(intersections)) return null;
        for (const hit of intersections) {
          let obj = hit?.object;
          while (obj && obj !== root) {
            if (interactive.has(obj)) return obj.userData.tacticsAction || null;
            obj = obj.parent;
          }
        }
        return null;
      },
      dispose,
    });
  } catch (error) {
    dispose();
    throw error;
  }
}
