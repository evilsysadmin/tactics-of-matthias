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

## Server-owned snapshot and action boundary
- `api/internal/tacticsengine` must decode only authenticated run.State and always rebuild terrain from the server-stored Run.Map, not an HTTP-provided grid or client positions. State v1 is strict and versioned.
- The Engine adapter must call legality checks before any game effects and reject fake next runs. Real `Effects` owns ALL damage, AI, traps, combat start/finish, progression and turn resolution; never inject test doubles in a production process.
- Keep end-to-end HTTP→engine→CAS tests. A green adapter test does not imply a complete game engine/Identity service or playable standalone.

## Authored trap effects
- Go `api/internal/tacticsrules/traps.go` implements a narrow but REAL original Tactics content effect: explicit authored one-time flags and damage to living party, with defeat. Missing flags must not satisfy an authored `equals:false` condition.
- Reject unknown trap effects and invalid placements rather than silently omitting content. The backend may ONLY call this with its own authored map manifest plus persisted flags/party; never from a browser-supplied trap definition or a fixture.
- Remaining traps and all other content effect types must be resolved by the later real `Effects` implementation. Do not expose this submodule as a complete game engine.

## Private authored tile content gate
- `StageMoveEntry` must use only previously validated `Run.State`, recheck the actual move, then retrieve authored content from a **server-private** ContentCatalog. Never serialize trap definitions into `Run.Map`/bootstrap responses or derive them from client requests.
- Do not commit `MoveStage.Pending`: it intentionally retains the previous worldVersion and must pass full encounter, enemy AI and scheduler resolution before a valid next version can be persisted.
- Terminal traps may set a validated defeated state; stale/forged turns, mismatched catalog geometry and unknown authored effects must fail closed.

## Private authored map catalog v1
- `api/internal/tacticscatalog` uses an explicit absolute server-side directory; only versioned strict manifests are read via a confined `os.Root`. No default world/maps, remote URLs or client-authored content.
- Always match `Run.Map.Version` to private `ContentCatalog.Revision` plus grid/ID; a content revision change cannot silently alter an existing expedition. Do not quietly drop unsupported authored triggers, interactables, treasures or exits.
- Ash Vault geometry/trap tests are provenance-backed **test fixtures**, not a playable map, runtime manifest, or full content migration.

## Terminal runs are not active worlds
- Keep owner-scoped terminal GET records readable, but POST to a terminal run must fail before any engine effects or storage writes. Do not resurrect a defeated campaign.
- Client `tacticsReadOnlyTerminal` must verify the run, next server version and terminal phase. Render a non-actionable read-only result, never a playable scene or a synthetic victory.

## Enemy engagement visibility gate
- `tacticsrules.EngagedEnemies` checks a server-sourced roster and authored engagement profile; it does not accept ranges, positions or enemy IDs from HTTP actions.
- Keep **Bresenham with diagonal-corner blocking** for sight/detection distinct from straight/diagonal ranged attacks. Proximity without visibility never starts an encounter.
- The result is a living engaged subset. Do not add it to a saved initiative round without correct party deployment and per-subset scheduler/AI handling; no fallback all-enemy turns or client-selected encounters.

## Engaged enemy initiative subset
- Use `RollEngagedInitiative` for a server-authorized encounter, NEVER roll all living map enemies by default. `AdvanceEngagedTurn` must receive the selected server-persisted enemy IDs and an initiative order with exactly those enemies plus living party actors.
- A distant unengaged enemy is not owed a turn and must not keep an encounter alive after the engaged opponents are defeated. Never accept engagedEnemyIDs from the browser or quietly drop a selected enemy's slot.
- These APIs are pure rules until the real combat reducer safely persists initiative + party footprint and resolves AI/effects; do not connect them as a partial gameplay fallback.

## Persisted engagement consistency
- `StateV1.engagedEnemyIds` is mandatory for combat, absent outside combat, and authored/updated only by the authoritative server reducer. Validate every restored combat against `tacticsrules.AdvanceEngagedTurn` (same semantics as live scheduling), not the entire world's living enemy roster.
- Saved combat must have all participating enemy slots and all living party slots, valid/non-phantom formation coordinates, and at least one living engaged enemy. Clear engagement IDs, initiative and tactical footprint together on returning to exploration or defeat.
- Never expose any API accepting client-supplied initiative/engagement IDs. This schema gate is NOT an encounter deployer or full gameplay engine.

## Combat terminal cleanup and authored map integrity
- Lethal trap transitions must clear `engagedEnemyIds` alongside `initiative` and `partyPositions`, including when a non-engaged living enemy remains on the map.
- A normal `Engine.Apply` movement or attack must not rewrite `Run.Map` or switch `MapID`. A map migration requires an explicit server-authored, separately validated transition in a later PR. Never treat a well-formed replacement grid as implicitly authorized.

## Connected Go tactical party deployment
- `PlanPartyDeployment` uses BFS through traversable map cells (not through walls/enemies), finds a connected role-aware cluster near the exploration anchor, avoids server-authored visible blockers, and returns only living canonical party IDs. Never let the client supply blockers or positions.
- In narrow corridors do not omit a living member merely because the preferred 4-slot role cell is missing. If no connected legal deployment fits, reject rather than teleport across masonry or onto enemies/interactive objects.
- Keep Go `X` exit blocking consistent with movement legality. A pure formation is not a full encounter or a saved checkpoint; combine with authoritative engagement roll, effects and CAS later.

## Uncommitted authenticated encounter start
- `StageEncounterStart` combines only server-owned private catalog/revision, living party/enemy positions, canonical connected formation and trusted authored enemy profiles/AGI. It persists the selected engaged subset in the PENDING world, never in a client-supplied action.
- The initial actor may be an ENEMY. Do not advance `worldVersion` or issue an authenticated player checkpoint before that enemy's required action has been resolved by the full real Effects implementation.
- No valid formation or no visible enemy means no combat start. Unknown/map-mismatched content fails closed; never replace it with a local test fixture.

## Enemy AI: authorized single-actor intents
- `DecideEnemyTurn` is pure and checks the **current server initiative enemy** against the persisted `engagedEnemyIds`; the browser must never select an AI enemy, target, movement or authored profile.
- Only `hold` and `cardinal-chase` are supported. Ranged shots use cardinal geometry and authored LOS, while cardinal BFS moves ONE tile towards a real reachable attack position and NEVER attacks after moving. Other AI policies (jump, patrol, awareness, retreat, multi-strike, paralysis) need separate explicit implementations and cannot default to this slice.
- A movement/attack/wait intent does not resolve damage, statuses, initiative or Mongo CAS. Keep the standalone server unavailable until the complete real Effects reducer is present.

## Pending enemy non-damage stage
- `StageEnemyNonAttack` must revalidate the private map revision and current enemy actor, apply only a legal one-cell move or wait, and advance initiative ONCE. Do not touch HP, grant a free attack, or accept client-selected AI/positions.
- When the decision is `attack`, return `ErrEnemyAttackUnresolved` without changing run/version/cursor; do not grant a zero-damage fake hit or skip the enemy's rightful action.
- Any successful stage still retains the old `worldVersion` and cannot become a CAS checkpoint until ALL Effects are implemented. Keep backend unavailable, no test-world runtime or Identity bypass.
