import { tacticsCreateSession, tacticsApplySessionRequest } from '../tacticsSession.js';
import { mountTacticsThreeViewport } from '../render/tacticsThreeViewport.js';

// Lifecycle ONLY: authentication, world/turn authority and remote persistence
// must all be provided by the real host integration, never by local fallbacks.
export function createTacticsGameHost({
  element, loadAuthorizedWorld, ports, commitTransition,
  viewportFactory = mountTacticsThreeViewport, onStatus = () => {},
} = {}) {
  if (!element || typeof loadAuthorizedWorld !== 'function'
    || typeof commitTransition !== 'function'
    || typeof viewportFactory !== 'function' || typeof onStatus !== 'function') {
    throw new TypeError('Tactics host needs mount, authorized loader, commit and viewport ports');
  }
  if (typeof ports?.hydrateState !== 'function'
    || typeof ports?.projectScene !== 'function' || !ports?.actionPorts) {
    throw new TypeError('Tactics host requires real world and action ports');
  }

  let revision = 0;
  let pending = null;
  let session = null;
  let viewport = null;
  let status = 'idle';
  let busy = false;
  let disposed = false;
  const notify = (kind, error = null) => {
    status = kind;
    onStatus(Object.freeze({ kind, error }));
  };
  const valid = (generation, signal) => !disposed
    && revision === generation && !signal.aborted;
  const clear = () => {
    pending?.abort();
    pending = null;
    viewport?.dispose();
    viewport = null;
    session = null;
    busy = false;
  };

  const dispatchAction = async action => {
    if (disposed || status !== 'ready' || busy || !session || !viewport
      || !['move', 'attack'].includes(action?.kind)) return false;
    const before = session;
    busy = true;
    const generation = revision;
    const signal = pending.signal;
    try {
      // Projection can fail if a world changed under us. Never leak an
      // unhandled promise rejection from a WebGL pointer event.
      const result = tacticsApplySessionRequest(before, {
        ...action, revision: before.revision,
      }, ports);
      if (!result) return false;
      const ack = await commitTransition({
        previous: before,
        next: result.session,
        actionKind: result.actionKind,
        actorMemberId: result.actorMemberId,
        signal,
      });
      if (!valid(generation, signal)) return false;
      if (ack !== true) throw new Error('Tactics server did not acknowledge transition');
      // Only a confirmed save may be revealed to the renderer.
      viewport.updateFrame(result.session.frame);
      session = result.session;
      return true;
    } catch (error) {
      if (valid(generation, signal)) {
        // The remote write might already have succeeded before a rendering
        // failure, so stop accepting commands until the next bootstrap.
        notify('error', error);
      }
      return false;
    } finally {
      if (revision === generation) busy = false;
    }
  };

  const start = async () => {
    if (disposed) throw new Error('Tactics host is disposed');
    const generation = ++revision;
    clear();
    const controller = new AbortController();
    pending = controller;
    notify('loading');
    try {
      const world = await loadAuthorizedWorld({ signal: controller.signal });
      if (!valid(generation, controller.signal)) return false;
      const next = tacticsCreateSession(world, ports);
      if (!valid(generation, controller.signal)) return false;
      const mounted = viewportFactory({
        element, frame: next.frame,
        onAction: action => { void dispatchAction(action); },
      });
      if (!valid(generation, controller.signal)) {
        mounted.dispose();
        return false;
      }
      viewport = mounted;
      session = next;
      notify('ready');
      return true;
    } catch (error) {
      if (!valid(generation, controller.signal)) return false;
      clear();
      notify('error', error);
      return false;
    }
  };

  const dispose = () => {
    if (disposed) return;
    revision++;
    disposed = true;
    clear();
    status = 'disposed';
  };
  return Object.freeze({
    start, dispatchAction, dispose,
    get status() { return status; },
    get session() { return session; },
  });
}
