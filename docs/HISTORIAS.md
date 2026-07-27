# Historias De Usuario - DeployDeck

## Convenciones

- ID de historia: `HU-XXX`.
- Prioridad: `Alta`, `Media`, `Baja`.
- Tipo: `Producto`, `Tecnica`, `Operativa`.
- Criterios de aceptacion escritos en formato verificable.
- Las tareas tecnicas no implican implementacion final de UI salvo que se indique.

## HU-001 - Validar Prerequisitos Locales

Prioridad: Alta  
Tipo: Operativa  
Fase: MVP Git

### Historia

Como desarrollador Salesforce, quiero que DeployDeck valide mi entorno local al iniciar para evitar fallos a mitad del flujo de promocion.

### Objetivo

Detectar de forma temprana si el usuario puede ejecutar el flujo completo: Git, Salesforce CLI, plugin `sfdx-git-delta`, remoto `origin`, ramas configuradas, alias Salesforce y working tree limpio.

### Tareas

- Crear modulo `internal/prereq` o implementar dentro de `internal/app` usando servicios de `internal/git`, `internal/salesforce` y `internal/delta`.
- Implementar wrapper comun en `internal/exec` para ejecutar comandos externos.
- Validar existencia de binarios: `git`, `sf`.
- Validar plugin `sfdx-git-delta` mediante `sf plugins --json` o salida equivalente si JSON no esta disponible.
- Validar que el directorio actual pertenece a un repo Git.
- Validar remoto `origin`.
- Validar working tree limpio.
- Validar ramas destino configuradas local o remotamente.
- Validar alias Salesforce configurados mediante `sf org list --json`.
- Validar versiones minimas de `git`, `sf` y `sfdx-git-delta` segun `minVersions` de configuracion.
- Validar que `.deploydeck/` esta en el `.gitignore` del repo (bloqueante: ahi se generan manifests y runs).
- Adquirir lock de instancia unica (`.deploydeck/lock`); bloquear si otra instancia esta activa sobre el repo.
- Detectar hooks de Git del repo que puedan interferir (informativo).
- Detectar `gh` disponible y autenticado (informativo, no bloqueante: habilita crear PR desde la TUI en HU-014).
- Mostrar resultado por chequeo: OK, warning o error bloqueante.
- Permitir copiar o visualizar comandos sugeridos para corregir errores.
- Exponer los mismos chequeos como subcomando CLI `deploydeck doctor` con exit code distinto de cero si hay bloqueantes.

### Criterios De Aceptacion

- Dado un entorno correcto, cuando el usuario abre DeployDeck, entonces se muestra que todos los prerequisitos criticos estan OK.
- Dado que `git` no esta disponible, cuando se ejecuta el chequeo, entonces se bloquea el flujo y se muestra una accion correctiva.
- Dado que el working tree tiene cambios, cuando se intenta iniciar una promocion, entonces se bloquea la modificacion de ramas.
- Dado que falta el plugin `sfdx-git-delta`, cuando se valida el entorno, entonces se muestra el comando sugerido de instalacion.
- Dado que un alias Salesforce configurado no existe, cuando se valida el entorno, entonces se muestra el alias faltante y se bloquea la validacion contra esa sandbox.
- Dado que `.deploydeck/` no esta en `.gitignore`, cuando se valida el entorno, entonces se bloquea el flujo y se ofrece anadir la entrada.
- Dado que otra instancia de DeployDeck esta activa sobre el repo, cuando se intenta iniciar, entonces se bloquea con mensaje indicando el proceso propietario del lock.
- Dado que una version instalada no cumple el minimo configurado, cuando se valida el entorno, entonces se muestra la version actual, la requerida y el comando de actualizacion.

### Informacion Tecnica

Comandos base:

```bash
git rev-parse --is-inside-work-tree
git status --short --branch
git remote -v
git branch --all
sf --version
sf plugins
sf org list --json
```

Datos esperados:

```go
type PrereqCheck struct {
    Name        string
    Status      string
    Blocking    bool
    Detail      string
    FixCommand  string
}
```

### Mockup TUI

Ver `docs/MOCKUPS_TUI.md`, pantalla `Doctor / Prerequisitos`.

### Test E2E

- Harness: repo git temporal + `exec` falso para `git --version`, `sf --version`, `sf plugins` y `sf org list`.
- Seed: repo con/sin remoto `origin`, `.gitignore` con/sin `.deploydeck/`, plugin `sfdx-git-delta` presente/ausente, versiones por debajo y por encima de `minVersions`, lock ya tomado.
- Asserta: cada `PrereqCheck` devuelve el `Status` esperado (OK/warning/bloqueante); el plugin ausente devuelve bloqueante con el `FixCommand` de instalacion y el alias inexistente nombra el alias faltante; la version por debajo de `minVersions` bloquea mostrando version actual, requerida y comando de actualizacion; el lock tomado bloquea indicando el proceso propietario; `deploydeck doctor` sale con exit code != 0 si hay bloqueantes.
- Variantes: `git` ausente, plugin `sfdx-git-delta` ausente, working tree sucio, alias inexistente, `.deploydeck/` no ignorado, otra instancia con el lock.
- En CI: si (exec falso; binarios reales cubiertos en integracion).

## HU-002 - Buscar Commits Por Ticket

Prioridad: Alta  
Tipo: Producto  
Fase: MVP Git

### Historia

Como desarrollador, quiero introducir un ticket o incidencia para encontrar automaticamente los commits y ramas relacionados.

### Objetivo

Reducir errores de seleccion manual buscando por mensaje de commit, nombre de rama y diferencias contra la rama destino.

### Tareas

- Crear funcion de busqueda por patron de ticket.
- Buscar commits con el ticket en el mensaje.
- Buscar ramas remotas y locales que contengan el ticket.
- Para cada rama candidata, calcular commits presentes en origen y ausentes en destino.
- Normalizar datos de commit: SHA, autor, fecha, mensaje, merge commit, ramas asociadas.
- Marcar commits ya presentes en destino por SHA y por equivalencia de contenido (`git cherry` / patch-id): en este flujo los cambios llegan a los ambientes via cherry-pick con SHA distinto.
- Forzar una unica rama origen por ejecucion: no se mezclan commits de ramas distintas.
- Para promociones entre ambientes (ej. `INT -> UAT`), sugerir por defecto como origen el ambiente anterior (lo validado en sandbox), no la rama feature.
- Ordenar resultados en orden topologico estable para cherry-pick.
- Avisar cuando la rama origen ya no exista y la busqueda dependa solo del grep de mensajes (los commits sin ticket en el mensaje seran invisibles).
- Avisar cuando el historial sugiera squash merges (equivalencia no detectable estaticamente).
- Manejar tickets sin resultados con pantalla accionable.

### Criterios De Aceptacion

- Dado un ticket existente en mensajes de commit, cuando el usuario busca, entonces se listan los commits relacionados.
- Dado un ticket existente en nombre de rama, cuando el usuario busca, entonces se muestran ramas candidatas.
- Dado un commit ya presente en destino por SHA, cuando aparece en resultados, entonces se marca como ya aplicado y no queda seleccionado por defecto.
- Dado un commit cuyo contenido ya fue cherry-pickeado a destino con otro SHA, cuando aparece en resultados, entonces se marca como equivalente ya aplicado (`git cherry`).
- Dado que hay commits en varias ramas candidatas, cuando el usuario continua, entonces debe elegir una unica rama origen para el run.
- Dado un merge commit, cuando aparece en resultados, entonces se identifica visualmente y aparece bloqueado.
- Dado un ticket sin resultados, cuando finaliza la busqueda, entonces la TUI muestra alternativas: busqueda manual, cambiar ticket o seleccionar rama origen.

### Informacion Tecnica

Comandos base:

```bash
git log --all --grep <ticket> --format='%H%x09%h%x09%an%x09%aI%x09%P%x09%s'
git branch --all --list '*<ticket>*'
git rev-list --reverse --topo-order origin/<target>..origin/<source>
git merge-base --is-ancestor <commit> origin/<target>
git cherry origin/<target> origin/<source>
git show <sha> | git patch-id --stable
```

Reglas:

- Un commit es merge si tiene mas de un parent.
- Un commit ya aplicado (por SHA o equivalencia) no debe seleccionarse por defecto.
- Un commit es equivalente-ya-aplicado si `git cherry` lo marca con `-`.
- El orden final debe ser topologico (`git rev-list --reverse --topo-order`): las fechas de autor no sobreviven fiablemente a rebases y amends, y un orden cronologico puede invertir el orden real de la rama.
- Si el repo usa squash merges, la equivalencia por contenido no es detectable estaticamente: documentar la limitacion y apoyarse en el manejo de pick vacio (HU-006). Ver DEC-001 en `docs/ARQUITECTURA.md`.

### Mockup TUI

Ver `docs/MOCKUPS_TUI.md`, pantalla `Busqueda De Ticket` y `Seleccion De Commits`.

### Test E2E

- Harness: repo git temporal, sin org.
- Seed: `main` + rama destino `UAT` + dos ramas candidatas cuyo nombre contiene el ticket (`feature/<ticket>`, `hotfix/<ticket>`) con commits del ticket (fechas de autor invertidas respecto al orden topologico para que topo != cronologico); uno ya presente en `UAT` por SHA identico (mergeado con merge commit, no squash) y otro ya cherry-pickeado a `UAT` con otro SHA; un merge commit; un commit que menciona dos tickets.
- Asserta: se listan los commits del ticket y las ramas candidatas por nombre; el commit que menciona dos tickets se lista al buscar cualquiera de los dos; el ya presente por SHA se marca por `git merge-base --is-ancestor` y el equivalente por `git cherry`, y ninguno queda seleccionado por defecto; el merge aparece bloqueado; continuar con varias ramas candidatas exige elegir una unica rama origen; el orden es topologico estable.
- Variantes: rama origen inexistente (solo grep de mensajes), historial con squash (aviso), ticket sin resultados.
- En CI: si.

## HU-003 - Seleccionar Commits Manualmente

Prioridad: Alta  
Tipo: Producto  
Fase: MVP Git

### Historia

Como desarrollador, quiero seleccionar los commits exactos que se van a promocionar para controlar el alcance del despliegue.

### Objetivo

Evitar que se promocionen commits incorrectos, incompletos o ya presentes en la rama destino.

### Tareas

- Implementar lista seleccionable con soporte de teclado.
- Mostrar SHA corto, mensaje, autor, fecha y flags.
- Bloquear seleccion de commits ya presentes (por SHA o equivalencia) salvo override explicito futuro.
- Bloquear seleccion de merge commits: no soportados en MVP (su cherry-pick requiere `-m`).
- Calcular, por fichero tocado por la seleccion, los commits intermedios no seleccionados en `origin/<target>..<source>` que tocan el mismo fichero, y mostrar warning de dependencia (riesgo de promocion parcial).
- Avisar cuando un commit mencione otros tickets ademas del buscado.
- Mostrar resumen de cantidad seleccionada.
- Permitir reordenar commits solo en modo avanzado, mostrando advertencia: alterar el orden topologico aumenta el riesgo de conflictos, porque cada commit se escribio sobre el estado que dejo el anterior.
- Confirmar seleccion antes de pasar a creacion de rama.

### Criterios De Aceptacion

- Dado un listado de commits, cuando se renderiza la pantalla, entonces cada fila muestra datos suficientes para identificar el cambio.
- Dado un commit ya presente en destino, cuando el usuario intenta seleccionarlo, entonces la TUI impide la seleccion o muestra advertencia bloqueante.
- Dado un merge commit, cuando se muestra la lista, entonces aparece bloqueado y no es seleccionable.
- Dado que un fichero tocado por la seleccion tiene commits intermedios no seleccionados de otros tickets, cuando el usuario confirma, entonces se muestra warning de dependencia por fichero antes de continuar.
- Dado un commit que menciona varios tickets, cuando se muestra en la lista, entonces aparece con aviso de tickets adicionales.
- Dado que el usuario confirma sin seleccionar commits, entonces se bloquea el avance.
- Dado que el usuario confirma una seleccion valida, entonces se genera un `DeploymentPlan` preliminar.

### Informacion Tecnica

Modelo sugerido:

```go
type CommitSelectionItem struct {
    Commit    Commit
    Selected  bool
    Disabled  bool
    Reason    string
}
```

### Mockup TUI

Ver `docs/MOCKUPS_TUI.md`, pantalla `Seleccion De Commits`.

### Test E2E

- Harness: repo git temporal, sin org.
- Seed: feature con un merge commit, un commit ya aplicado en destino (por SHA o equivalencia `git cherry`), un commit que menciona varios tickets, y una seleccion cuyo fichero tocado tiene un commit intermedio no seleccionado de otro ticket.
- Asserta: merge y ya-aplicados salen `Disabled` con `Reason`; el commit con varios tickets muestra aviso de tickets adicionales; se emite warning de dependencia por fichero; se bloquea el avance con seleccion vacia; se genera un `DeploymentPlan` preliminar.
- Variantes: reordenar en modo avanzado con warning.
- En CI: si.

## HU-004 - Seleccionar Destino Y Sandbox

Prioridad: Alta  
Tipo: Producto  
Fase: MVP Git

### Historia

Como desarrollador, quiero elegir la rama destino y sandbox asociada para preparar la promocion correcta.

### Objetivo

Alinear desde el inicio el target Git y el target Salesforce.

### Tareas

- Leer ramas destino desde configuracion YAML.
- Leer sandboxes y alias Salesforce desde configuracion YAML.
- Mostrar ramas destino soportadas: `INT`, `UAT`, `Release/*`, `main`, custom.
- Resolver sandbox para ramas `Release/*` mediante patron en configuracion; bloquear con mensaje accionable si una rama destino no tiene sandbox mapeada.
- Mostrar HEAD remoto de la rama destino.
- Mostrar sandbox asociada y test level configurado.
- Permitir rama custom validando que exista local o remota.
- Guardar seleccion en `DeploymentPlan`.

### Criterios De Aceptacion

- Dado que existen ramas configuradas, cuando el usuario selecciona destino, entonces se muestran con su sandbox asociada.
- Dado que la rama destino no existe, cuando el usuario intenta continuar, entonces se bloquea el flujo.
- Dado que la sandbox asociada no esta autenticada, cuando se selecciona el destino, entonces se muestra advertencia antes de validar.
- Dado que el usuario selecciona una rama `Release/*`, entonces la sandbox se resuelve por patron; si no hay mapeo, se bloquea con mensaje accionable.
- Dado que el usuario selecciona `main`, entonces la TUI muestra advertencia de entorno productivo.

### Informacion Tecnica

Config esperada:

```yaml
branches:
  integration: INT
  uat: UAT
  production: main

sandboxes:
  UAT:
    alias: UAT_SANDBOX
    testLevel: RunLocalTests
```

Comando base:

```bash
git rev-parse origin/<target>
```

### Mockup TUI

Ver `docs/MOCKUPS_TUI.md`, pantalla `Seleccion De Destino`.

### Test E2E

- Harness: config YAML fixture + repo git temporal + `sf org list` falso.
- Seed: config con `INT`/`UAT`/`Release/*`/`main`; rama `Release/Julio2026` y `origin/UAT` en un HEAD conocido; una rama destino sin sandbox mapeada.
- Asserta: cada destino resuelve su alias y test level; `Release/*` resuelve por patron; el destino sin mapeo bloquea con mensaje accionable; se muestra el HEAD remoto (`git rev-parse origin/<target>`); `main` marca advertencia productiva; la seleccion (rama, alias, test level) queda en el `DeploymentPlan`.
- Variantes: rama destino inexistente, sandbox no autenticada (aviso).
- En CI: si.

## HU-005 - Crear Rama Temporal De Promocion

Prioridad: Alta  
Tipo: Operativa  
Fase: MVP Git

### Historia

Como desarrollador, quiero crear una rama temporal desde la rama destino para no modificar ramas protegidas directamente.

### Objetivo

Garantizar que la promocion se prepara sobre el estado actual del destino remoto.

### Tareas

- Ejecutar `git fetch origin` antes de crear rama.
- Generar nombre de rama con formato configurable.
- Validar que no se esta actualmente sobre rama protegida para modificarla directamente.
- Validar si la rama temporal ya existe local o remota.
- Ofrecer editar nombre antes de crear.
- Crear rama desde `origin/<target>`.
- Guardar rama creada en el `DeploymentPlan`.

### Criterios De Aceptacion

- Dado un target valido, cuando se crea la rama, entonces parte de `origin/<target>`.
- Dado que la rama temporal ya existe, cuando se intenta crear, entonces se pide accion al usuario.
- Dado que falla el fetch, cuando se crea rama, entonces el flujo se detiene sin cambiar de rama.
- Dado que la rama se crea correctamente, entonces el plan registra el nombre final.

### Informacion Tecnica

Comandos base:

```bash
git fetch origin
git checkout -b deploy/<ticket>-to-<target> origin/<target>
```

Nombre recomendado:

```text
deploy/{{ticket}}-to-{{target}}
```

### Mockup TUI

Ver `docs/MOCKUPS_TUI.md`, pantalla `Preview Del Plan`.

### Test E2E

- Harness: repo git temporal con remoto bare local, sin org.
- Seed: `origin/UAT` con un HEAD inicial; tras el clon local, el remoto avanza `UAT` a un nuevo HEAD que solo un fetch trae; una rama `deploy/<ticket>-to-UAT` preexistente en `origin` para la colision remota.
- Asserta: `git fetch origin` corre antes de crear la rama; la rama `deploy/<ticket>-to-UAT` parte exactamente del HEAD de `origin/UAT` posterior al fetch (no del ref local desactualizado); el plan registra el nombre final.
- Variantes: rama temporal ya existente en local y en remoto (pide accion), usuario sobre rama protegida al iniciar (no se modifica directamente), fallo de fetch (no cambia de rama).
- En CI: si.

## HU-006 - Ejecutar Cherry-Pick Controlado

Prioridad: Alta  
Tipo: Operativa  
Fase: MVP Git

### Historia

Como desarrollador, quiero aplicar los commits seleccionados uno a uno para preparar la rama de promocion de forma controlada.

### Objetivo

Aplicar solo los commits seleccionados y detener el flujo ante conflictos sin ocultar el estado real de Git.

### Tareas

- Ejecutar cherry-picks secuencialmente en orden topologico.
- Registrar commit actual en el run local.
- Detener flujo si un cherry-pick falla.
- Detectar archivos en conflicto y clasificarlos: texto, binario, modify/delete.
- Modo espera con deteccion activa: releer el estado del repo periodicamente y actualizar la lista de conflictos en vivo; el usuario resuelve con su herramienta preferida (IDE, otra terminal) sin avisar a la TUI.
- Ofrecer atajos opcionales: abrir `$EDITOR` sobre el fichero conflictivo o lanzar `git mergetool`, cediendo la terminal (`tea.ExecProcess`) y recuperandola al salir.
- Habilitar "continuar" solo cuando la TUI confirme: cero paths sin mergear, resueltos staged (ofrecer stagear si falta) y sin marcadores de conflicto (`<<<<<<<`) en ficheros staged.
- Para conflictos modify/delete, ofrecer eleccion explicita: conservar (`git add`) o borrar (`git rm`).
- Reconciliar acciones externas: detectar si el usuario ejecuto `--continue`/`--abort` fuera de DeployDeck releyendo `CHERRY_PICK_HEAD` y `.git/sequencer`, y resincronizar el estado.
- Permitir salir de la TUI con el conflicto abierto: el run queda en `CherryPickConflict` y es reanudable (HU-013).
- Sugerir habilitar `git rerere`; si rerere auto-resuelve un conflicto, mostrarlo como "auto-resuelto con resolucion previa" y pedir confirmacion.
- Permitir continuar con `git cherry-pick --continue` tras resolucion (ejecucion no interactiva: `GIT_EDITOR=true`).
- Detectar cherry-pick vacio (cambio ya aplicado en destino con otro SHA) y ofrecer `git cherry-pick --skip` con mensaje claro.
- Ofrecer resolucion de conflictos en ficheros binarios (static resources) con `git checkout --theirs/--ours`.
- Permitir abortar con `git cherry-pick --abort`; si el abort ocurre a mitad de secuencia, ofrecer eliminar la rama temporal con picks parciales o conservarla marcada en el run.
- Al completar todos los picks, verificar estado final contra la rama origen (`git diff HEAD <source> -- <ficheros tocados>`); si algun fichero difiere, mostrar warning de promocion parcial antes de generar delta.
- No intentar resolver conflictos automaticamente.

### Criterios De Aceptacion

- Dado un conjunto de commits validos, cuando se ejecuta el cherry-pick, entonces se aplican en orden.
- Dado que un commit genera conflicto, cuando falla el comando, entonces el flujo se detiene y muestra archivos conflictivos clasificados por tipo.
- Dado que el usuario resuelve conflictos por cualquier medio externo, cuando la TUI relee el estado del repo, entonces la lista de conflictos se actualiza sola sin intervencion del usuario.
- Dado que quedan conflictos sin resolver o sin stagear, cuando el usuario intenta continuar, entonces la opcion esta deshabilitada con el detalle de lo pendiente.
- Dado que un fichero staged contiene marcadores de conflicto, cuando se valida antes de continuar, entonces se bloquea el continue y se senala el fichero.
- Dado que todo esta resuelto y staged, cuando el usuario selecciona continuar, entonces DeployDeck ejecuta `git cherry-pick --continue` de forma no interactiva.
- Dado que el usuario ejecuto `--continue` o `--abort` fuera de DeployDeck, cuando la TUI refresca, entonces detecta el estado real y se resincroniza.
- Dado que el usuario cierra la TUI con un conflicto abierto, cuando reabre DeployDeck, entonces se ofrece retomar el run en la pantalla de conflicto con su contexto.
- Dado que el usuario aborta, cuando confirma, entonces DeployDeck ejecuta `git cherry-pick --abort` y marca el run como abortado.
- Dado un abort a mitad de secuencia, cuando el usuario confirma, entonces se le ofrece limpiar la rama temporal con los picks parciales aplicados.
- Dado un cherry-pick vacio, cuando se detecta, entonces se informa que el cambio ya estaba en destino y se ofrece saltarlo con `--skip`.
- Dado que el estado final de un fichero difiere del de la rama origen, cuando terminan los picks, entonces se muestra warning de promocion parcial por fichero antes de generar el delta.
- Dado un cherry-pick fallido, entonces DeployDeck no genera delta ni valida Salesforce.

### Informacion Tecnica

Comandos base:

```bash
git cherry-pick <sha>
git status --porcelain
git diff --name-only --diff-filter=U
git cherry-pick --continue
git cherry-pick --skip
git cherry-pick --abort
git diff HEAD <source> -- <ficheros tocados>
```

Estados recomendados:

```text
CherryPickPending
CherryPickRunning
CherryPickConflict
CherryPickEmpty
CherryPickCompleted
PickVerification
CherryPickAborted
```

### Mockup TUI

Ver `docs/MOCKUPS_TUI.md`, pantalla `Cherry-Pick En Progreso` y `Conflicto De Cherry-Pick`.

### Test E2E

- Harness: repo git temporal, sin org (es el ejemplo de referencia; el helper `newTempRepo` se reutiliza en toda la familia Git).
- Seed: `main` + rama `UAT` + feature con 2 commits (`a.cls`, `b.cls`).
- Asserta: la rama temporal contiene ambos cambios, su contenido == rama origen y el orden es topologico; ante conflicto el flujo se detiene con los ficheros clasificados por tipo (texto/binario/modify-delete); mientras queden paths sin mergear, sin stagear o con marcadores `<<<<<<<` en ficheros staged, la opcion de continuar queda deshabilitada con el detalle de lo pendiente; al resolver y stagear por fuera, la relectura periodica del repo actualiza la lista de conflictos sin accion en la TUI y solo entonces se habilita `git cherry-pick --continue` no interactivo; ante fallo no se genera delta ni se valida.
- Variantes: conflicto (texto, binario, modify/delete), pick vacio -> `--skip`, abort a mitad con limpieza de rama parcial, verificacion post-pick que detecta promocion parcial, reconciliacion tras `--continue`/`--abort` externo.
- En CI: si.

## HU-007 - Generar Delta Package

Prioridad: Alta  
Tipo: Producto  
Fase: Delta Salesforce

### Historia

Como desarrollador, quiero generar automaticamente un delta package para validar solo la metadata modificada.

### Objetivo

Reducir alcance de validaciones Salesforce y evitar packages manuales incompletos.

### Tareas

- Crear modulo `internal/delta`.
- Leer `sourceDirs`, `outputDir`, `ignoreFile` e `ignoreDestructiveFile` desde configuracion.
- Spike inicial: verificar si la version pineada de `sfdx-git-delta` soporta multiples `--source-dir`; si no, iterar por directorio y fusionar packages.
- Generar artefactos bajo `.deploydeck/manifest/` (ignorado por Git segun HU-001) para no ensuciar el working tree.
- Ejecutar `sf sgd source delta` comparando `origin/<target>` contra `HEAD`.
- Validar existencia de `package.xml` generado.
- Validar existencia de destructive changes si aplica.
- Guardar paths generados en `DeploymentPlan`.
- Guardar salida raw para diagnostico.

### Criterios De Aceptacion

- Dado que hay cambios Salesforce, cuando se genera delta, entonces se crea `package.xml`.
- Dado que hay metadata eliminada, cuando se genera delta, entonces se crea `destructiveChanges.xml`.
- Dado que el package queda vacio, entonces se muestra advertencia y se bloquea validacion salvo confirmacion futura.
- Dado que `sfdx-git-delta` falla, entonces se muestra stderr/stdout y no se ejecuta validacion.

### Informacion Tecnica

Comando base:

```bash
sf sgd source delta \
  --from origin/<target> \
  --to HEAD \
  --output-dir .deploydeck/manifest/delta/<ticket>-to-<target> \
  --generate-delta \
  --source-dir <source-dir>
```

Artefactos esperados:

```text
.deploydeck/manifest/delta/<ticket>-to-<target>/package/package.xml
.deploydeck/manifest/delta/<ticket>-to-<target>/destructiveChanges/destructiveChanges.xml
.deploydeck/manifest/delta/<ticket>-to-<target>/destructiveChanges/package.xml
```

### Mockup TUI

Ver `docs/MOCKUPS_TUI.md`, pantalla `Resumen De Package`.

### Test E2E

- Harness: repo git temporal + `sfdx-git-delta` real, sin org.
- Seed: cambios Salesforce en `force-app` entre `origin/UAT` y `HEAD`, incluyendo metadata borrada.
- Asserta: se genera `package.xml` bajo `.deploydeck/manifest/`; con borrados aparece `destructiveChanges.xml`; los paths quedan en el plan y el raw guardado; el working tree sigue limpio (los artefactos solo se escriben bajo `.deploydeck/`, ignorado por Git).
- Variantes: package vacio (aviso y validacion bloqueada salvo confirmacion futura), fallo de sgd (muestra stderr, no valida), spike multi `--source-dir`.
- En CI: si (requiere `sfdx-git-delta` instalado en el runner).

## HU-008 - Resumir Package XML Y Destructive Changes

Prioridad: Alta  
Tipo: Producto  
Fase: Delta Salesforce

### Historia

Como tech lead, quiero ver un resumen claro del package generado para revisar alcance antes de validar.

### Objetivo

Dar visibilidad del contenido del despliegue antes de ejecutar Salesforce validate.

### Tareas

- Parsear `package.xml`.
- Contar miembros por metadata type.
- Parsear `destructiveChanges.xml` si existe.
- Detectar metadata sensible.
- Mostrar archivos fuera de `sourceDirs` configurados si existen en diff.
- Mostrar warning si hay `Profile`, `PermissionSet`, `Flow`, `CustomObject`, `CustomField`.
- Bloquear o pedir confirmacion fuerte si hay destructive changes.

### Criterios De Aceptacion

- Dado un package con varios tipos de metadata, cuando se muestra el resumen, entonces aparece el conteo por tipo.
- Dado un package vacio, cuando se muestra el resumen, entonces se advierte claramente.
- Dado que existen destructive changes, cuando se muestra el resumen, entonces aparecen separados y resaltados.
- Dado que hay metadata sensible, entonces la TUI muestra advertencia antes de continuar.

### Informacion Tecnica

Modelo sugerido:

```go
type PackageSummary struct {
    Types              []MetadataTypeSummary
    DestructiveTypes   []MetadataTypeSummary
    HasDestructive     bool
    SensitiveTypes     []string
    OutsideSourceDirs  []string
    Empty              bool
}
```

Tests recomendados:

- Package normal.
- Package vacio.
- Package con destructive changes.
- Package con metadata sensible.

### Mockup TUI

Ver `docs/MOCKUPS_TUI.md`, pantalla `Resumen De Package`.

### Test E2E

- Harness: fixtures `package.xml` / `destructiveChanges.xml` + fixture con la lista de ficheros cambiados del delta (para el chequeo de `sourceDirs`); unit puro, sin repo ni org.
- Seed: package normal, vacio, con destructive changes, con metadata sensible y con un fichero cambiado fuera de `sourceDirs`.
- Asserta: conteo correcto por tipo en `Types`; `HasDestructive` con sus `DestructiveTypes` contados por separado y no mezclados en `Types`; `SensitiveTypes` detecta `Profile`/`PermissionSet`/`Flow`/`CustomObject`/`CustomField`; `Empty`; `OutsideSourceDirs` lista los ficheros cambiados fuera de `sourceDirs`.
- Variantes: los cuatro packages fixture mas el caso con un fichero cambiado fuera de `sourceDirs`.
- En CI: si.

## HU-009 - Consultar Cola De Despliegues

Prioridad: Media  
Tipo: Producto  
Fase: Cola De Deploys

### Historia

Como desarrollador, quiero ver si hay validaciones o deploys en curso en la sandbox para entender si mi job quedara en cola.

### Objetivo

Dar visibilidad operativa del estado de la sandbox antes y durante una validacion.

### Tareas

- Crear funcion `ListDeployQueue` en `internal/salesforce`.
- Ejecutar query Tooling API contra `DeployRequest`.
- Incluir el campo `CheckOnly` para distinguir validaciones de deploys reales.
- Manejar perfiles sin permisos de Tooling API: avisar y continuar el flujo sin cola (no bloquear).
- Parsear JSON de Salesforce CLI.
- Mostrar jobs `Pending` e `InProgress`.
- Mostrar usuario, estado, fecha, progreso y tiempo transcurrido.
- Identificar posicion aproximada del job propio si existe.

### Criterios De Aceptacion

- Dado que hay jobs en curso, cuando se consulta la cola, entonces se muestran ordenados por fecha de creacion.
- Dado que no hay jobs, cuando se consulta la cola, entonces se muestra estado vacio claro.
- Dado que el job propio esta en cola, entonces se resalta y se muestra posicion aproximada.
- Dado que falla la query, entonces se muestra error accionable sin abortar necesariamente todo el flujo.
- Dado que el perfil no tiene permisos de Tooling API, cuando falla la query por permisos, entonces se avisa y el flujo continua sin la vista de cola.

### Informacion Tecnica

Comando base:

```bash
sf data query \
  --target-org <alias> \
  --use-tooling-api \
  --json \
  --query "SELECT Id, Status, CheckOnly, CreatedDate, StartDate, CompletedDate, CreatedBy.Name, NumberComponentsTotal, NumberComponentsDeployed, NumberComponentErrors, NumberTestsTotal, NumberTestsCompleted, NumberTestErrors FROM DeployRequest WHERE Status IN ('Pending','InProgress') ORDER BY CreatedDate ASC"
```

### Mockup TUI

Ver `docs/MOCKUPS_TUI.md`, pantalla `Cola De Deploys`.

### Test E2E

- Harness: `sf data query` falso (JSON `DeployRequest` enlatado) en CI; alias local para real.
- Seed: JSON con jobs `Pending`/`InProgress` de varios usuarios incluido el propio, con la identidad del usuario actual fijada por el harness (ej. `sf org display` falso o inyeccion directa) para hacer determinista la deteccion del job propio; JSON de error por permisos Tooling API.
- Asserta: se listan ordenados por `CreatedDate`; cada job muestra usuario, estado, progreso (componentes/tests) y tiempo transcurrido; el job propio se resalta con posicion; `CheckOnly` distingue validacion de deploy; sin permisos -> aviso y el flujo continua.
- Variantes: cola vacia (estado vacio claro); fallo de query generico no relacionado con permisos (error accionable, no aborta necesariamente todo el flujo); job propio ausente de la cola (lista el resto sin resaltado).
- En CI: parcial (fake en CI; real contra alias en local).

## HU-010 - Ejecutar Validacion Salesforce Async

Prioridad: Alta  
Tipo: Producto  
Fase: Validacion Salesforce

### Historia

Como desarrollador, quiero validar el package contra una sandbox en modo async para comprobar el despliegue antes del PR o deploy real.

### Objetivo

Ejecutar validaciones Salesforce reproducibles y registrar el `jobId` para seguimiento.

### Tareas

- Crear funcion `ValidateDeploy` en `internal/salesforce`.
- Construir comando con manifest, target org y test level.
- Permitir ajustar el test level por ejecucion, incluyendo `RunSpecifiedTests` con lista de clases (`--tests`); el default viene de configuracion.
- Incluir destructive changes cuando aplique.
- Ejecutar con `--async --json`.
- Parsear `jobId`.
- Guardar run local inmediatamente tras obtener job.
- Mostrar job id y siguiente paso.

### Criterios De Aceptacion

- Dado un package valido, cuando se lanza validacion, entonces se obtiene un `jobId`.
- Dado que el package tiene destructive changes, cuando se lanza validacion, entonces se incluye `--post-destructive-changes`.
- Dado que el usuario elige `RunSpecifiedTests`, cuando se lanza la validacion, entonces se incluye `--tests` con las clases indicadas.
- Dado que Salesforce CLI devuelve error, entonces se muestra mensaje y JSON raw si existe.
- Dado que se obtiene `jobId`, entonces se persiste en `.deploydeck/runs/`.

### Informacion Tecnica

Comando base:

```bash
sf project deploy validate \
  --manifest <package.xml> \
  --target-org <alias> \
  --test-level <test-level> \
  --async \
  --json
```

Con destructive changes:

```bash
sf project deploy validate \
  --manifest <package.xml> \
  --post-destructive-changes <destructiveChanges.xml> \
  --target-org <alias> \
  --test-level <test-level> \
  --async \
  --json
```

### Mockup TUI

Ver `docs/MOCKUPS_TUI.md`, pantalla `Validacion En Vivo`.

### Test E2E

- Harness: `sf project deploy validate` falso devolviendo `jobId`; alias local para real.
- Seed: manifest fixture con y sin destructive changes.
- Asserta: se captura el `jobId`; con destructive incluye `--post-destructive-changes` y sin destructive no lo incluye; con `RunSpecifiedTests` incluye `--tests` con las clases indicadas; el run se persiste de inmediato en `.deploydeck/runs/`.
- Variantes: error de CLI con JSON (muestra mensaje y JSON raw); error de CLI sin JSON parseable (muestra mensaje y salida cruda sin romper el flujo).
- En CI: parcial (fake en CI; e2e real con `DEPLOYDECK_E2E_ORG`).

## HU-011 - Mostrar Progreso En Vivo De Validacion

Prioridad: Alta  
Tipo: Producto  
Fase: Validacion Salesforce

### Historia

Como desarrollador, quiero ver el progreso de una validacion para entender si esta avanzando, fallando o esperando cola.

### Objetivo

Transformar el polling manual de `deploy report` en una pantalla clara y reanudable.

### Tareas

- Crear funcion `ReportDeploy` en `internal/salesforce`.
- Implementar polling configurable.
- Parsear estado general, componentes, tests, fallos y tiempos.
- Mostrar progreso en TUI.
- Guardar cada respuesta raw relevante.
- Detener polling en estados terminales.
- Permitir refrescar manualmente.
- Permitir salir dejando job activo.

### Criterios De Aceptacion

- Dado un `jobId` valido, cuando se abre la pantalla, entonces se consulta `deploy report` periodicamente.
- Dado que el job avanza, entonces la TUI actualiza componentes y tests.
- Dado que hay errores de metadata, entonces se muestran con componente, tipo y mensaje.
- Dado que hay tests fallidos, entonces se muestran nombre de clase, metodo y mensaje.
- Dado que el usuario sale, entonces el job sigue activo y el run queda reanudable.

### Informacion Tecnica

Comando base:

```bash
sf project deploy report \
  --job-id <job-id> \
  --target-org <alias> \
  --json
```

Estados terminales esperados:

```text
Succeeded
SucceededPartial
Failed
Canceled
```

### Mockup TUI

Ver `docs/MOCKUPS_TUI.md`, pantalla `Validacion En Vivo` y `Resultado De Validacion`.

### Test E2E

- Harness: `sf project deploy report` falso devolviendo una secuencia de estados (`InProgress` -> terminal); alias local para real.
- Seed: secuencia de reports con componentes/tests crecientes y un fallo terminal que incluye un error de metadata (componente, tipo, mensaje) y un test fallido (clase, metodo, mensaje).
- Asserta: el polling actualiza componentes y tests y se detiene en estado terminal; los errores de metadata se muestran con componente, tipo y mensaje; los tests fallidos con clase, metodo y mensaje; salir deja el job activo y el run reanudable; cada report raw se guarda.
- Variantes: timeout, reintento de polling, refresco manual, terminal exitoso (Succeeded/SucceededPartial).
- En CI: parcial (fake en CI; real en local).

## HU-012 - Cancelar Validacion Propia

Prioridad: Media  
Tipo: Operativa  
Fase: Cola De Deploys

### Historia

Como desarrollador, quiero cancelar mi propia validacion si detecto que el package es incorrecto o ya no se necesita.

### Objetivo

Permitir liberar la cola de la sandbox sin afectar jobs de otros usuarios.

### Tareas

- Crear funcion `CancelDeploy`.
- Habilitar cancelacion solo para job asociado al run actual.
- Pedir confirmacion antes de cancelar.
- Ejecutar comando de cancelacion.
- Actualizar estado del run.
- Mostrar resultado.

### Criterios De Aceptacion

- Dado un job propio en progreso, cuando el usuario confirma cancelar, entonces DeployDeck ejecuta cancelacion.
- Dado un job ajeno, cuando se muestra en cola, entonces no aparece accion de cancelar.
- Dado que la cancelacion falla, entonces se muestra error y el run no se marca como cancelado.

### Informacion Tecnica

Comando base:

```bash
sf project deploy cancel \
  --job-id <job-id> \
  --target-org <alias>
```

### Mockup TUI

Ver `docs/MOCKUPS_TUI.md`, pantalla `Confirmacion De Cancelacion`.

### Test E2E

- Harness: `sf project deploy cancel` falso; alias local para real.
- Seed: run con `jobId` propio en progreso; cola de la sandbox con al menos un job ajeno (otro `jobId`, otro usuario).
- Asserta: solo se ofrece cancelar el job del run actual; tras confirmar se ejecuta `sf project deploy cancel` con el `jobId` propio y el run pasa a `Canceled`; un job ajeno no muestra accion; un cancel fallido muestra error y no marca el run.
- Variantes: cancelacion fallida; el usuario no confirma (no se ejecuta el cancel).
- En CI: parcial (fake en CI; real contra alias en local).

## HU-013 - Guardar Historico Local Y Reanudar Runs

Prioridad: Alta  
Tipo: Operativa  
Fase: Productizacion

### Historia

Como desarrollador, quiero retomar una validacion anterior si cierro la terminal o necesito revisar resultados despues.

### Objetivo

Persistir contexto suficiente para reanudar, auditar y diagnosticar ejecuciones.

### Tareas

- Crear modulo `internal/runs`.
- Crear directorio `.deploydeck/runs/` si no existe.
- Guardar un JSON por run.
- Guardar logs y salidas raw en subdirectorio por run.
- Implementar listado de runs.
- Implementar reanudacion por `jobId` (fase Salesforce).
- Implementar reanudacion de runs en fase Git: si el run quedo en `CherryPickConflict` o `CherryPickRunning`, detectar `CHERRY_PICK_HEAD`/`.git/sequencer` al arrancar y ofrecer volver a la pantalla de conflicto con su contexto (ticket, pick N de M).
- Actualizar estado del run durante el flujo, incluido el progreso del cherry-pick (commit actual, estado).
- Aplicar politica de retencion configurable (por defecto: ultimos 30 runs o 90 dias) con comando `deploydeck runs prune`.

### Criterios De Aceptacion

- Dado que se lanza una validacion, cuando se obtiene `jobId`, entonces se crea registro local.
- Dado que se cierra la TUI, cuando se abre historial, entonces aparece el run anterior.
- Dado un run con `jobId`, cuando se selecciona reanudar, entonces se consulta `deploy report`.
- Dado un run sin job, entonces se muestra hasta que paso llego el flujo.
- Dado un run en `CherryPickConflict` y un `CHERRY_PICK_HEAD` presente en el repo, cuando arranca DeployDeck, entonces se ofrece retomar directamente la pantalla de conflicto.
- Dado un run en fase Git cuyo estado del repo ya no coincide (el usuario resolvio o aborto por fuera), cuando se retoma, entonces se resincroniza con el estado real antes de continuar.

### Informacion Tecnica

Ruta:

```text
.deploydeck/runs/<run-id>/run.json
.deploydeck/runs/<run-id>/logs/
.deploydeck/runs/<run-id>/raw/
```

Modelo minimo:

```go
type RunRecord struct {
    ID                 string
    Ticket             string
    TargetBranch       string
    TargetOrg          string
    DeployBranch       string
    JobID              string
    Status             string
    CreatedAt          time.Time
    UpdatedAt          time.Time
    Commits            []string
    PickIndex          int
    PickTotal          int
    CurrentCommit      string
    TestLevel          string
    PackageXML         string
    DestructiveChanges string
    PRUrl              string
    SourceRunID        string
}
```

### Mockup TUI

Ver `docs/MOCKUPS_TUI.md`, pantalla `Historial De Runs`.

### Test E2E

- Harness: filesystem temporal (`.deploydeck/runs/`) + repo git temporal + `sf project deploy report` falso, sin org.
- Seed: run guardado en `CherryPickConflict` con `CHERRY_PICK_HEAD` presente; otro run en `CherryPickConflict` pero con el repo ya limpio (resuelto o abortado por fuera, sin `CHERRY_PICK_HEAD`); run con `jobId`; run sin job; mas de `keepLast` runs, alguno mas viejo que `keepDays`, para ejercitar la retencion.
- Asserta: al lanzar validacion se crea el registro; el historial lista los runs; un run con `jobId` reanuda por `deploy report`; un run en conflicto ofrece volver a la pantalla de conflicto con su contexto (ticket, pick N de M); si el estado real cambio (resuelto/abortado por fuera) se resincroniza; se aplica retencion `keepLast`/`keepDays` y `runs prune`.
- Variantes: run sin job muestra hasta que paso llego el flujo.
- En CI: si (sin org).

## HU-014 - Preparar Push Y PR

Prioridad: Media  
Tipo: Producto  
Fase: Productizacion

### Historia

Como desarrollador, quiero hacer push de la rama validada y ver la informacion necesaria para abrir PR.

### Objetivo

Cerrar el flujo de preparacion sin reemplazar el proceso formal de revision.

### Tareas

- Detectar validacion exitosa.
- Mostrar comando de push.
- Ejecutar push si el usuario confirma.
- Mostrar base y compare para PR.
- Generar titulo sugerido.
- Detectar disponibilidad y autenticacion de `gh` (`gh auth status`).
- Si `gh` esta disponible: ofrecer crear el PR con `gh pr create`, mostrando el comando antes y pidiendo confirmacion; nunca crear el PR sin accion explicita del usuario.
- Si `gh` no esta disponible: mostrar la URL de compare derivada del remoto `origin` para abrir el PR en el navegador.
- Registrar en el run la URL del PR creado.

### Criterios De Aceptacion

- Dado que la validacion fue exitosa, cuando termina el flujo, entonces se ofrece push de rama.
- Dado que la validacion fallo, entonces no se ofrece push como accion principal.
- Dado que el usuario confirma push, entonces se ejecuta `git push -u origin <branch>`.
- Dado que el push termina OK, entonces se muestra base, compare y titulo sugerido de PR.
- Dado que `gh` esta instalado y autenticado, cuando el usuario confirma crear PR, entonces se ejecuta `gh pr create` y se muestra la URL resultante.
- Dado que `gh` no esta disponible o no esta autenticado, entonces se muestra la URL de compare y el flujo continua sin error.
- Dado que la creacion del PR falla, entonces se muestra el error y los datos para crearlo manualmente.

### Informacion Tecnica

Comando base:

```bash
git push -u origin <deploy-branch>

gh auth status
gh pr create --base <target-branch> --head <deploy-branch> --title "<ticket> - Promote changes to <target-branch>"
```

Datos de PR:

```text
base: <target-branch>
compare: <deploy-branch>
title: <ticket> - Promote changes to <target-branch>
url compare (fallback sin gh): https://github.com/<org>/<repo>/compare/<target>...<deploy-branch>
```

`gh` es dependencia opcional: chequeo informativo en el doctor (HU-001), nunca bloqueante.

### Mockup TUI

Ver `docs/MOCKUPS_TUI.md`, pantalla `Push Y PR`.

### Test E2E

- Harness: repo git temporal con remoto bare local + `gh` falso, sin org real.
- Seed: rama deploy validada; push contra el remoto bare local; para derivar el compare, `origin` con URL estilo GitHub en SSH (`git@github.com:org/repo.git`) y HTTPS; `gh` en tres estados: disponible y autenticado, disponible pero sin autenticar, y ausente.
- Asserta: push a `origin` de la rama con upstream (`git push -u`); muestra base, compare y titulo (`<ticket> - Promote changes to <target>`); con `gh` autenticado y confirmacion explicita se ejecuta `gh pr create --base <target> --head <deploy>` y se registra la URL en `PRUrl` del run; sin accion explicita del usuario no se crea ningun PR; con `gh` ausente o sin autenticar se muestra la URL de compare `https://github.com/<org>/<repo>/compare/<target>...<deploy>` normalizada tanto desde origin SSH como HTTPS y el flujo continua sin error; un fallo de PR muestra los datos para hacerlo manual.
- Variantes: validacion fallida -> no ofrece push como accion principal.
- En CI: si (`gh` falso; remoto bare local).

### Idea Futura (Opcional): Titulo Y Descripcion De PR Con Modelo Local

Extension opcional de "Generar titulo sugerido": usar un modelo local pequeno
(Gemma 3 1B/4B, Qwen2.5-Coder 1.5B/3B, via Ollama u otro servidor) para redactar
el titulo (conventional-commit) y la descripcion del PR a partir del delta y los
commits del ticket. Principios (coherentes con como DeployDeck trata git/sf/sgd/gh):

- NO bundlear ni instalar el modelo desde el paquete. Es una dependencia externa
  que el dev tiene corriendo; DeployDeck solo apunta a ella.
- Config en `deploydeck.yaml`, bloque nuevo opcional:
  `ai: { endpoint: "http://localhost:11434", model: "gemma3:4b", enabled: true }`.
- Chequeo INFORMATIVO/no bloqueante en el doctor (HU-001): endpoint accesible +
  modelo disponible. Igual que los checks de `gh`/git-hooks/gpgsign.
- Degradado elegante: sin `ai` configurado o inalcanzable -> no se ofrece la
  generacion; el flujo de push/PR sigue igual (mismo patron que el degradado por
  permisos de la cola en HU-009).
- Integracion via HTTP al endpoint (puerto), no shell-out obligatorio, para
  soportar Ollama / LM Studio / llama.cpp / remoto compatible. Cuidar diffs
  grandes (truncar/resumir por el limite de contexto del modelo pequeno).
- Fuera del MVP de HU-014; se implementa solo si se prioriza.

## HU-015 - Quick Deploy Opcional

Prioridad: Baja  
Tipo: Operativa  
Fase: Futuro

### Historia

Como release manager, quiero ver un comando de quick deploy cuando una validacion fue exitosa para reutilizarla si el proceso lo permite.

### Objetivo

Soportar escenarios controlados de release sin convertir DeployDeck en herramienta de deploy productivo por defecto.

### Tareas

- Detectar validaciones exitosas elegibles: con tests requeridos ejecutados y menos de 10 dias de antiguedad (limite de Salesforce para quick deploy).
- Mostrar comando sugerido, no ejecutarlo por defecto.
- Requerir confirmacion fuerte si se habilita ejecucion futura.
- Bloquear quick deploy a produccion salvo configuracion explicita.
- Registrar accion en historial.

### Criterios De Aceptacion

- Dado que una validacion fue exitosa, cuando se consulta el resultado, entonces se puede mostrar comando de quick deploy.
- Dado que la validacion tiene mas de 10 dias o no corrio los tests requeridos, entonces no se muestra como elegible.
- Dado que el entorno es produccion, entonces quick deploy no se ejecuta sin configuracion explicita.
- Dado que el usuario no confirma, entonces no se ejecuta ningun deploy.

### Informacion Tecnica

Comando base:

```bash
sf project deploy quick \
  --job-id <job-id> \
  --target-org <alias>
```

### Mockup TUI

Ver `docs/MOCKUPS_TUI.md`, pantalla `Quick Deploy Opcional`.

### Test E2E

- Harness: runs fixture (`.deploydeck/runs/`) con validaciones exitosas + `sf project deploy quick` falso; alias local para real.
- Seed: validacion exitosa elegible (< 10 dias, tests requeridos ejecutados), una no elegible por antiguedad (> 10 dias) y una no elegible por test level (exitosa pero sin ejecutar los tests requeridos).
- Asserta: la elegible muestra el comando con su `--job-id`; ninguna no elegible (por antiguedad o por test level) lo ofrece; produccion no ejecuta sin config explicita; sin confirmacion fuerte no despliega; con ejecucion habilitada y confirmacion fuerte sobre un destino permitido se ejecuta `sf project deploy quick` con el `--job-id` correcto y la accion queda registrada en el run.
- Variantes: elegibilidad por antiguedad y por test level.
- En CI: parcial (fake en CI; real via alias en local; fase Futuro).

## HU-016 - Re-Promocionar Ticket Entre Ambientes

Prioridad: Alta  
Tipo: Producto  
Fase: Productizacion

### Historia

Como desarrollador, quiero reutilizar los commits de una promocion anterior del mismo ticket para llevarlos al siguiente ambiente sin re-seleccionar a mano.

### Objetivo

Garantizar consistencia entre ambientes: el mismo ticket viaja por `INT -> UAT -> Release -> main` tres o cuatro veces, y cada re-seleccion manual es una oportunidad de olvidar un commit. El historico deja de ser un log pasivo y se convierte en la fuente de la siguiente promocion.

### Tareas

- Al iniciar una promocion, buscar runs previos del mismo ticket hacia el ambiente anterior.
- Si existe un run con validacion exitosa, ofrecer "promocionar los mismos N commits" precargando la seleccion.
- Mapear los SHAs del run previo a sus equivalentes en el nuevo origen (patch-id) cuando difieran.
- Permitir ajustar la seleccion precargada antes de continuar.
- Registrar en el nuevo run la referencia al run origen.

### Criterios De Aceptacion

- Dado un run previo exitoso del mismo ticket, cuando se inicia una promocion al siguiente ambiente, entonces se ofrece reutilizar sus commits.
- Dado que el usuario acepta, entonces la seleccion queda precargada con los commits equivalentes y editable.
- Dado que un commit del run previo no se encuentra en el nuevo origen, entonces se avisa explicitamente en lugar de omitirlo en silencio.
- Dado que se completa la promocion, entonces el run registra de que run proviene.

### Mockup TUI

Ver `docs/MOCKUPS_TUI.md`, pantalla `Historial De Runs`.

### Test E2E

- Harness: repo git temporal + runs fixture, sin org.
- Seed: run previo exitoso del ticket hacia el ambiente anterior; un run previo del mismo ticket con validacion fallida (no elegible); nuevo origen con SHAs equivalentes (distinto SHA, mismo patch-id) y un commit ausente.
- Asserta: se ofrece reutilizar los N commits precargados y editables; se mapean los SHAs por patch-id; el commit ausente se avisa explicitamente; el nuevo run registra `SourceRunID`.
- Variantes: seleccion precargada editada antes de continuar; run previo con validacion fallida no ofrece precarga; ticket sin run previo cae al flujo manual sin precarga.
- En CI: si.

## HU-017 - Limpieza De Ramas Y Runs

Prioridad: Media  
Tipo: Operativa  
Fase: Productizacion

### Historia

Como desarrollador, quiero que DeployDeck limpie las ramas temporales y los runs antiguos para que el repo no acumule ramas muertas.

### Objetivo

Evitar la acumulacion de ramas `deploy/*` obsoletas y devolver siempre al usuario a un estado conocido del repo.

### Tareas

- Al finalizar o abandonar un flujo, restaurar la rama en la que estaba el usuario al empezar.
- Ofrecer borrar la rama temporal local (y remota si se hizo push) tras merge del PR o abandono.
- Detectar ramas `deploy/*` huerfanas de runs anteriores y ofrecer limpiarlas en lote.
- Aplicar la politica de retencion de runs (`runs.keepLast` / `runs.keepDays`).
- Nunca borrar ramas con trabajo sin push sin confirmacion explicita.

### Criterios De Aceptacion

- Dado que el flujo termina o se aborta, entonces el usuario vuelve a su rama original.
- Dado un PR mergeado o un flujo abandonado, cuando el usuario confirma, entonces se borra la rama temporal.
- Dado que existen ramas `deploy/*` huerfanas, cuando se abre la limpieza, entonces se listan con su antiguedad y estado de push.
- Dado que una rama tiene commits sin push, entonces no se borra sin confirmacion fuerte.

### Test E2E

- Harness: repo git temporal con remoto bare local + `.deploydeck/runs/`, sin org.
- Seed: rama original del usuario; ramas `deploy/*` huerfanas con y sin push; runs antiguos fuera de retencion.
- Asserta: al terminar/abortar se restaura la rama original; un PR mergeado o abandono borra la rama temporal tras confirmar, y si la rama se habia pusheado tambien se elimina su ref remota en el bare local; las huerfanas se listan con antiguedad y estado de push; una rama con commits sin push no se borra sin confirmacion fuerte; los runs fuera de `keepLast`/`keepDays` se eliminan y los recientes se conservan.
- Variantes: abort a mitad de secuencia.
- En CI: si.

## HU-018 - Modos Standalone: Delta Y Validacion Sueltas

Prioridad: Media  
Tipo: Producto  
Fase: Futuro

### Historia

Como desarrollador, quiero generar un delta o validar un package existente sin pasar por el flujo completo de promocion, para usar DeployDeck con ramas preparadas a mano.

### Objetivo

Dar soporte real a las entradas "Generar delta package" y "Validar package contra sandbox" del menu principal, que hoy no tienen flujo definido.

### Tareas

- Modo delta standalone: elegir rama base y ref actual, generar delta y mostrar resumen (reutiliza HU-007/HU-008).
- Modo validacion standalone: elegir un `package.xml` existente y sandbox, lanzar validacion (reutiliza HU-010/HU-011).
- Crear run local tambien en estos modos.
- Hasta que se implementen, ocultar ambas entradas del menu principal.

### Criterios De Aceptacion

- Dado un repo con una rama preparada a mano, cuando se usa el modo delta, entonces se genera y resume el package sin cherry-picks.
- Dado un `package.xml` existente, cuando se usa el modo validacion, entonces se lanza y monitoriza igual que en el flujo completo.
- Dado que los modos no estan implementados, entonces no aparecen en el menu.

### Test E2E

- Harness: repo git temporal (+ `sfdx-git-delta`) para el modo delta; `sf` falso / alias local para el modo validacion.
- Seed: rama preparada a mano; `package.xml` existente.
- Asserta: el modo delta genera `package.xml` bajo `.deploydeck/manifest/` y su resumen por tipo sin cherry-picks; el modo validacion captura el `jobId` y hace polling hasta estado terminal igual que el flujo completo; ambos crean run local en `.deploydeck/runs/`; las entradas del menu aparecen y quedan operativas al estar los modos implementados (ocultas mientras no lo esten).
- Variantes: modo validacion con `package.xml` inexistente o invalido (error accionable, no lanza validacion); modo delta con delta vacio (reutiliza el aviso de HU-007/HU-008).
- En CI: parcial (delta en CI; validacion con fake/alias).

## HU-019 - Pipeline De Release De DeployDeck

Prioridad: Media  
Tipo: Tecnica  
Fase: Productizacion

### Historia

Como equipo, queremos un pipeline de CI/release para DeployDeck que compile, teste y publique binarios versionados, para distribuir la herramienta sin builds manuales.

### Objetivo

Cumplir "binarios distribuibles" de la Fase 5 con un proceso repetible: cada release etiquetada produce binarios para macOS (y preparados para Linux/Windows).

### Tareas

- CI en cada PR: build, `go vet`, tests.
- Release con `goreleaser` al etiquetar version semver: compilar binarios para macOS (amd64 + arm64), Linux y Windows.
- Publicar binarios y checksums en GitHub Releases.
- Distribucion via package manager (decision tomada): Homebrew tap propio para macOS/Linux (`brew install <org>/tap/deploydeck`) y Scoop bucket para Windows (`scoop install deploydeck`), ambos actualizados automaticamente por goreleaser en cada release.
- Fallback universal para devs con Go: `go install <module>/cmd/deploydeck@latest`.
- Descartados en el MVP: winget y chocolatey (mas friccion; reevaluar solo si el proyecto se hace publico).
- Incluir version embebida en el binario (`deploydeck --version`).
- Aviso de nueva version disponible al arrancar (chequeo no bloqueante).
- Documentar instalacion y actualizacion en el README.

### Criterios De Aceptacion

- Dado un PR, cuando se abre, entonces CI ejecuta build, `go vet` y tests.
- Dado un tag semver, cuando se publica, entonces se generan binarios adjuntos a la release y se actualizan la formula Homebrew y el manifiesto Scoop.
- Dado un usuario macOS/Linux, cuando ejecuta `brew install`, entonces obtiene el binario; dado un usuario Windows, cuando ejecuta `scoop install`, entonces obtiene el binario.
- Dado un binario instalado, cuando se ejecuta `--version`, entonces muestra la version de la release.
- Dado que existe una version mas nueva, cuando arranca la TUI, entonces se avisa sin bloquear.

Nota (decision pendiente): si el repo de DeployDeck es privado, las releases tambien lo son y `brew`/`scoop` requieren un token de GitHub por usuario; si es publico, la instalacion es sin friccion. La eleccion publico/privado condiciona la UX de instalacion.

Infraestructura necesaria (una vez): dos repos auxiliares, `homebrew-tap` y `scoop-bucket`, mas un token con permiso de escritura sobre ellos expuesto al workflow de release.

### Test E2E

- Harness: el propio CI (GitHub Actions) + `goreleaser --snapshot` en dry-run + endpoint de GitHub Releases falso (JSON enlatado) para el chequeo de nueva version.
- Seed: tag semver de prueba.
- Asserta: en cada PR corre build + `go vet` + tests; `goreleaser check` valida la config y `goreleaser --snapshot` produce binarios para macOS/Linux/Windows y genera la formula Homebrew y el manifiesto Scoop en `dist/` (en snapshot no publica al tap/bucket); `deploydeck --version` coincide con la version inyectada por ldflags; con una version enlatada mas nueva, al arrancar la TUI se muestra el aviso.
- Variantes: endpoint de releases caido o lento -> el arranque no se bloquea (chequeo con timeout y error ignorado).
- En CI: si (es CI en si mismo; la release real solo se dispara al taggear).

## Orden De Implementacion Recomendado

1. HU-001 - Validar prerequisitos locales.
2. HU-002 - Buscar commits por ticket.
3. HU-003 - Seleccionar commits manualmente.
4. HU-004 - Seleccionar destino y sandbox.
5. HU-005 - Crear rama temporal.
6. HU-006 - Ejecutar cherry-pick controlado (incluye verificacion post-pick).
7. HU-007 - Generar delta package.
8. HU-008 - Resumir package XML y destructive changes.
9. HU-010 - Ejecutar validacion Salesforce async.
10. HU-011 - Mostrar progreso en vivo.
11. HU-013 - Guardar historico y reanudar runs.
12. HU-016 - Re-promocionar ticket entre ambientes.
13. HU-009 - Consultar cola de despliegues.
14. HU-012 - Cancelar validacion propia.
15. HU-014 - Preparar push y PR.
16. HU-017 - Limpieza de ramas y runs.
17. HU-019 - Pipeline de release de DeployDeck.
18. HU-018 - Modos standalone (futuro).
19. HU-015 - Quick deploy opcional (futuro).

## Estimacion Orientativa

Tallas: S = 1-2 dias, M = 3-5 dias, L = 1-2 semanas (una persona, sin pulir UI mas alla del mockup). Pendiente de refinar en equipo.

| HU | Titulo | Fase | Talla |
| --- | --- | --- | --- |
| HU-001 | Prerequisitos + doctor | MVP Git | M |
| HU-002 | Buscar commits (equivalencia, topo-order) | MVP Git | L |
| HU-003 | Seleccion con dependencias | MVP Git | M |
| HU-004 | Destino y sandbox | MVP Git | S |
| HU-005 | Rama temporal | MVP Git | S |
| HU-006 | Cherry-pick controlado + verificacion | MVP Git | L |
| HU-007 | Delta package (+ spike multi source-dir) | Delta | M |
| HU-008 | Resumen package | Delta | M |
| HU-010 | Validacion async | Validacion | M |
| HU-011 | Progreso en vivo | Validacion | L |
| HU-013 | Historico y reanudacion | Productizacion | M |
| HU-016 | Re-promocion entre ambientes | Productizacion | M |
| HU-009 | Cola de deploys | Cola | M |
| HU-012 | Cancelar job propio | Cola | S |
| HU-014 | Push y PR | Productizacion | S |
| HU-017 | Limpieza de ramas y runs | Productizacion | S |
| HU-019 | Pipeline de release | Productizacion | M |
| HU-018 | Modos standalone | Futuro | M |
| HU-015 | Quick deploy | Futuro | S |

Total orientativo del alcance de la epica (sin las dos historias de fase Futuro): 10 a 14 semanas de una persona.

## Definicion De Ready Para Cada Historia

- Tiene objetivo claro.
- Tiene criterios de aceptacion verificables.
- Tiene dependencias identificadas.
- Tiene comandos externos definidos si aplica.
- Tiene mockup si impacta la TUI.
- Tiene impacto en datos persistidos identificado si aplica.

## Definicion De Done Para Cada Historia

- Codigo implementado.
- Tests unitarios donde aplique.
- Test e2e de la HU en verde, con el harness definido en su subseccion `Test E2E` (repo git temporal, `sf`/`gh` falso o alias local segun aplique).
- Errores externos manejados.
- Logs o raw output guardados si aplica.
- Pantalla TUI implementada si corresponde.
- Documentacion actualizada.
- Validacion manual ejecutada en escenario representativo.

## Indice De Tests E2E

Regla del proyecto: cada HU nace con su test (ver la subseccion `Test E2E` de cada historia). Leyenda de harness: `temp` = repo git temporal; `sgd` = + `sfdx-git-delta` real; `sf-fake` = `exec` falso con JSON enlatado; `alias` = org real en local via `DEPLOYDECK_E2E_ORG`; `fs` = fixtures de filesystem; `gh-fake` = `gh` falso.

| HU | Harness | Verifica (resumen) | CI |
| --- | --- | --- | --- |
| HU-001 | temp + sf-fake | prereqs y exit code del doctor | si |
| HU-002 | temp | commits por ticket, equivalencia, topo-order | si |
| HU-003 | temp | dependencias por fichero y bloqueos de seleccion | si |
| HU-004 | fs + temp + sf-fake | resolucion de destino y sandbox | si |
| HU-005 | temp | rama temporal parte de origin/target | si |
| HU-006 | temp | picks == origen, conflicto, vacio, abort | si |
| HU-007 | temp + sgd | package.xml y destructiveChanges correctos | si |
| HU-008 | fs | conteo por tipo y metadata sensible | si |
| HU-009 | sf-fake / alias | cola ordenada, job propio, permisos | parcial |
| HU-010 | sf-fake / alias | jobId, destructive, tests, run persistido | parcial |
| HU-011 | sf-fake / alias | polling a terminal, errores, reanudable | parcial |
| HU-012 | sf-fake / alias | cancelar solo el job propio | parcial |
| HU-013 | fs + temp + sf-fake | historico y reanudacion (git y job) | si |
| HU-014 | temp + gh-fake | push y PR con y sin gh | si |
| HU-015 | fs + sf-fake / alias | elegibilidad de quick deploy (futuro) | parcial |
| HU-016 | temp + fs | re-promocion por patch-id y SourceRunID | si |
| HU-017 | temp + fs | restaurar rama, limpiar huerfanas, retencion | si |
| HU-018 | temp + sgd / sf-fake / alias | modos delta y validacion standalone (futuro) | parcial |
| HU-019 | CI + goreleaser | build/test en PR y snapshot de release | si |
