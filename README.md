# Tactics of Matthias

RPG táctico isométrico del universo **ShadowOps**. Su origen es el modo Tactics de [Chronicles of Matthias](https://github.com/evilsysadmin/chronicles-of-matthias).

> **Estado: shell Vite independiente, todavía NO jugable.** Los primeros módulos portables de clases y de geometría de objetivos están en `src/`, con `npm test` y CI. Este repositorio ya contiene un scene graph Three.js real, pero aún no tiene cliente web jugable ni bootstrap SSO propios. La versión jugable de Tactics sigue estando en Chronicles mientras trasladamos sus módulos con pruebas y preservamos los guardados existentes.

## Contratos de independencia

- **Producto propio:** progresión táctica, combate por casillas, habilidades de clase, renderizado isométrico, UI y releases autónomos.
- **Universo compartido, no estado acoplado:** nombres, estética, narrativa y assets compatibles pueden reutilizarse; no depender de la instancia de Chronicles ni intercambiar partidas implícitamente.
- **Identidad:** la integración ShadowOps SSO se validará por separado. No copiar claves ni credenciales.
- **Persistencia:** no se migran ni eliminan partidas de Chronicles hasta tener contratos versionados y ruta explícita de importación/migración probada. No cambiar IDs ni sobrescribir saves existentes.
- **Infraestructura:** los despliegues pertenecen a [shadowops-infra](https://github.com/evilsysadmin/shadowops-infra); este repositorio no publica producción por su cuenta.
- **Assets:** reutilizar referencias autorizadas de Cloudflare R2; evitar duplicar binarios sin necesidad.

## Plan incremental

1. Desacoplar las reglas propias de Tactics *dentro de Chronicles*, preservando API pública y pruebas.
2. Incorporar el runtime y shell mínimos en este repositorio, con tests y un arranque reproducible; mantener el Tactics original como rollback.
3. Desacoplar contenido, perfiles, progresión, generación y contrato de guardados; introducir migraciones idempotentes de partidas solo tras test de ida/vuelta.
4. Configurar CI y staging independiente; validar juego real, escritorio/móvil, recuperación, SSO y partidas históricas.
5. Retirar el modo Tactics de Chronicles únicamente tras comprobar la paridad del nuevo juego y ofrecer una transición segura.

Trabajo pequeño, **PR a PR**, sin despliegues ni borrados destructivos por comodidad. Ver [AGENTS.md](AGENTS.md) una vez incorporado.

### Presentación isométrica portable

- `src/render/tacticsPartyPresentation.js`: formación de los cuatro héroes, orientación de cámara y colores de selección.
- `src/render/tacticsLighting.js`: selección determinista y limitada de antorchas según muros y tipo de sala. **Solo son parámetros de escena**, todavía no se ha migrado Three.js ni hay viewport ejecutable.

### Reglas portadas

- `src/tacticsProfiles.js`: identidad y habilidades de las cuatro clases originales.
- `src/tacticsTargetGeometry.js`: distancias por patrón y obstrucciones de línea de visión. El runtime inyectará los tiles y las posiciones de enemigos; no se enlaza todavía con el mundo ni los saves de Chronicles.

### Contrato de escena para el futuro renderer

- `src/adapters/tacticsSceneFrame.js` acepta **solo** la proyección de escena v1 que hoy genera `chroniclesProjectSceneModel`: terreno, posiciones, actor seleccionado y acciones legales.
- Rechaza escenas incompatibles o inválidas, devuelve datos desligados e inmutables y omite historia, progreso, flags y guardados. Se preservan las reglas de validación antes de dibujar.
- **No arranca ni simula una partida:** todavía deben migrarse el world bootstrap autoritativo, el loop táctico real y el renderer 3D. Nunca admitir un mapa fallback en runtime para aparentar disponibilidad.

### Puente de entrada para acciones del tablero

- `src/adapters/tacticsActionIntent.js` solo transforma clics de **mover/atacar** en intenciones cuando el frame contiene exactamente esa acción legal. No fabrica capacidades ni ejecuta turnos.
- El motor táctico **debe volver a validar** cada intención al aplicar cambios; un frame puede quedar obsoleto entre render y clic. Esperar al runtime real para habilitar pasar, usar o habilidades con su iniciativa.

### Frontera real de turnos

- `src/tacticsTurnResolution.js`: transición de exploración a combate, avance del cursor de iniciativa, ataque forzado y cambio de mapa, **extraída de la lógica jugable de Chronicles**.
- Recibe puertos del motor real (`activeEnemies`, `combatActive`, `advanceExplorationEnemies`, `startInitiativeCombat`, `advanceCombatInitiative`, `prepareExplorationSpawn`). No crea enemigos o campañas de sustitución. El futuro runtime deberá conectar implementaciones reales con pruebas de paridad, igual que el adaptador que todavía vive en Chronicles.

### Acción jugable de la UI al motor (sin runtime ficticio)

- `src/tacticsActionDispatch.js` es una copia exacta del módulo que el Tactics integrado en Chronicles usa para ejecutar **movimiento y ataque legales**, a través de puertos del motor.
- `src/adapters/tacticsFrameActionPipeline.js` compone la proyección de escena, los clics legales y la ejecución mediante puertos de mundo/turnos. Vuelve a validar actor, mapa, movimientos y objetivos en el estado **actual**, por lo que un frame obsoleto no tiene autoridad sobre el motor.
- El test de integración prueba de extremo a extremo **el contrato entre estos módulos** usando dobles de puertos. Es una prueba de arquitectura, no significa que el juego standalone ya tenga mapas/IA/renderizador reales.

### Sesión headless (sin juego standalone todavía)

- `src/tacticsSession.js` exige que el host entregue un mundo remoto **ya validado** y puertos auténticos para hidratar estado, proyectar escena y ejecutar acciones. Rechaza run local, terminal, versiones inválidas y mapas divergentes; la marca remota **no sustituye autenticación SSO**.
- Una acción debe incluir la revisión del frame actual; se rechazan clics obsoletos y se reconstruye el frame desde el estado actualizado solo después de resolver con las reglas. El host sigue siendo responsable de persistir el checkpoint/progresión y de validarlo contra el backend.
- Las pruebas demuestran el contrato usando dobles **solo en tests**. Todavía no existe implementación standalone de los puertos del mundo, renderizador visual ni bootstrap autenticado.

### Geometría del mapa remoto real

- `src/adapters/tacticsWorldScene.js` extrae **suelo, muros y caras expuestas** directamente del manifest `world.map.grid` instalado tras un bootstrap remoto autoritativo (sin incluir mapas jugables locales).
- Reproduce el orden, geometría y reglas de contorno del plan isométrico existente de Chronicles; lo verificamos contra el grid original de la Cripta de las Ocho Casillas, conservado **solo dentro de las pruebas**.
- `tacticsSceneModelFromRemoteWorld` combina esa geometría con los personajes, enemigos y acciones legales **ya proyectados por el host real**; no atribuye HP, posiciones, IA, interacción o estados inventados. La sesión headless puede consumir el resultado.
- No sustituye aún la instalación de mapas, hidratación, IA, renderer 3D o SSO. Un cambio de región exige recibir el mapa correspondiente del backend, no reutilizar un grid antiguo.

### Coordenadas 3D derivadas del mundo real

- `src/render/tacticsSpatialRenderPlan.js` transforma las casillas validadas de un frame (obtenido del mapa remoto actual) en coordenadas isométricas 3D, base del tablero, volúmenes de muro, puntos de antorcha, posiciones de actores y marcadores legales.
- Conserva la escala del renderer original (`2.45` unidades/celda) y la centralización geométrica; el módulo sigue siendo puro e independiente de Three.js para facilitar pruebas y comparación de paridad.
- **Aún no pinta meshes ni anima sprites**. El siguiente paso será montar un renderizador que consuma este plan y disponga los assets y personajes reales, conectado a una sesión autenticada.

### Escena Three.js (renderer visual en construcción)

- `src/render/tacticsThreeScene.js` construye **objetos Three.js reales** desde un frame validado: suelos y muros instanciados, luces, cámara, marcadores legales y anillos localizadores de los actores vivos. Reutiliza `tacticsSpatialRenderPlan`.
- `createActorVisual` permite inyectar los modelos/sprites auténticos cuando su pipeline R2 esté listo. Los localizadores de actor no pretenden ser personajes definitivos; las geometrías/materiales creados internamente se liberan con `dispose()`, sin liberar assets compartidos del host.
- El picking se restringe a marcadores de movimientos/ataques legales y selección de grupo; no concede permiso para ejecutar turnos ni acepta golpes sobre objetos externos a la escena.
- **Falta el montaje WebGL en navegador, sus controles, auth, assets definitivos, puertos del mundo y staging**. Los tests de Node validan un scene graph Three.js real, no equivalen a smoke visual de Chromium.

### Montaje real en canvas WebGL

- `mountTacticsThreeViewport({ element, frame, onAction })` crea un `THREE.WebGLRenderer`, monta el canvas, adapta la cámara y el tamaño al contenedor, calcula picking por raycast y entrega solo intenciones legales. El host debe someterlas al motor autoritativo.
- `updateFrame(nextFrame)` valida y crea la escena nueva **antes** de desmontar la anterior; frames inválidos no dejan el viewport en negro. `dispose()` limpia canvas, observadores, listeners y recursos Three.js.
- Los tests verifican el ciclo de vida y acciones con un renderer/raycaster inyectados (sin un navegador). **Faltan smoke WebGL real, app shell, autenticación, sprites originales y runtime de mundo** antes de servir la página a usuarios.

### Host navegador — ciclo de sesión autenticada

- `src/browser/tacticsGameHost.js` integra **únicamente** el ciclo de vida: recibe un loader de mundo **autenticado por el backend**, puertos del juego real, confirmación de checkpoint y un viewport WebGL. No provee ni finge esas integraciones.
- Solo monta el canvas tras `tacticsCreateSession`. Cancela arranques obsoletos y desmonta escenas al reiniciar. Una acción legal no cambia la escena hasta recibir `commitTransition(...)=true`; bloquea dobles acciones y detiene el juego si falla el guardado o renderizado.
- Testea todo con loaders/renders de prueba **solo en tests**. Todavía no hay cliente de SSO, API Tactics ni gestor de guardados independiente, por lo que no se debe publicar un acceso jugable.

### Interfaz del host Tactics

- `src/browser/tacticsAppShell.js` monta una pantalla propia con contenedor WebGL, estado de autenticación/arranque, error y reintento; `tacticsAppShell.css` contiene el estilo adaptable a móvil.
- Si faltan los puertos auténticos del juego/SSO, muestra **servidor pendiente** sin montar escena falsa ni ofrecer una partida inexistente. Los mensajes de error no imprimen excepciones crudas ni posibles secretos.
- Es un componente de integración, **no todavía el punto de entrada publicado**: faltan Vite build, módulo de SSO real, API y tests de Chromium.

### Arranque web del repositorio independiente

El repositorio ya dispone de una página Vite compilable:

```sh
npm ci --ignore-scripts
npm test
npm run dev
npm run build
```

En `http://127.0.0.1:5173` se muestra la interfaz Tactics y el estado **«Servidor de Tactics pendiente»**. Esto es intencional: **no hay cuenta, partida ni mundo ficticios**. La interfaz se habilitará cuando ShadowOps Infra y una API Tactics independiente puedan proporcionar loader SSO autorizado, hidratación de mundo/reglas y confirmación de checkpoints; los tests Node actuales no sustituyen una prueba de navegador WebGL.

La CI comprueba tests y `vite build` en cada PR. Este workflow de código fuente no despliega a Cloudflare Pages ni modifica la Home actual de Chronicles.

### Primera frontera HTTP propia (sin despliegue)

- `api/internal/tacticsapi` incorpora un **handler HTTP real** con `GET /api/tactics/runs/{id}/bootstrap` y `POST /api/tactics/runs/{id}/actions`. La lectura exige identidad verificada y un run activo perteneciente al usuario; la escritura exige `expectedWorldVersion`, acción estricta, validación del **motor en servidor** y actualización atómica CAS por propietario/ID/versión.
- El navegador nunca puede enviar un estado de mundo arbitrario para guardarlo. No hay aceptación optimista de ataques, movimientos, otros propietarios, runs terminales ni versiones obsoletas. Respuestas sin cache y errores que no revelan información del almacén o del JWT.
- El handler **no incorpora auth simulada ni almacén en producción**: `Authenticator` debe verificar el token ShadowOps con audiencia propia de Tactics; `Store` debe ser persistente y owner-scoped; `Engine` debe ejecutar las reglas reales. El constructor rechaza los tres ausentes. Las pruebas Go usan puertos de prueba únicamente para verificar aislamiento, conflictos y seguridad.
- La API aún no tiene binario de arranque, IdP/SSO, store Mongo propio ni servidor de juego conectado, así que **NO es desplegable/jugable**. Los guardados Chronicles quedan completamente independientes.

### Respuesta de acciones: servidor como autoridad

`tacticsReconcileServerSession(previous, confirmedWorld, ports)` exige el **mismo run ID**, la siguiente versión exacta del mundo remoto y estado activo; vuelve a hidratar desde el **resultado del servidor** y a proyectar la escena. El controlador `createTacticsGameHost` ya no considera `true` un ACK suficiente ni muestra como definitiva la predicción del cliente. Esto permite que combate, IA y resultados aleatorios decididos en backend se reflejen correctamente. Ante resultado inválido o render fallido se suspende la sesión hasta nuevo bootstrap; no escribe nada en guardados Chronicles.

### HTTP cliente de runs Tactics (sin login habilitado)

- `src/browser/tacticsRunApi.js` implementa GET bootstrap y POST acciones sobre la API Go de #17. Exige una **URL HTTPS de origen explícitamente aprobado**, un run ID y un proveedor real de bearer con audiencia `tactics`. No guarda tokens ni soporta cookies del juego, redirecciones, URLs con credenciales o HTTP no-local.
- Envía solo `expectedWorldVersion` y las propiedades permitidas de la acción. Valida runId, origen remoto, estado activo, manifest del mapa y versión del resultado; el host de #18 usa **ese mundo devuelto por servidor** para pintar la escena. Los errores de red/autorización no incluyen ni token ni respuesta cruda.
- Se puede conectar cuando SSO habilite de verdad la audiencia `tactics` y exista backend/almacén/rules del juego. **No está cableado a `main.js`**, porque Identity todavía no concede audiencias para Tactics ni hay despliegue API. No se debe usar un token de Chronicles ni fabricarlo en runtime.

### Tactics Mongo storage P0
- `api/internal/tacticsmongo` provides an actual `tacticsapi.Store` in the **`tactics_runs`** collection of an explicit `shadowops_tactics_staging` / `shadowops_tactics_production` database. A separate `shadowops_tactics_ci` database is used only against local disposable MongoDB 7 in CI.
- Read queries bind both immutable run ID and authenticated owner; updates match **ID + owner + expected worldVersion + active status** in one atomic MongoDB UpdateOne, with majority write concern, no upsert and no terminal-run resurrection. JSON engine snapshots remain opaque and unmodified by BSON mapping.
- New integration checks run against a real ephemeral Mongo container to verify isolation, replay rejection, terminal locks and 20 competing actions across two store handles; no production secrets or connections are configured.
- This store does not create runs or implement tokens, initiative or the game engine. Until SSO and an authoritative game server are wired, **no standalone run is playable/deployed**.

### Decisiones de movimiento del motor Go (paridad de Chronicles)

`api/internal/tacticsrules/movement.go` porta a un módulo **Go puro y comprobable** la legalidad de desplazamiento que aplica el Tactics integrado en Chronicles: direcciones N/E/S/O, muro `#` y salida `X` bloqueados, enemigos vivos que ocupan casillas, posición individual y otras unidades de la compañía que bloquean en combate, actor de iniciativa correcto, y rechazo de movimiento de jugadores caídos, turno enemigo y estados terminales. La exploración **no consume turno**; una acción de combate sí debe consumir uno.

`DecideMove` devuelve una decisión validada y el indicador de turno debido, **pero no aplica traps, cambios de mapa, pasos de enemigos o iniciativa**. Esas responsabilidades siguen pendientes y necesitan un motor/ports del servidor antes de conectar `tacticsapi.Engine.Apply`. Las pruebas de paridad usan solo una copia del grid canónico de la Cripta de las Ocho Casillas en `*_test.go`; no hay mundos falsos en producción.

### Ataques: geometría de clases y línea de visión en Go

`api/internal/tacticsrules/attack.go` porta **la geometría de objetivos del Tactics original** (`tacticsAttackDistance` y `tacticsLineIsClear`): adyacente de Matthias, ortogonal de torre, diagonal de alfil y líneas de caballo, con alcance derivado exclusivamente de perfiles del servidor y modificadores validados. Verifica que el enemigo esté vivo, que el actor tenga la iniciativa y que no haya muro ni enemigo vivo intermedio. Las pruebas Go comparan las geometrías y casos del proyecto fuente y emplean la cripta real solo como fixture de test.

**No inflige daño ni comienza combates por su cuenta.** Un ataque solicitado en exploración devuelve `RequiresInitiative`: la creación del encuentro y su primer turno deben suceder **antes** de cualquier daño, exactamente como en Chronicles. En combate devuelve la obligación de avanzar turno. No conectar un `Engine.Apply` sin portar el resto de la iniciativa, IA y efectos.

### Planificador de iniciativa Go

`api/internal/tacticsrules/initiative.go` incorpora **AGI + 1d8** con desempates fieles al planificador de Chronicles (puntuación, AGI, tirada, tipo e ID), además de seleccionar el siguiente actor, saltarse los caídos, pasar de ronda y terminar encuentros cuando no quedan enemigos o ha caído toda la compañía. La agilidad es aportada por el motor del servidor, y la fuente de azar es `crypto/rand` (inyectable solo para pruebas deterministas).

Una orden corrupta, un enemigo vivo sin turno o un personaje repetido se rechaza en vez de conceder acciones extra. **No está conectado aún a `tacticsapi.Engine.Apply`**: falta resolver el efecto real de cada acción, IA enemiga, trampas, buffs y la persistencia del resultado. Las pruebas Go cubren orden, empates, wrap, enemigos muertos, derrota, victoria y corrupción.

### Primera conexión real: HTTP → mundo guardado → reglas → efectos obligatorios

`api/internal/tacticsengine` implementa `tacticsapi.Engine` como **frontera de validación**, no como simulador nuevo. `DecodeState` exige un snapshot del servidor `version:1`, con mapa, posiciones, personajes, enemigos y orden de iniciativa coherentes; el grid se deriva **únicamente del mapa del run autoritativo**, nunca del navegador. Rejecta inventar grids, fases, actores, solapamientos, órdenes duplicadas y versiones de estado desconocidas.

`Engine.Apply` valida acciones por las reglas Go de movimiento/alcance y entrega únicamente la decisión legal al puerto obligatorio `Effects`, encargado de **todos** los efectos reales, IA, trampas, turnos y recompensas. Comprueba que el resultado conserve propietario/ID, incremente exactamente la versión y produzca otro estado válido antes del CAS de la API. La suite incluye un recorrido HTTP integrado con almacenamiento y efectos **dobles solo en pruebas**. Aún **no hay resolver Effects real ni servicio desplegable**, y los tests de esa frontera no equivalen a una partida standalone.

### Primera resolución concreta de efecto: trampa authored Tactics

`api/internal/tacticsrules/traps.go` porta de Chronicles el efecto **`set` (bandera booleana del mundo)** y **`damage-party` (daño a los miembros vivos, con derrota al caer todos)**, disparado al entrar realmente en una casilla con una trampa definida por el mapa del servidor. Sigue el contrato original de `when: [{key, equals:false}]`: una bandera inexistente NO equivale a `false`, y una trampa marcada como gastada no vuelve a herir al regresar. El código **no muta el estado anterior** y rechaza efectos desconocidos, condiciones malformadas, ubicaciones en muros, definiciones duplicadas y valores extremos.

La trampa de pinchos del test original de Chronicles se usa **únicamente como fixture de paridad Go**. Los contenidos reales siguen necesitando un manifiesto de mapa server-owned y resolver el movimiento, encuentro, turno enemigo y guardado en el motor principal; todavía **no se ha activado este efecto en el Engine de producción** ni inventado ningún mapa de inicio.

### Entrada a una trampa sin filtrar contenido oculto al navegador

`api/internal/tacticsengine/tile_entry.go` conecta un movimiento **revalidado** con el mundo `Run.State` del servidor y un `ContentCatalog` estrictamente privado. Comprueba que la versión de terreno authored del catálogo coincide exactamente con el mapa guardado, aplica los efectos reales de pinchos/flags de #25 sobre una copia y devuelve `MoveStage`. El stage **mantiene la versión anterior**: no puede pasar el CAS hasta que la resolución de iniciativa, encuentro e IA complete el turno. Si una trampa causa un knockout completo, el stage pasa a derrota, limpia la formación táctica y el esquema v1 admite la transición terminal validada.

Los manifiestos de trampas **no se incluyen en `Run.Map` ni en el bootstrap HTTP**, evitando revelar trampas ocultas. El catálogo real está pendiente y nunca se sustituye por fixtures fuera de las pruebas. El paso final que compone `MoveStage` con el resto de efectos reales **aún no está habilitado**.

### Private authored map catalog (v1)

`api/internal/tacticscatalog` now reads a **real server-private, explicitly configured absolute directory** containing `{mapId}.json` manifests with `schemaVersion=1`, positive `mapRevision`, `mapId`, `grid` and `traps`. Reads are path-confined by Go `os.OpenRoot`, size-limited and strict: missing maps, unsupported events/treasures/exits, malformed traps or unknown effects **fail closed**. There are no bundled production maps or fallback world.

A run's `Map.Version` is matched with the private catalog revision **even when grid geometry is unchanged**, preventing gameplay content drift under existing saves. Tests reproduce the **real Ash Vault v3 vent and geometry from Chronicles only as a fixture** and verify no trap definitions leak in API data. Other map features still need deliberate gameplay porting before an authored manifest is eligible for live use. The server is not deployed and the standalone game remains unavailable.

### Read-only terminal expeditions

Server-authorized `GET /api/tactics/runs/{id}/bootstrap` now returns owner-scoped terminal runs as immutable history, while `POST /actions` still rejects them with 409 before any engine effect. The browser transport validates defeat/completion responses; `tacticsReadOnlyTerminal` verifies run identity, exact next version, phase, and fallen party. The host drops its actionable WebGL viewport and displays an explicit **read-only end result**, including on reload, without inventing a recovered party or restarting combat. No local save/SSO fallback is enabled.

### Authoritative encounter visibility (Go)

`api/internal/tacticsrules/encounter.go` now ports **Bresenham enemy sightline** and the **proximity/engagement decision** from Chronicles. Solid masonry and blocked diagonal corners prevent detection, including between non-cardinal/non-45° tiles. The roster is filtered to **living, visible, in-range enemies**, using only server-authored attack reach (1..16) and optional engage range (1..64). Range defaults to attack reach, matching the Chronicles contract. Missing/malformed authoring, dead parties, enemy turns and corrupted rosters fail closed.

**Sight is not the same as attack.** Ranged attack geometry in `attack.go` remains independently constrained. This pure selector is NOT a complete encounter reducer: deployment, AGI+1d8 roll for the selected enemy subset, enemy AI, damage and persistence are still pending, and no runtime SSO/game map was enabled.

### Initiative only for engaged enemies

`tacticsrules.RollEngagedInitiative` now composes the **real enemy visibility selector** with existing **AGI+1d8** and rolls only the enemies currently detecting the party, preserving Chronicles' engaged-subset semantics. A distant monster does not get initiative simply by existing in the same map. `AdvanceEngagedTurn` validates the persisted selected roster, rejects phantom/missing actor slots, skips dead enemies and ends combat when the final **engaged** enemy dies even if distant enemies remain alive outside that encounter.

These are **pure server decisions** and do not persist a round, deploy the party or execute AI/damage. The eventual world reducer must persist the selected engaged enemy IDs, deploy the party, and safely use this scheduler; its HTTP API does not accept client-provided engagement rosters.

### Persisted engaged-enemy roster is mandatory for combat saves

`tacticsengine.StateV1` now persists `engagedEnemyIds` for combat only. `DecodeState` validates this private server-authored subset against the SAME `tacticsrules.AdvanceEngagedTurn` used for actual scheduler decisions: exact participating enemy slots, all living party actors, no ghost/duplicate/missing actors, valid footprint and non-terminal combat. **Living enemies elsewhere on the map are not required in initiative.** Exploratory or defeated runs must clear initiative, footprint and the engaged roster. The server still never accepts these values from client actions.

This corrects an incompatibility left by #30; it does not yet construct the correct party deployment, start an encounter, perform real AI/damage or expose a deployable standalone service.

### Terminal trap and map integrity hardening

A fatal trap during an active encounter now clears **all three linked pieces**: `initiative`, `partyPositions` and `engagedEnemyIds`, so the defeated run validates as terminal rather than leaving phantom turns. Tests exercise the live combat-to-defeat stage with a distant enemy still alive.

`validNext` also forbids changing **the current map's ID, revision, grid or title** through a normal movement/attack checkpoint, even when the replacement is structurally valid. Future authored map transitions must pass an explicit separate contract; they are not implicitly granted to arbitrary effects. No Tactics API or Chronicles production behavior is deployed.

### BFS party deployment in Go (server-only)

`tacticsrules.PlanPartyDeployment` ports the real `chroniclesPartyGridFootprint` selection: **BFS radius 6**, up to **24 walk-reachable candidates**, connected clusters, a role-weighted assignment for Matthias/rook/bishop/knight, and facing-relative positioning. It uses the server's current map, alive party, real enemy positions and visible blocking authored content only. It never spawns a member inside an enemy/object or across a wall; when a cramped map cannot fit the living team it **fails closed** rather than teleports. For narrow fallback clusters, living actors are all retained (fixes a legacy sparse-party slot omission). Exits `X` remain blocked, matching Tactics Go movement rules.

The function is pure, uses no client-authored positions, and does not roll initiative, mutate saves or make a playable backend; the encounter-start reducer must combine it with `RollEngagedInitiative` and all remaining effects.

### Uncommitted encounter start stage — Go

`tacticsengine.StageEncounterStart` now composes, on the **server-run snapshot**, the exact private map revision check, `PlanPartyDeployment` connected formation, `RollEngagedInitiative` visible enemy subset and the saved `StateV1.engagedEnemyIds`/initiative/positions. It refuses missing authored enemy profiles, incomplete AGI, invalid catalog, malformed run and a map with no room for a legal formation. The first scheduled actor is recorded and its enemy-first status is explicit.

**Safety boundary:** the generated `EncounterStage.Pending` retains the OLD worldVersion, so it cannot pass the HTTP engine's next-version check or reach Mongo CAS. A real resolver must still execute opening enemy AI, damage, content effects and all other turn consequences before creating the next-version run. Authored content inputs are backend-only; no public endpoints, example map or SSO were added.

### Server-only enemy turn decisions (first tactical AI slice)

`api/internal/tacticsrules/enemy_turn.go` ports the **single-action** contract of Chronicles' `chroniclesResolveEnemyActor` for the authored `hold` and `cardinal-chase` movement policies: the current initiative enemy selects the nearest legal living hero (frontline/rotation tie-breaks), attacks if already in reach, otherwise follows BFS N/E/S/W to the closest attackable tile (one move only), or waits when unreachable. Ranged attack checks orthogonal reach and optional authored line-of-sight; it never equates detection with attack. The decision always validates the engaged initiative subset and all living occupied cells, including enemies outside that subset.

**Explicitly unsupported:** knight jumps, patrols, roam/awareness, retreat, paralysis, multi-strike sequences and numeric damage. Such policies must be ported deliberately; never silently substitute cardinal-chase or invent hits. This pure decision returns no actual damage, does not execute a move or advance a run, and is not yet wired to `Effects` or SSO. Tests cover walls/LOS, target priority, pathfinding, blocking, wait, corruption and turn exclusivity.
