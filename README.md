# Tactics of Matthias

RPG táctico isométrico del universo **ShadowOps**. Su origen es el modo Tactics de [Chronicles of Matthias](https://github.com/evilsysadmin/chronicles-of-matthias).

> **Estado: migración en preparación.** Los primeros módulos portables de clases y de geometría de objetivos están en `src/`, con `npm test` y CI. Este repositorio todavía no contiene un juego ejecutable. La versión jugable de Tactics sigue estando en Chronicles mientras trasladamos sus módulos con pruebas y preservamos los guardados existentes.

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
