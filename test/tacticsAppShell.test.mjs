import assert from 'node:assert/strict';
import test from 'node:test';
import { mountTacticsAppShell } from '../src/browser/tacticsAppShell.js';

// Tiny DOM implementation for shell event lifecycle; not a fake game runtime.
function createDocument() {
  const doc = { createElement };
  function createElement(tag) {
    const listeners = new Map();
    return {
      tag, ownerDocument: doc, children: [], dataset: {}, attributes: {},
      hidden: false, disabled: false, textContent: '', className: '',
      append(...items) { this.children.push(...items); },
      replaceChildren(...items) { this.children = [...items]; },
      get firstChild() { return this.children[0] || null; },
      setAttribute(key, value) { this.attributes[key] = value; },
      addEventListener(key, fn) { listeners.set(key, fn); },
      removeEventListener(key, fn) {
        if (listeners.get(key) === fn) listeners.delete(key);
      },
      click() { listeners.get('click')?.(); },
    };
  }
  return { doc, root: doc.createElement('div') };
}
function find(element, className) {
  if (element.className === className) return element;
  return element.children.flatMap(child => {
    const match = find(child, className);
    return match ? [match] : [];
  })[0] || null;
}

test('without authorized providers, shell shows honest unavailability and never starts', async () => {
  const { root } = createDocument();
  const app = mountTacticsAppShell({ root });
  assert.equal(app.state, 'unavailable');
  assert.equal(await app.start(), false);
  assert.match(find(root, 'tactics-app__status-title').textContent, /pendiente/i);
  assert.equal(find(root, 'tactics-app__retry').hidden, true);
  assert.equal(find(root, 'tactics-app__stage').children.length, 0);
  app.dispose();
  assert.equal(root.children.length, 0);
});

test('authorized loader mounts only through host; loading, ready and errors are reflected safely', async () => {
  const { root } = createDocument();
  let callbacks = null;
  let starts = 0;
  let disposals = 0;
  const app = mountTacticsAppShell({
    root, loadAuthorizedWorld: async () => ({}), commitTransition: async () => true,
    ports: { hydrateState: () => ({}), projectScene: () => ({}), actionPorts: {} },
    hostFactory({ element, onStatus }) {
      callbacks = { element, onStatus };
      return {
        async start() {
          starts++;
          onStatus({ kind: 'loading' });
          onStatus({ kind: 'ready' });
          return true;
        },
        dispose() { disposals++; },
      };
    },
  });
  assert.equal(callbacks.element, find(root, 'tactics-app__stage'));
  assert.equal(await app.start(), true);
  assert.equal(starts, 1);
  assert.equal(app.state, 'ready');
  assert.equal(find(root, 'tactics-app__retry').hidden, true);
  callbacks.onStatus({ kind: 'error', error: { message: 'SECRET_TOKEN=abc', status: 403 } });
  assert.equal(app.state, 'error');
  assert.equal(find(root, 'tactics-app__retry').hidden, false);
  assert.match(find(root, 'tactics-app__status-description').textContent, /ShadowOps/);
  assert.doesNotMatch(find(root, 'tactics-app__status-description').textContent, /SECRET_TOKEN/);
  find(root, 'tactics-app__retry').click();
  await Promise.resolve();
  assert.equal(starts, 2);
  app.dispose();
  app.dispose();
  assert.equal(disposals, 1);
  assert.equal(root.children.length, 0);
});

test('disposing an outdated shell does not erase a newer mount or accept late callbacks', async () => {
  const { root } = createDocument();
  let update;
  const app = mountTacticsAppShell({
    root, loadAuthorizedWorld: async () => ({}), commitTransition: async () => true,
    ports: { hydrateState: () => ({}), projectScene: () => ({}), actionPorts: {} },
    hostFactory({ onStatus }) {
      update = onStatus;
      return { start: async () => true, dispose() {} };
    },
  });
  const newLayout = root.ownerDocument.createElement('article');
  root.replaceChildren(newLayout);
  app.dispose();
  update({ kind: 'ready' });
  assert.equal(root.firstChild, newLayout);
  assert.equal(app.state, 'disposed');
  await assert.rejects(app.start(), /disposed/);
});

test('server-confirmed terminal result is shown without exposing internal errors or retry', async () => {
  const { root } = createDocument();
  let report;
  const app = mountTacticsAppShell({
    root, loadAuthorizedWorld: async () => ({}),
    commitTransition: async () => ({}),
    ports: { hydrateState: () => ({}), projectScene: () => ({}), actionPorts: {} },
    hostFactory: ({ onStatus }) => {
      report = onStatus;
      return { start: async () => true, dispose() {} };
    },
  });
  report({ kind: 'finished', runStatus: 'defeated' });
  assert.equal(app.state, 'finished');
  assert.match(find(root, 'tactics-app__status-title').textContent, /caído/);
  assert.match(find(root, 'tactics-app__status-description').textContent, /servidor/);
  assert.equal(find(root, 'tactics-app__retry').hidden, true);
  app.dispose();
});
