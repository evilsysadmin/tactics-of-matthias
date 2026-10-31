import * as THREE from 'three';
import { createTacticsThreeScene } from './tacticsThreeScene.js';

// WebGL mounting boundary. Requires a VALIDATED scene frame from a genuine
// game session; it never creates maps, authenticates users or writes saves.
export function mountTacticsThreeViewport({
  element,
  frame,
  onAction,
  createActorVisual = null,
  rendererFactory = () => new THREE.WebGLRenderer({ antialias: true }),
  raycasterFactory = () => new THREE.Raycaster(),
  observerFactory = null,
  pixelRatio = 1,
} = {}) {
  if (!element || typeof element.appendChild !== 'function'
    || typeof element.removeChild !== 'function') {
    throw new TypeError('Tactics viewport needs a DOM mounting element');
  }
  if (typeof onAction !== 'function') {
    throw new TypeError('Tactics viewport requires an action dispatcher');
  }

  let view = createTacticsThreeScene(frame, { createActorVisual });
  let renderer;
  let raycaster;
  let observer = null;
  let stopFallbackResize = null;
  let mounted = false;
  let disposed = false;
  let pointerListener = null;

  const render = () => {
    if (disposed) return;
    const width = Math.max(1, Math.round(Number(element.clientWidth) || 1));
    const height = Math.max(1, Math.round(Number(element.clientHeight) || 1));
    view.camera.aspect = width / height;
    view.camera.updateProjectionMatrix();
    renderer.setSize(width, height, false);
    renderer.render(view.scene, view.camera);
  };

  const dispose = () => {
    if (disposed) return;
    disposed = true;
    observer?.disconnect();
    stopFallbackResize?.();
    if (pointerListener) renderer?.domElement?.removeEventListener('pointerup', pointerListener);
    if (mounted) {
      element.removeChild(renderer.domElement);
      mounted = false;
    }
    view?.dispose();
    renderer?.dispose();
  };

  try {
    renderer = rendererFactory();
    if (!renderer?.domElement || typeof renderer.setSize !== 'function'
      || typeof renderer.render !== 'function'
      || typeof renderer.dispose !== 'function') {
      throw new TypeError('Invalid Tactics WebGL renderer');
    }
    raycaster = raycasterFactory();
    if (typeof raycaster?.setFromCamera !== 'function'
      || typeof raycaster?.intersectObjects !== 'function') {
      throw new TypeError('Invalid Tactics raycaster');
    }
    renderer.setPixelRatio(Math.min(2, Math.max(1, Number(pixelRatio) || 1)));
    renderer.domElement.style.touchAction = 'manipulation';
    renderer.domElement.style.width = '100%';
    renderer.domElement.style.height = '100%';
    element.appendChild(renderer.domElement);
    mounted = true;

    pointerListener = (event) => {
      if (disposed || event?.button !== 0) return;
      const bounds = renderer.domElement.getBoundingClientRect();
      if (!bounds.width || !bounds.height) return;
      const localX = (event.clientX - bounds.left) / bounds.width;
      const localY = (event.clientY - bounds.top) / bounds.height;
      if (localX < 0 || localX > 1 || localY < 0 || localY > 1) return;
      raycaster.setFromCamera(new THREE.Vector2(localX * 2 - 1, 1 - localY * 2), view.camera);
      const action = view.pick(raycaster.intersectObjects(view.root.children, true));
      if (action) onAction(action);
    };
    renderer.domElement.addEventListener('pointerup', pointerListener);

    if (observerFactory) {
      observer = observerFactory(render);
      observer.observe(element);
    } else if (typeof ResizeObserver !== 'undefined') {
      observer = new ResizeObserver(render);
      observer.observe(element);
    } else if (typeof window !== 'undefined') {
      window.addEventListener('resize', render);
      stopFallbackResize = () => window.removeEventListener('resize', render);
    }
    render();

    return Object.freeze({
      get scene() { return view.scene; },
      get camera() { return view.camera; },
      get frame() { return view.plan; },
      render,
      updateFrame(nextFrame) {
        if (disposed) throw new Error('Tactics viewport is disposed');
        // Allocate and validate the new scene first. A bad runtime frame does
        // not destroy the last good visible scene or its event registration.
        const next = createTacticsThreeScene(nextFrame, { createActorVisual });
        const old = view;
        view = next;
        try {
          render();
        } catch (error) {
          view = old;
          next.dispose();
          throw error;
        }
        old.dispose();
      },
      dispose,
    });
  } catch (error) {
    dispose();
    throw error;
  }
}
