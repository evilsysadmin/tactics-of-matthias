import './tacticsAppShell.css';
import { mountTacticsAppShell } from './tacticsAppShell.js';

// Standalone frontend is now buildable. Until its own authenticated Tactics
// backend/SSO exists, show a safe unavailable state: NEVER impersonate
// Chronicles, create placeholder characters or wire a local save adapter.
const root = document.getElementById('app');
if (!root) throw new Error('Tactics application root missing');
const app = mountTacticsAppShell({ root });
window.addEventListener('pagehide', () => app.dispose(), { once: true });
