# Epica: DeployDeck - Asistente TUI Para Despliegues Salesforce

## Resumen Ejecutivo

DeployDeck sera una herramienta TUI compartida para el equipo de desarrollo Salesforce que permita preparar, validar y promocionar cambios entre ramas y sandboxes sin depender de GitHub Actions.

El objetivo es convertir el proceso manual actual de cherry-picks, creacion de ramas, generacion de delta packages y validaciones contra sandbox en un flujo guiado, repetible y seguro.

## Problema

Actualmente los despliegues Salesforce se apoyan en operaciones manuales:

- Buscar commits por ticket.
- Crear ramas `-INT`, `-UAT` o temporales.
- Hacer cherry-pick manual.
- Generar `package.xml` manualmente o con herramientas externas.
- Validar contra sandboxes desde CLI.
- Revisar errores de deploy y tests manualmente.
- Saber si otra persona tiene una validacion o deploy en curso.

Esto genera riesgos:

- Commits incorrectos o incompletos.
- Diferencias entre lo validado y lo que se sube a PR.
- Paquetes delta incompletos.
- Destructive changes no revisados.
- Validaciones bloqueadas por trabajos en cola sin visibilidad.
- Dificultad para retomar una validacion si se cierra la terminal.
- Variabilidad en el proceso entre desarrolladores.

## Objetivo De La Epica

Construir DeployDeck, una TUI en Go que ayude a todos los devs a:

- Seleccionar commits por ticket, rama o manualmente.
- Crear ramas temporales de despliegue desde ramas destino.
- Aplicar cherry-picks de forma ordenada y controlada.
- Generar delta packages con `sfdx-git-delta`.
- Revisar `package.xml` y `destructiveChanges.xml`.
- Ejecutar validaciones Salesforce contra sandboxes.
- Mostrar progreso en vivo de validaciones.
- Mostrar cola de despliegues de una sandbox.
- Guardar historico local de ejecuciones.
- Preparar push, PR y quick deploy cuando aplique.

## Fuera De Alcance Inicial

- Reemplazar GitHub o el proceso formal de PR.
- Ejecutar deploy productivo por defecto.
- Resolver conflictos Git automaticamente.
- Modificar metadata Salesforce inteligentemente.
- Sustituir una futura plataforma CI/CD completa.
- Gestionar permisos de Salesforce o autenticaciones OAuth centralizadas.

## Usuarios Objetivo

- Desarrollador Salesforce que necesita promocionar un ticket a `INT` o `UAT`.
- Tech Lead que necesita revisar que commits y metadata se van a validar.
- Release Manager que necesita preparar una rama de release y validar contra sandbox.
- Equipo de soporte que necesita aplicar un fix/hotfix con trazabilidad.

## Flujo Funcional Principal

```text
1. Abrir DeployDeck desde el repo Salesforce
2. Validar prerequisitos locales
3. Validar que el working tree esta limpio
4. Introducir ticket, patron o rama origen
5. Detectar commits candidatos
6. Seleccionar commits
7. Seleccionar rama destino
8. Seleccionar sandbox destino
9. Crear rama temporal desde rama destino
10. Aplicar cherry-pick de commits
11. Generar delta package
12. Revisar package.xml y destructive changes
13. Consultar cola de despliegues en sandbox
14. Lanzar validacion async
15. Mostrar progreso en vivo
16. Si la validacion falla, permitir ajustar seleccion o revalidar en la misma rama
17. Guardar reporte local
18. Ofrecer push, PR o quick deploy
19. Limpiar rama temporal y restaurar rama original al finalizar o abandonar
```

## Modelo De Branching Soportado

DeployDeck debe funcionar con el modelo actual del repo:

```text
feature/fix/epic/userstory
    -> INT
        -> UAT
            -> Release/MesYYYY
                -> main
```

La herramienta no debe obligar a cambiar el flujo, pero si debe promover buenas practicas:

- Crear ramas temporales tipo `deploy/{{ticket}}-to-{{target}}`.
- Evitar commits directos sobre `INT`, `UAT`, `Release/*` o `main`.
- Validar antes de pedir PR o despliegue real.
- Registrar que se intento mover, hacia donde y con que resultado.
- Recomendar al equipo que los PR de promocion se mergeen con merge commit, no squash, para preservar la equivalencia de commits entre ambientes (ver DEC-001 en `docs/ARQUITECTURA.md`). El merge del PR ocurre en GitHub, fuera de DeployDeck: la herramienta debe funcionar con cualquier politica y avisar si detecta squash.

## Requisitos Funcionales

### RF-001 - Validacion De Prerequisitos

Al iniciar, DeployDeck debe validar:

- Que se ejecuta dentro de un repo Git.
- Que existen `git`, `sf` y el plugin `sfdx-git-delta`.
- Que el repo tiene remoto `origin`.
- Que las ramas configuradas existen local o remotamente.
- Que los alias Salesforce configurados existen.
- Que el working tree esta limpio antes de modificar ramas.
- Que las versiones cumplen los minimos configurados (`minVersions` en config).
- Que `gh` esta disponible y autenticado (informativo, no bloqueante: solo habilita crear PR desde la TUI).
- Que `.deploydeck/` esta incluido en el `.gitignore` del repo (bloqueante: ahi se generan manifests y runs).
- Que no existe otra instancia activa de DeployDeck sobre el repo (lock file).

Comandos base:

```bash
git status --short --branch
git remote -v
sf --version
sf plugins
sf org list --json
```

### RF-002 - Busqueda De Commits Por Ticket

El usuario podra introducir un ticket como `OTACUPYR-1386` o una incidencia como `INC000001170030`.

La herramienta buscara:

- Commits con el ticket en el mensaje.
- Ramas que contengan el ticket.
- Commits presentes en ramas candidatas y no presentes en destino.

Comandos base:

```bash
git log --all --grep OTACUPYR-1386 --oneline --decorate
git branch -r --list '*OTACUPYR-1386*'
git log origin/UAT..origin/feature/OTACUPYR-1386 --oneline --topo-order
git cherry origin/UAT origin/feature/OTACUPYR-1386
```

Reglas:

- La deteccion de "ya presente en destino" debe hacerse por equivalencia de contenido (`git cherry` / patch-id), no solo por SHA: en este flujo los commits llegan a los ambientes via cherry-pick con SHA distinto.
- Solo se permite una rama origen por ejecucion; no se mezclan commits de ramas distintas.
- Para promociones entre ambientes (ej. `INT -> UAT`), la fuente sugerida por defecto es el ambiente anterior, no la rama feature: es lo que se valido en sandbox.
- Si la rama origen ya no existe, la busqueda depende solo del grep de mensajes; los commits sin ticket en el mensaje seran invisibles y debe avisarse.
- Si el repo usa squash merges, la equivalencia por contenido no es fiable; se avisa y se confia en la deteccion de cherry-pick vacio (RF-006).

### RF-003 - Seleccion Manual De Commits

La TUI debe mostrar una lista seleccionable con:

- SHA corto.
- Mensaje.
- Autor.
- Fecha.
- Rama asociada si se puede inferir.
- Indicador de merge commit.
- Indicador de commit ya presente en destino.

Reglas:

- Los merge commits estan bloqueados: no son seleccionables en el MVP (su cherry-pick requiere `-m` y queda fuera de alcance).
- Los commits ya presentes en destino, por SHA o por equivalencia de contenido, deben aparecer bloqueados o avisados.
- El orden final de cherry-pick debe ser topologico (`git rev-list --reverse --topo-order`), no cronologico: las fechas de autor no sobreviven fiablemente a rebases y amends.
- Si un fichero tocado por los commits seleccionados tiene commits intermedios no seleccionados en `origin/<target>..<source>`, se muestra warning de dependencia por fichero (riesgo de promocion parcial).
- Si un commit menciona otros tickets ademas del buscado, se muestra aviso.

### RF-004 - Seleccion De Rama Destino

La TUI debe ofrecer ramas destino configuradas:

- `INT`
- `UAT`
- `Release/*`
- `main`
- Rama custom

Debe mostrar el commit HEAD remoto de la rama destino antes de continuar.

Cada rama destino debe resolver una sandbox. Las ramas `Release/*` resuelven por patron en configuracion; si una rama destino no tiene sandbox mapeada, se bloquea la validacion con mensaje accionable.

### RF-005 - Creacion De Rama Temporal

La herramienta creara una rama temporal desde la rama destino.

Ejemplo:

```bash
git fetch origin
git checkout -b deploy/OTACUPYR-1386-to-UAT origin/UAT
```

Formato recomendado:

```text
deploy/{{ticket}}-to-{{target}}
```

### RF-006 - Cherry-Pick Controlado

DeployDeck ejecutara cherry-picks uno a uno.

Ejemplo:

```bash
git cherry-pick 0f95dd3e
git cherry-pick d0927ef
```

Si hay conflicto, la TUI entra en modo espera con deteccion activa:

- Detener el flujo y mostrar los archivos en conflicto.
- El usuario resuelve con la herramienta que prefiera (IDE, otra terminal, mergetool); la TUI no obliga a resolver dentro de ella.
- La TUI relee el estado real del repo periodicamente (`git diff --name-only --diff-filter=U`, `git status --porcelain`) y actualiza la lista en vivo: cada fichero resuelto y staged desaparece solo, sin que el usuario avise.
- Atajos opcionales desde la pantalla: abrir `$EDITOR` sobre el fichero conflictivo o lanzar `git mergetool`, cediendo la terminal y recuperandola al salir.
- "Continuar" solo se habilita cuando la propia TUI confirma que no quedan paths sin mergear, que lo resuelto esta staged (ofrecer stagear si falta) y que ningun fichero staged contiene marcadores `<<<<<<<` olvidados.
- Conflictos modify/delete: no hay texto que editar; ofrecer explicitamente conservar (`git add`) o borrar (`git rm`).
- Reconciliar acciones externas: si el usuario ejecuto `--continue` o `--abort` por su cuenta en otra terminal, la TUI lo detecta al releer `CHERRY_PICK_HEAD` y `.git/sequencer` y se resincroniza en lugar de asumir su propio estado.
- Permitir salir de la TUI dejando el conflicto abierto: el run queda en estado `CherryPickConflict` y se retoma al reabrir.
- Permitir abortar cherry-pick.
- Sugerir `git rerere` para que las resoluciones se reapliquen automaticamente al re-promocionar el mismo ticket al siguiente ambiente.

Comandos base:

```bash
git status --porcelain
git diff --name-only --diff-filter=U
git cherry-pick --continue
git cherry-pick --skip
git cherry-pick --abort
```

Si el cherry-pick queda vacio (el cambio ya existe en destino con otro SHA):

- Detectar el estado "the previous cherry-pick is now empty".
- Informar que el cambio ya estaba en destino y ofrecer `git cherry-pick --skip`.

Si el usuario aborta a mitad de secuencia:

- `git cherry-pick --abort` solo deshace el pick en curso; los picks anteriores quedan en la rama temporal.
- Ofrecer eliminar la rama temporal con picks parciales o conservarla marcada en el run.

Si el conflicto es en ficheros binarios (static resources):

- No se resuelven editando; ofrecer `git checkout --theirs/--ours <fichero>`.

Al terminar todos los picks, verificar el estado final contra la rama origen:

```bash
git diff HEAD <source> -- <ficheros tocados>
```

Si el contenido final de un fichero difiere del de la rama origen, mostrar warning de promocion parcial por fichero antes de generar el delta. Este es el mecanismo contra el caso mas peligroso: un pick que aplica limpio pero produce un estado que no existe en ninguna rama y nadie ha probado.

### RF-007 - Generacion De Delta Package

Despues del cherry-pick, DeployDeck generara un delta package comparando la rama destino contra `HEAD`.

Los artefactos se generan bajo `.deploydeck/`, que debe estar en `.gitignore` (validado en RF-001): de lo contrario el propio delta ensuciaria el working tree que la herramienta exige limpio.

Ejemplo:

```bash
sf sgd source delta \
  --from origin/UAT \
  --to HEAD \
  --output-dir .deploydeck/manifest/delta/OTACUPYR-1386-to-UAT \
  --generate-delta \
  --source-dir up_saln0001_giss_salesforce/force-app
```

Artefactos esperados:

```text
.deploydeck/manifest/delta/OTACUPYR-1386-to-UAT/package/package.xml
.deploydeck/manifest/delta/OTACUPYR-1386-to-UAT/destructiveChanges/destructiveChanges.xml
.deploydeck/manifest/delta/OTACUPYR-1386-to-UAT/destructiveChanges/package.xml
```

### RF-008 - Resumen Del Package

La TUI debe leer y resumir el `package.xml`:

```text
ApexClass:                  3
LightningComponentBundle:   2
CustomField:                4
Profile:                    1
Layout:                     1
Deleted metadata:           0
```

Debe detectar:

- Package vacio.
- Destructive changes.
- Metadata sensible como `Profile`, `PermissionSet`, `Flow`, `CustomObject`, `CustomField`.
- Ficheros que no pertenecen a carpetas Salesforce configuradas.

### RF-009 - Validacion Contra Sandbox

La TUI debe ejecutar validaciones, no deploy real por defecto.

El test level por defecto viene de configuracion, pero debe poder ajustarse por ejecucion, incluyendo `RunSpecifiedTests` con lista de clases (`--tests`) para acortar validaciones largas.

Comando base:

```bash
sf project deploy validate \
  --manifest .deploydeck/manifest/delta/OTACUPYR-1386-to-UAT/package/package.xml \
  --target-org UAT_SANDBOX \
  --test-level RunLocalTests \
  --async \
  --json
```

Con destructive changes:

```bash
sf project deploy validate \
  --manifest .deploydeck/manifest/delta/OTACUPYR-1386-to-UAT/package/package.xml \
  --post-destructive-changes .deploydeck/manifest/delta/OTACUPYR-1386-to-UAT/destructiveChanges/destructiveChanges.xml \
  --target-org UAT_SANDBOX \
  --test-level RunLocalTests \
  --async \
  --json
```

### RF-010 - Progreso En Vivo

La herramienta debe lanzar la validacion async, capturar el `jobId` y consultar estado periodicamente.

Polling:

```bash
sf project deploy report \
  --job-id 0AfXXXXXXXXXXXX \
  --target-org UAT_SANDBOX \
  --json
```

Debe mostrar:

- Estado.
- Componentes desplegados / total.
- Tests completados / total.
- Errores de metadata.
- Tests fallidos.
- Tiempo transcurrido.
- Job Id.

### RF-011 - Cola De Despliegues

Antes y durante una validacion, DeployDeck debe mostrar si hay deploys o validaciones en curso en la sandbox.

Query Tooling API:

```bash
sf data query \
  --target-org UAT_SANDBOX \
  --use-tooling-api \
  --json \
  --query "SELECT Id, Status, CheckOnly, CreatedDate, StartDate, CompletedDate, CreatedBy.Name, NumberComponentsTotal, NumberComponentsDeployed, NumberComponentErrors, NumberTestsTotal, NumberTestsCompleted, NumberTestErrors FROM DeployRequest WHERE Status IN ('Pending','InProgress') ORDER BY CreatedDate ASC"
```

La query requiere permisos de Tooling API. Si el perfil del usuario no los tiene, se muestra aviso y el flujo continua sin cola (no bloquear). `CheckOnly` distingue validaciones de deploys reales.

Debe mostrar:

- Jobs en curso.
- Jobs pendientes.
- Usuario que los lanzo.
- Estado.
- Tiempo transcurrido.
- Progreso conocido.
- Posicion aproximada del job propio.

### RF-012 - Cancelacion De Job Propio

La herramienta debe permitir cancelar la validacion propia.

```bash
sf project deploy cancel \
  --job-id 0AfXXXXXXXXXXXX \
  --target-org UAT_SANDBOX
```

No debe cancelar jobs ajenos salvo modo admin futuro.

### RF-013 - Guardado De Historico

DeployDeck debe guardar ejecuciones localmente para retomar validaciones.

Directorio:

```text
.deploydeck/runs/
```

Retencion: conservar por defecto los ultimos 30 runs o 90 dias (configurable); comando `deploydeck runs prune` para purgar. Los runs de un mismo ticket deben poder reutilizarse al promocionar al siguiente ambiente (ver HU-016 en `docs/HISTORIAS.md`).

Ejemplo de registro:

```json
{
  "ticket": "OTACUPYR-1386",
  "targetBranch": "UAT",
  "targetOrg": "UAT_SANDBOX",
  "deployBranch": "deploy/OTACUPYR-1386-to-UAT",
  "jobId": "0AfXXXXXXXXXXXX",
  "status": "InProgress",
  "createdAt": "2026-07-01T10:14:00Z",
  "commits": ["0f95dd3e", "d0927ef"],
  "pickIndex": 2,
  "pickTotal": 2,
  "testLevel": "RunLocalTests",
  "prUrl": null,
  "sourceRunId": null,
  "packageXml": ".deploydeck/manifest/delta/OTACUPYR-1386-to-UAT/package/package.xml",
  "destructiveChanges": ".deploydeck/manifest/delta/OTACUPYR-1386-to-UAT/destructiveChanges/destructiveChanges.xml"
}
```

### RF-014 - Push Y Preparacion De PR

Tras validacion correcta, la TUI debe ofrecer:

- Hacer push de la rama temporal.
- Mostrar comando de push.
- Mostrar base y compare para PR.
- Generar titulo sugerido del PR.
- Si `gh` esta instalado y autenticado: crear el PR con `gh pr create`, mostrando el comando antes y pidiendo confirmacion.
- Sin `gh`: mostrar la URL de compare derivada del remoto `origin` para abrir el PR en el navegador.

Ejemplo:

```bash
git push -u origin deploy/OTACUPYR-1386-to-UAT

gh pr create \
  --base UAT \
  --head deploy/OTACUPYR-1386-to-UAT \
  --title "OTACUPYR-1386 - Promote changes to UAT"
```

`gh` es una dependencia opcional: su ausencia nunca bloquea el flujo, solo desactiva la creacion del PR desde la TUI.

### RF-015 - Quick Deploy Opcional

Fase: Futuro (fuera del alcance de esta epica; ver HU-015 en `docs/HISTORIAS.md`).

Si una validacion fue exitosa y el entorno lo permite, la TUI puede mostrar comando de quick deploy. Solo es elegible si la validacion corrio los tests requeridos y tiene menos de 10 dias (limite de Salesforce para quick deploy).

```bash
sf project deploy quick \
  --job-id 0AfXXXXXXXXXXXX \
  --target-org PROD
```

Debe requerir confirmacion fuerte y no estar activado por defecto.

## Requisitos No Funcionales

### RNF-001 - Seguridad Operativa

- No modificar ramas protegidas directamente.
- Confirmar acciones destructivas.
- Mostrar comandos antes de ejecutarlos.
- Bloquear si el working tree esta sucio.
- No guardar credenciales.

### RNF-002 - Portabilidad

- Binario distribuible para macOS inicialmente.
- Preparado para Linux y Windows.
- Evitar dependencias de shell especificas.

### RNF-003 - Observabilidad Local

- Guardar logs por ejecucion.
- Guardar JSON raw de `sf` para diagnostico.
- Guardar package generado.
- Permitir reanudar consulta de un job por `jobId`.

### RNF-004 - Usabilidad

- Navegacion con teclado.
- Mensajes claros.
- Preview antes de acciones irreversibles.
- Pantallas de error accionables.
- Modo CLI futuro para automatizar pasos.

### RNF-005 - Robustez

- Manejo de timeouts.
- Reintento de polling.
- Manejo de `sf` no autenticado.
- Manejo de plugin no instalado.
- Soporte de errores de red.
- Lock de instancia unica por repo (evitar dos DeployDeck intercalando operaciones Git).
- Deteccion de hooks de Git del repo que puedan interferir en checkout/cherry-pick.

## Arquitectura Tecnica

Stack:

```text
Go
Bubble Tea
Bubbles
Lip Gloss
Cobra
YAML/JSON
git CLI
sf CLI
sfdx-git-delta
gh CLI (opcional, para crear PR)
```

Estructura propuesta:

```text
deploydeck/
  cmd/
    deploydeck/
      main.go

  internal/
    app/
      model.go
      update.go
      views.go
      commands.go

    git/
      status.go
      branches.go
      commits.go
      cherry_pick.go
      push.go

    salesforce/
      orgs.go
      deploy_validate.go
      deploy_report.go
      deploy_queue.go
      deploy_cancel.go
      deploy_quick.go

    delta/
      sgd.go
      package_summary.go
      destructive.go

    config/
      config.go
      defaults.go

    runs/
      store.go
      report.go

    exec/
      command.go
      json.go

  configs/
    deploydeck.example.yaml

  EPICA.md

  docs/
    ARQUITECTURA.md
    HISTORIAS.md
    MOCKUPS_TUI.md
```

## Configuracion

Archivo sugerido:

```yaml
branches:
  integration: INT
  uat: UAT
  production: main
  releasePattern: "Release/*"

sandboxes:
  INT:
    alias: INT_SANDBOX
    testLevel: RunLocalTests

  UAT:
    alias: UAT_SANDBOX
    testLevel: RunLocalTests

  "Release/*":
    alias: PREPROD_SANDBOX
    testLevel: RunLocalTests

  main:
    alias: PROD
    testLevel: RunLocalTests

delta:
  outputDir: .deploydeck/manifest/delta
  sourceDirs:
    - up_saln0001_giss_salesforce/force-app
  ignoreFile: .sgdignore
  ignoreDestructiveFile: .sgd-destructive-ignore

ticketPatterns:
  - "OTACUPYR-[0-9]+"
  - "INC[0-9]+"

branchPrefixes:
  - feature/
  - fix/
  - hotfix/
  - Epic/
  - UserStory/

branchFormat: "deploy/{{ticket}}-to-{{target}}"
pollIntervalSeconds: 10

runs:
  keepLast: 30
  keepDays: 90

minVersions:
  git: "2.30.0"
  sf: "2.0.0"
  sfdx-git-delta: "5.0.0"
```

## Modelo De Datos Interno

```go
type Commit struct {
    SHA         string
    ShortSHA    string
    Message     string
    Author      string
    Date        time.Time
    IsMerge     bool
    InTarget    bool
    BranchHints []string
}

type DeploymentPlan struct {
    Ticket             string
    SourceBranch       string
    TargetBranch       string
    TargetOrg          string
    TemporaryBranch    string
    Commits            []Commit
    PackageXML         string
    DestructiveChanges string
}

type ValidationStatus struct {
    JobID             string
    Status            string
    Done              bool
    ComponentsDone    int
    ComponentsTotal   int
    TestsDone         int
    TestsTotal        int
    ComponentFailures []string
    TestFailures      []string
    StartedAt         time.Time
    CompletedAt       *time.Time
}

type DeployQueueItem struct {
    JobID              string
    Status             string
    CreatedBy          string
    CreatedDate        time.Time
    StartDate          *time.Time
    ComponentsDone     int
    ComponentsTotal    int
    TestsDone          int
    TestsTotal         int
}
```

## Pantallas Principales

### Menu Principal

```text
+------------------------------------------------------------------------------+
| DeployDeck                                                                   |
+------------------------------------------------------------------------------+

  Que quieres hacer?

  > Promocionar commits
    Generar delta package
    Validar package contra sandbox
    Ver cola de despliegues
    Ver historial de validaciones
    Configuracion
    Salir

+------------------------------------------------------------------------------+
```

### Seleccion De Commits

```text
+------------------------------------------------------------------------------+
| Commits Encontrados: OTACUPYR-1386                                           |
+------------------------------------------------------------------------------+

  Rama origen sugerida:
  origin/feature/OTACUPYR-1386

  [x] 0f95dd3e  feature/OTACUPYR-1386 Fix INT
  [x] d0927ef  feature/OTACUPYR-1386 Fields Read Only
  [ ] 90f8634  MERGE - Pull request #213
  [!] 3c9cb8a  Ya existe en destino

+------------------------------------------------------------------------------+
| Space marcar   a todos   Enter continuar   Esc volver                        |
+------------------------------------------------------------------------------+
```

### Validacion En Vivo

```text
+------------------------------------------------------------------------------+
| Validando Contra UAT_SANDBOX                                                 |
+------------------------------------------------------------------------------+

  Job Id: 0AfXXXXXXXXXXXX
  Estado: InProgress

  Metadata:
  [############..............] 12 / 34 componentes

  Tests:
  [#################.........] 80 / 142 tests

  Tiempo: 00:18:42

  Ultimo evento: Ejecutando Apex tests...

+------------------------------------------------------------------------------+
| r refrescar ahora   c cancelar validacion   q salir dejando job activo       |
+------------------------------------------------------------------------------+
```

## Historias De Usuario

Las historias de usuario, sus criterios de aceptacion, prioridades y orden de implementacion se mantienen en un unico documento para evitar numeraciones divergentes:

Ver `docs/HISTORIAS.md` (fuente de verdad, HU-001 a HU-019).

## Plan De Entrega

### Fase 1 - MVP Git

- Crear proyecto Go.
- Implementar TUI base.
- Validar prerequisitos.
- Buscar commits por ticket.
- Seleccionar commits.
- Crear rama temporal.
- Ejecutar cherry-pick.
- Verificar estado final tras cherry-pick (deteccion de promocion parcial).

### Fase 2 - Delta Salesforce

- Integrar `sfdx-git-delta`.
- Generar delta package.
- Parsear `package.xml`.
- Detectar destructive changes.
- Mostrar resumen.

### Fase 3 - Validacion Salesforce

- Ejecutar `sf project deploy validate --async --json`.
- Capturar `jobId`.
- Implementar polling con `deploy report`.
- Mostrar progreso.
- Guardar reporte local.

### Fase 4 - Cola De Deploys

- Query a `DeployRequest`.
- Mostrar jobs activos.
- Mostrar posicion aproximada.
- Permitir cancelar job propio.

### Fase 5 - Productizacion

- Push de rama.
- Preparacion de PR.
- Historial de ejecuciones y re-promocion de tickets entre ambientes.
- Limpieza de ramas temporales y retencion de runs.
- Config YAML.
- Binarios distribuibles y pipeline CI de build/release.
- Documentacion de uso.

### Fase Futura (fuera de esta epica)

- Quick deploy opcional.
- Modos standalone: generar delta o validar package sin flujo completo.
- Promocion multi-ticket para ramas de release.

## Riesgos

| Riesgo | Mitigacion |
| --- | --- |
| Delta package incompleto | Resumen visible, fallback manual/full deploy |
| Destructive changes peligrosos | Confirmacion explicita y resaltado |
| Conflictos de cherry-pick | Detener flujo y dejar instrucciones claras |
| CLI Salesforce cambia salida | Usar `--json`, no parsear salida humana |
| Sandbox ocupada | Mostrar cola antes de lanzar |
| Usuarios con versiones distintas | Validar versiones al inicio |
| Credenciales expiradas | Detectar org no autenticada y mostrar comando |
| Merge commits complejos | Bloquear su seleccion en MVP |
| Squash merges ocultan commits originales | Politica de merge commit en PRs de promocion (DEC-001) y deteccion de pick vacio |
| Promocion parcial de ficheros compartidos entre tickets | Warning de dependencias en seleccion y verificacion post-pick contra rama origen |
| Perfil sin permisos Tooling API | Cola opcional: avisar y continuar sin bloquear |
| Hooks de Git del repo interfieren | Detectarlos en doctor y documentar workaround |
| sgd sin soporte multi `--source-dir` | Spike tecnico al inicio de Fase 2; fallback: iterar por directorio y fusionar packages |

## Metricas De Exito

Para evaluar adopcion tras la entrega:

- Tiempo medio de preparacion de una promocion (antes vs despues de DeployDeck).
- Porcentaje de validaciones fallidas por package incompleto o commit olvidado.
- Numero de usuarios activos semanales sobre el total del equipo.
- Numero de promociones realizadas con DeployDeck vs manuales.

## Criterios De Aceptacion De La Epica

La epica se considera completada cuando:

- Un desarrollador puede seleccionar commits por ticket.
- La herramienta detecta commits ya promocionados por equivalencia de contenido, no solo por SHA.
- La herramienta avisa de promociones parciales de ficheros compartidos entre tickets.
- La herramienta crea una rama temporal desde destino.
- La herramienta aplica cherry-picks seleccionados.
- La herramienta genera delta package.
- La herramienta muestra resumen del package.
- La herramienta valida contra sandbox con `sf`.
- La herramienta muestra progreso en vivo.
- La herramienta muestra cola de despliegues.
- La herramienta guarda el historial local.
- La herramienta permite hacer push de la rama validada.
- El flujo esta documentado para el equipo.

## Definicion De Done

- Codigo versionado en repo de DeployDeck.
- README con instalacion y uso.
- Config de ejemplo incluida.
- Binario compilable localmente.
- Pruebas unitarias para parsing de Git, package XML y JSON de Salesforce.
- Prueba manual end-to-end contra sandbox de INT.
- Documentacion de troubleshooting.
- Validacion con al menos dos desarrolladores del equipo.
