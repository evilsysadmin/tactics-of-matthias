import { createTacticsGameHost } from './tacticsGameHost.js';

// The UI is intentionally provider-neutral. No hardcoded login, test world,
// injected public credentials, or magical "play offline" fallback.
export function mountTacticsAppShell({
  root, loadAuthorizedWorld, ports, commitTransition,
  hostFactory = createTacticsGameHost,
} = {}) {
  const doc = root?.ownerDocument;
  if (!doc || typeof doc.createElement !== 'function'
    || typeof root.replaceChildren !== 'function') {
    throw new TypeError('Tactics app needs a browser root element');
  }

  const node = (tag, className, content = '') => {
    const element = doc.createElement(tag);
    element.className = className;
    element.textContent = content;
    return element;
  };

  const layout = node('main', 'tactics-app');
  const header = node('header', 'tactics-app__header');
  const brand = node('span', 'tactics-app__eyebrow', 'SHADOWOPS · TACTICS');
  const heading = node('h1', 'tactics-app__title', 'Tactics of Matthias');
  const subtitle = node('p', 'tactics-app__subtitle',
    'Combate táctico isométrico. Mundo y partidas verificados por el servidor.');
  header.append(brand, heading, subtitle);

  const stage = node('section', 'tactics-app__stage');
  stage.setAttribute('aria-label', 'Tablero táctico');
  const statusPanel = node('div', 'tactics-app__status');
  statusPanel.setAttribute('role', 'status');
  statusPanel.setAttribute('aria-live', 'polite');
  const statusTitle = node('h2', 'tactics-app__status-title');
  const statusDescription = node('p', 'tactics-app__status-description');
  const retry = node('button', 'tactics-app__retry', 'Reintentar conexión');
  retry.type = 'button';
  retry.hidden = true;
  statusPanel.append(statusTitle, statusDescription, retry);
  layout.append(header, stage, statusPanel);
  root.replaceChildren(layout);

  let disposed = false;
  let host = null;
  let state = 'idle';

  const status = ({ kind, error = null }) => {
    if (disposed) return;
    state = kind;
    layout.dataset.state = kind;
    retry.hidden = kind !== 'error';
    retry.disabled = kind !== 'error';
    if (kind === 'idle') {
      statusTitle.textContent = 'Sala táctica lista para conectar';
      statusDescription.textContent = 'La partida se abrirá tras validar tu sesión.';
    } else if (kind === 'loading') {
      statusTitle.textContent = 'Preparando expedición…';
      statusDescription.textContent = 'Verificando identidad, mundo y partida.';
    } else if (kind === 'ready') {
      statusTitle.textContent = 'Tablero preparado';
      statusDescription.textContent = 'Las jugadas se validan y confirman en el servidor.';
    } else if (kind === 'unavailable') {
      statusTitle.textContent = 'Servidor de Tactics pendiente';
      statusDescription.textContent =
        'El juego independiente aún no tiene integración autorizada. El modo actual sigue en Chronicles.';
    } else {
      statusTitle.textContent = 'No se pudo abrir la expedición';
      // Never echo a server stack, token, URL or untrusted exception text.
      statusDescription.textContent = [401, 403].includes(error?.status)
        ? 'Tu sesión no está autorizada. Accede de nuevo a ShadowOps.'
        : 'No hay ningún mundo de sustitución. Comprueba la conexión y vuelve a intentarlo.';
    }
  };

  const available = typeof loadAuthorizedWorld === 'function'
    && typeof commitTransition === 'function'
    && typeof ports?.hydrateState === 'function'
    && typeof ports?.projectScene === 'function'
    && Boolean(ports?.actionPorts);

  if (available) {
    try {
      host = hostFactory({
        element: stage, loadAuthorizedWorld, ports, commitTransition,
        onStatus: status,
      });
    } catch {
      host = null;
    }
  }

  const start = async () => {
    if (disposed) throw new Error('Tactics app is disposed');
    if (!host) {
      status({ kind: 'unavailable' });
      return false;
    }
    return host.start();
  };
  const retryClick = () => { void start(); };
  retry.addEventListener('click', retryClick);
  status({ kind: host ? 'idle' : 'unavailable' });

  const dispose = () => {
    if (disposed) return;
    disposed = true;
    host?.dispose();
    retry.removeEventListener('click', retryClick);
    if (root.firstChild === layout) root.replaceChildren();
    state = 'disposed';
  };
  return Object.freeze({
    start, dispose,
    get state() { return state; },
    get element() { return layout; },
  });
}
