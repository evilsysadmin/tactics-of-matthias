import assert from 'node:assert/strict';
import test from 'node:test';
import { tacticsSceneFrame } from '../src/adapters/tacticsSceneFrame.js';
import { tacticsSceneModelFromRemoteWorld } from '../src/adapters/tacticsWorldScene.js';
import { mountTacticsThreeViewport } from '../src/render/tacticsThreeViewport.js';

const GRID = [
  '#######', '#..X..#', '#.###.#', '#...#.#',
  '#.#S#.#', '#P.E..#', '#######',
];
function frame() {
  const world = {
    source: 'remote', runStatus: 'active', runId: 'server-run',
    currentMapId: 'crypt-eight-squares',
    map: { id: 'crypt-eight-squares', title: 'Cripta', grid: GRID },
  };
  const state = { mapId: world.currentMapId, phase: 'explore' };
  return tacticsSceneFrame(tacticsSceneModelFromRemoteWorld(world, state, {
    selectedMemberId: 'matthias', focusCell: { x: 1, y: 5 },
    party: [{ id: 'matthias', visible: true, hpRatio: 1, cell: { x: 1, y: 5 } }],
    enemies: [{
      id: 'corrupted-pawn', visible: true, visualType: 'corrupted-pawn',
      cell: { x: 3, y: 5 },
    }],
    interaction: {
      mode: 'hybrid', legalMoves: [{ x: 1, y: 4 }],
      legalTargets: [{ enemyId: 'corrupted-pawn', x: 3, y: 5 }],
    },
  }));
}

function harness() {
  const canvas = {
    style: {}, listeners: new Map(),
    addEventListener(event, fn) { this.listeners.set(event, fn); },
    removeEventListener(event, fn) {
      if (this.listeners.get(event) === fn) this.listeners.delete(event);
    },
    getBoundingClientRect() { return { left: 0, top: 0, width: 800, height: 450 }; },
  };
  const host = {
    clientWidth: 800, clientHeight: 450, children: [],
    appendChild(node) { this.children.push(node); },
    removeChild(node) { this.children.splice(this.children.indexOf(node), 1); },
  };
  const calls = { rendered: [], dimensions: [], pixels: [], disposed: 0, observerStopped: 0 };
  let target = null;
  let pickCoordinates = null;
  const factory = () => ({
    domElement: canvas,
    setPixelRatio(ratio) { calls.pixels.push(ratio); },
    setSize(...dims) { calls.dimensions.push(dims); },
    render(scene, camera) { calls.rendered.push({ scene, camera }); },
    dispose() { calls.disposed += 1; },
  });
  return {
    host, canvas, calls, factory,
    observerFactory: onResize => ({
      observe() { this.onResize = onResize; },
      disconnect() { calls.observerStopped += 1; },
    }),
    raycasterFactory: () => ({
      setFromCamera(position) { pickCoordinates = position; },
      intersectObjects() { return target ? [{ object: target }] : []; },
    }),
    setTarget(next) { target = next; },
    pickCoordinates: () => pickCoordinates,
  };
}

test('mounts a real Three scene into a browser renderer and resizes to container', () => {
  const h = harness();
  const actions = [];
  const viewport = mountTacticsThreeViewport({
    element: h.host, frame: frame(), onAction: action => actions.push(action),
    rendererFactory: h.factory, raycasterFactory: h.raycasterFactory,
    observerFactory: h.observerFactory, pixelRatio: 4,
  });
  assert.equal(h.host.children.length, 1);
  assert.equal(h.calls.rendered.length, 1);
  assert.equal(h.calls.rendered[0].scene, viewport.scene);
  assert.equal(h.calls.dimensions[0][0], 800);
  assert.equal(h.calls.dimensions[0][1], 450);
  assert.equal(viewport.camera.aspect, 800 / 450);
  assert.deepEqual(h.calls.pixels, [2]);
  assert.equal(h.canvas.style.touchAction, 'manipulation');
  viewport.dispose();
  viewport.dispose();
  assert.equal(h.host.children.length, 0);
  assert.equal(h.calls.disposed, 1);
  assert.equal(h.calls.observerStopped, 1);
  assert.equal(h.canvas.listeners.size, 0);
  assert.deepEqual(actions, []);
});

test('pointer hit dispatches only legal actions from current scene, not foreign mesh', () => {
  const h = harness();
  const events = [];
  const viewport = mountTacticsThreeViewport({
    element: h.host, frame: frame(), onAction: action => events.push(action),
    rendererFactory: h.factory, raycasterFactory: h.raycasterFactory,
    observerFactory: h.observerFactory,
  });
  const click = h.canvas.listeners.get('pointerup');
  h.setTarget(viewport.scene.getObjectByName('tactics-move-1-4'));
  click({ button: 0, clientX: 400, clientY: 225 });
  assert.deepEqual(events, [{ kind: 'move', x: 1, y: 4 }]);
  assert.ok(Math.abs(h.pickCoordinates().x) < 1e-9);
  assert.ok(Math.abs(h.pickCoordinates().y) < 1e-9);
  h.setTarget(viewport.scene.getObjectByName('tactics-attack-corrupted-pawn'));
  click({ button: 0, clientX: 100, clientY: 100 });
  assert.deepEqual(events[1], { kind: 'attack', enemyId: 'corrupted-pawn' });
  h.setTarget(viewport.scene.getObjectByName('tactics-walls'));
  click({ button: 0, clientX: 400, clientY: 225 });
  click({ button: 2, clientX: 400, clientY: 225 });
  click({ button: 0, clientX: -10, clientY: -10 });
  assert.equal(events.length, 2);
  viewport.dispose();
  assert.equal(events.length, 2);
});

test('invalid frame update cannot blank live scene; valid update replaces and disposes old', () => {
  const h = harness();
  const viewport = mountTacticsThreeViewport({
    element: h.host, frame: frame(), onAction: () => {},
    rendererFactory: h.factory, raycasterFactory: h.raycasterFactory,
    observerFactory: h.observerFactory,
  });
  const before = viewport.scene;
  assert.throws(() => viewport.updateFrame({ version: 9 }), /Tactics/i);
  assert.equal(viewport.scene, before);
  assert.equal(h.calls.rendered.length, 1);
  viewport.updateFrame(frame());
  assert.notEqual(viewport.scene, before);
  assert.equal(before.children.length, 0);
  assert.equal(h.calls.rendered.length, 2);
  viewport.dispose();
  assert.throws(() => viewport.updateFrame(frame()), /disposed/);
});

test('mount fails closed and cleans scene if renderer is unavailable', () => {
  const h = harness();
  assert.throws(() => mountTacticsThreeViewport({
    element: h.host, frame: frame(), onAction: () => {},
    rendererFactory: () => { throw new Error('WebGL context unavailable'); },
  }), /WebGL context unavailable/);
  assert.equal(h.host.children.length, 0);
  assert.throws(() => mountTacticsThreeViewport({
    element: h.host, frame: frame(), onAction: null,
    rendererFactory: h.factory,
  }), /dispatcher/i);
});
