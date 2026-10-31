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

## Standalone frontend shell
- Vite 6.4.3 and Three 0.186.1 are pinned; `npm test && npm run build` must pass on PR and main. The page is **not playable** until a real SSO/API Tactics integration supplies the required authenticated host ports.
- Never wire demo worlds, a fake identity, unacknowledged checkpoint writes, or the Chronicles save endpoint into `src/browser/main.js`. Treat an unavailable service as a visible, honest status, not an opportunity to bypass auth.
- The source workflow builds only; deployment/production ownership remains with `evilsysadmin/shadowops-infra`.

## Independent API authority
- Go HTTP layer resides in `api/internal/tacticsapi`. A real ShadowOps verifier must check the dedicated `tactics` audience, issuer, signature, expiry and session freshness; do not reuse Chronicles bearer tokens or trust unverified username headers.
- Server-side engine revalidates every move/attack against authoritative run state. `Store` must atomically compare-and-swap by **owner + run ID + world version**, in a separate Tactics collection. Never expose an unconfigured backend or demo store in production.
- API tests use in-memory doubles only to verify route authority; passing them does not establish a deployable SSO or Mongo integration. Do not change Chronicles save schemas.

## Server-authoritative display
- Browser `commitTransition` must return the full, validated server run with the next exact `worldVersion` and matching `runId`. A boolean 2xx/ACK does NOT validate game outcomes.
- Never promote client-predicted damage, XP or enemy positions after a server write; rehydrate the confirmed world and rebuild the scene from it. Error/rebootstrap on drift.

## Tactics API client configuration
- `createTacticsRunApi` must receive an exactly pinned approved API origin, an existing owner-authorized run ID and a REAL access token exchange for audience `tactics`. The independent Identity issuer does not currently advertise this audience; do not invent it, reuse a Chronicles token or hardcode an unverified endpoint.
- Bootstrap and action responses must be validated; no token storage, redirect following, exposed server error bodies, synthetic fallback world or client checkpoint state. Continue server-confirmed reconciliation after writes.

## Mongo storage
- `api/internal/tacticsmongo` must always bind owner + run ID + expected worldVersion + active status for atomic CAS. No upserts, terminal resurrection or writes to any Chronicles/Chess/Identity database.
- Mongo CI integration must run only on disposable localhost `shadowops_tactics_ci`; require explicit per-environment connection and dedicated credentials in production. This store does not confer game or identity authority.

## Go movement parity (incomplete authoritative game engine)
- `api/internal/tacticsrules` owns pure movement legality decisions based on the *server's* state and current map. Do not trust client-supplied party/enemy positions or use a local test map as a runtime fallback.
- The movement decision is NOT the complete server Engine. Downstream traps/content effects, real initiative, enemy AI and campaign transitions are mandatory before hooking it to `tacticsapi.Engine.Apply`.
- Preserve source parity: exploration moves do not consume a tactical turn; combat moves have individual party footprints and advance initiative. Deny enemy turns, terminal/dead actors, overlapping pieces, walls, exits and occupied enemy cells.

## Go attack target parity
- `api/internal/tacticsrules/attack.go` returns attack *decisions*, never damage, RNG, loot, XP or a completed turn. Damage and first-contact initiative must be resolved by the future server engine from trusted authored profiles/state, never from the client.
- Keep exact Chronicles geometries: adjacent Manhattan=1, orthogonal lines, diagonal squares, line ray, line-of-sight intermediate walls/living enemies, and explicit reach bonuses. A ranged exploration attempt is an initiative boundary, NOT a free attack.

## Go initiative scheduler
- `api/internal/tacticsrules/initiative.go` rolls server-owned AGI+1d8 and advances through living combatants. Never accept initiative rolls, ordering, actor ID or agility values from browser payloads. Tests may inject deterministic entropy, but production defaults to cryptographic entropy.
- A scheduling decision does not apply enemy AI/damage/traps or mutate the save; do not wire incomplete `Engine.Apply` that skips these effects. Combat ending must clear tactical footprints and transient effects in the later real reducer.
