# AGENTS.md — Tactics of Matthias

## Ownership
- This repository owns the eventual standalone **isometric Tactics RPG**, not Chronicles/MM3.
- Source of the original Tactics implementation: https://github.com/evilsysadmin/chronicles-of-matthias
- Never pull live JS imports directly from Chronicles. Migrate modules deliberately with tests and documented provenance; prevent source drift by validating behavior at each slice.
- This is **not yet a playable frontend**. Do not claim gameplay or staging exists just because profile tests pass.
- Identity, authored lore and some R2 assets may be shared via explicit contracts; **character progression, tactical rules and saved campaigns must become independent**.

## Workflow
- Use one focused PR per improvement. Verify correct main SHA, relevant open PRs, tests, and actual merge status.
- Add regression tests alongside every moved runtime contract. First test suite: `npm test` (Node 22, no native modules or external dependencies).
- Keep Chronicles usable during extraction; do not delete or rewrite legacy saves, checkpoint schemas or assets.
- No production/manual deployments, infrastructure changes or copying secrets. Deployment ownership is in `evilsysadmin/shadowops-infra`.
- Preserve SSH/Cloudflare/SSO scopes and test authentication and cross-domain handoff before exposing standalone Tactics to players.
- New assets should reuse authorized versioned R2 paths where possible; do not check in large or copyrighted third-party binaries.
- Distinguish PR checks, merged source, staging availability and actual gameplay acceptance; do not treat CI alone as visual validation.
- Future independent Home entry is launched from Chronicles' existing expedition book.

## Scene migration boundary
- Keep the v1 renderer-scene adapter data-only. Real maps and legal actions must be produced by a validated game runtime; do not treat a scene fixture as a game world.
- Do not put saves, identity, progression, story or API credentials into a renderer frame. Version scene inputs explicitly and fail closed on unknown versions.

## Headless session contract
- `src/tacticsSession.js` is a coordinator, not an authority or game implementation. Never supply local fallback worlds in production; hydrate only server-validated run objects.
- Preserve action revision checking, state/scene map matching, and host ownership of saves, progression, backend authorization and real-world ports.

## Authoritative scene topology
- The standalone `tacticsRemoteMapTopology` may only use a current remotely validated map manifest; do not bundle usable fallback maps or infer game rules from tile labels.
- Test fixtures may reproduce real authored grids to assert parity, but never inject them into production hydration. The host must supply actors and actions from its current, checked world state.

## Real Three scene ownership
- `src/render/tacticsThreeScene.js` owns only the resources it creates. Actor sprite/model factory owns its own shared geometry/material/texture lifecycle; do not dispose borrowed assets.
- Keep picking restricted to known legal markers and let the authoritative rule engine revalidate every intent. Do not invent a fallback playable map, browser auth token, or deploy to prod.
- Node tests of Three scene graphs are not a WebGL/Chromium visual smoke; build a browser host and separately verify its rendering before claiming playable Tactics.

## Browser viewport boundary
- Never mount `mountTacticsThreeViewport` before a valid current scene frame from an authenticated, backend-validated session. Browser host handles auth, persistent actions, asset lifecycle and error fallback.
- WebGL context failures must surface to the UI, not trigger local world creation. Confirm real browser/Chromium rendering before marking standalone playable.

## Browser-host authority gate
- Do not register `createTacticsGameHost` with fake loaders or local saves in runtime: a real authenticated, backend-authorized `loadAuthorizedWorld` and `commitTransition` are mandatory.
- `source: remote` alone is not authentication. Do not advance displayed frames until the authoritative write is acknowledged; stale loads/actions and denied turns must fail closed.
