# Mockups TUI - DeployDeck

## Principios De UI

- Navegacion principalmente por teclado.
- Mostrar comandos antes de acciones de impacto.
- Separar errores bloqueantes de warnings.
- Mantener visible ticket, destino y sandbox durante el flujo.
- Permitir salir sin perder contexto cuando exista un run guardado.

## Doctor / Prerequisitos

```text
+------------------------------------------------------------------------------+
| DeployDeck > Doctor                                                          |
+------------------------------------------------------------------------------+

  Repo: /workspace/salesforce

  Prerequisitos

  [OK] Git disponible                         git version 2.45.0
  [OK] Dentro de repo Git                      origin/git-repo
  [OK] Remoto origin                           git@github.com:org/repo.git
  [OK] Salesforce CLI                          @salesforce/cli 2.x
  [OK] Plugin sfdx-git-delta                   installed
  [OK] Versiones minimas                       git 2.45 / sf 2.x / sgd 5.x
  [OK] .deploydeck/ en .gitignore              ignorado
  [OK] Lock de instancia                       sin otra instancia activa
  [OK] Working tree limpio                     sin cambios
  [--] gh CLI (opcional)                       no instalado: PR manual via URL
  [!!] Alias UAT_SANDBOX                       no autenticado

  Accion sugerida:
  sf org login web --alias UAT_SANDBOX

+------------------------------------------------------------------------------+
| r reintentar   c continuar con warnings   q salir                             |
+------------------------------------------------------------------------------+
```

## Menu Principal

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
| Enter seleccionar   ↑/↓ navegar   q salir                                     |
+------------------------------------------------------------------------------+
```

## Busqueda De Ticket

```text
+------------------------------------------------------------------------------+
| DeployDeck > Promocionar Commits                                             |
+------------------------------------------------------------------------------+

  Ticket o incidencia

  OTACUPYR-1386_

  Patrones configurados:
  - OTACUPYR-[0-9]+
  - INC[0-9]+

  Tambien se buscaran ramas que contengan el texto introducido.

+------------------------------------------------------------------------------+
| Enter buscar   Esc volver                                                     |
+------------------------------------------------------------------------------+
```

## Seleccion De Commits

```text
+------------------------------------------------------------------------------+
| DeployDeck > Commits Encontrados                                             |
+------------------------------------------------------------------------------+

  Ticket: OTACUPYR-1386
  Rama sugerida: origin/feature/OTACUPYR-1386
  Destino preliminar: UAT

  [x] 0f95dd3e  2026-06-29  ana   Fix INT validation
  [x] d0927ef1  2026-06-30  ana   Fields read only
  [-] 90f8634a  2026-07-01  bot   MERGE - Pull request #213        [merge - bloqueado]
  [!] 3c9cb8aa  2026-07-01  ana   Already promoted                 [equivalente en destino]

  Seleccionados: 2

  Warnings:
  - AccountService.cls tiene 1 commit intermedio no seleccionado (OTACUPYR-1390)
  - d0927ef1 menciona tambien OTACUPYR-1390

+------------------------------------------------------------------------------+
| Space marcar   a todos   i detalle   Enter continuar   Esc volver             |
+------------------------------------------------------------------------------+
```

## Seleccion De Destino

```text
+------------------------------------------------------------------------------+
| DeployDeck > Seleccionar Destino                                             |
+------------------------------------------------------------------------------+

  Ticket: OTACUPYR-1386
  Commits seleccionados: 2

  Rama destino
  > INT             origin/INT             a13df2c  Sandbox: INT_SANDBOX
    UAT             origin/UAT             0b451fe  Sandbox: UAT_SANDBOX
    Release/Julio…  origin/Release/Jul…    77ab0c1  Sandbox: PREPROD_SANDBOX
    main            origin/main            98caa10  Sandbox: PROD  [protegida]
    custom...

  Test level: RunLocalTests

+------------------------------------------------------------------------------+
| Enter seleccionar   ↑/↓ navegar   Esc volver                                  |
+------------------------------------------------------------------------------+
```

## Preview Del Plan

```text
+------------------------------------------------------------------------------+
| DeployDeck > Preview Del Plan                                                |
+------------------------------------------------------------------------------+

  Ticket:          OTACUPYR-1386
  Target branch:   UAT
  Target org:      UAT_SANDBOX
  Deploy branch:   deploy/OTACUPYR-1386-to-UAT

  Commits a aplicar:
  1. 0f95dd3e  Fix INT validation
  2. d0927ef1  Fields read only

  Comandos previstos:
  git fetch origin
  git checkout -b deploy/OTACUPYR-1386-to-UAT origin/UAT
  git cherry-pick 0f95dd3e
  git cherry-pick d0927ef1

+------------------------------------------------------------------------------+
| Enter ejecutar   e editar rama   Esc volver                                   |
+------------------------------------------------------------------------------+
```

## Cherry-Pick En Progreso

```text
+------------------------------------------------------------------------------+
| DeployDeck > Aplicando Cherry-Picks                                          |
+------------------------------------------------------------------------------+

  Rama: deploy/OTACUPYR-1386-to-UAT
  Base: origin/UAT

  [OK] git fetch origin
  [OK] crear rama temporal
  [OK] 0f95dd3e Fix INT validation
  [SK] 3c9cb8aa Already promoted (pick vacio: ya estaba en destino, saltado)
  [..] d0927ef1 Fields read only

  Ultimo comando:
  git cherry-pick d0927ef1

+------------------------------------------------------------------------------+
| q salir cuando termine                                                        |
+------------------------------------------------------------------------------+
```

## Conflicto De Cherry-Pick

```text
+------------------------------------------------------------------------------+
| DeployDeck > Conflicto De Cherry-Pick (pick 2 de 5)                          |
+------------------------------------------------------------------------------+

  Commit actual: d0927ef1 Fields read only

  Archivos en conflicto (se actualiza solo al detectar cambios en el repo):

  [U] classes/AccountService.cls                       sin resolver
  [S] objects/Account/fields/Status__c.field-meta.xml  resuelto y staged
  [D] classes/LegacyHelper.cls                         modify/delete: conservar o borrar

  Resuelve con tu herramienta preferida (IDE, otra terminal); esta pantalla
  detecta la resolucion automaticamente. Tambien puedes abrir desde aqui:

  e  abrir $EDITOR sobre el fichero seleccionado
  m  lanzar git mergetool

  Continuar: deshabilitado (1 sin resolver, 1 modify/delete pendiente)

+------------------------------------------------------------------------------+
| e editor   m mergetool   c continuar   s saltar   a abortar   q salir y retomar |
+------------------------------------------------------------------------------+
```

Notas:

- `c continuar` solo se habilita cuando no quedan paths sin mergear, lo resuelto esta staged y no hay marcadores `<<<<<<<` en ficheros staged.
- Si el usuario ejecuta `--continue` o `--abort` por fuera, la pantalla lo detecta y se resincroniza.
- `q salir y retomar` guarda el run en `CherryPickConflict`; al reabrir DeployDeck se ofrece volver aqui.
- Si `git rerere` auto-resuelve, el fichero aparece como "auto-resuelto con resolucion previa" pendiente de confirmar.

## Verificacion Post Cherry-Pick

```text
+------------------------------------------------------------------------------+
| DeployDeck > Verificacion De Promocion                                       |
+------------------------------------------------------------------------------+

  Comparando estado final contra origin/feature/OTACUPYR-1386

  [OK] classes/AccountService.cls                  identico a origen
  [OK] lwc/accountPanel/accountPanel.js            identico a origen
  [!!] objects/Account/fields/Status__c.field-meta.xml
       difiere de origen: promocion parcial (commits de OTACUPYR-1390
       no seleccionados tocan este fichero)

  El estado resultante de los ficheros marcados no existe en ninguna rama
  y no ha sido probado. Revisa antes de generar el delta.

+------------------------------------------------------------------------------+
| Enter continuar igualmente   v ver diff   e editar seleccion   Esc volver     |
+------------------------------------------------------------------------------+
```

## Resumen De Package

```text
+------------------------------------------------------------------------------+
| DeployDeck > Resumen De Package                                              |
+------------------------------------------------------------------------------+

  Package: .deploydeck/manifest/delta/OTACUPYR-1386-to-UAT/package/package.xml

  Metadata incluida:
  ApexClass                   3
  LightningComponentBundle    2
  CustomField                 4   [sensible]
  Profile                     1   [sensible]
  Layout                      1

  Destructive changes:
  CustomField                 1   [requiere confirmacion]

  Warnings:
  - El package contiene Profile.
  - Hay destructive changes.

+------------------------------------------------------------------------------+
| Enter continuar   d ver destructive   p abrir package   Esc volver            |
+------------------------------------------------------------------------------+
```

## Cola De Deploys

```text
+------------------------------------------------------------------------------+
| DeployDeck > Cola De Deploys: UAT_SANDBOX                                    |
+------------------------------------------------------------------------------+

  Jobs activos

  1. 0AfAAA  InProgress  maria.garcia   18m  Components 12/34  Tests 80/142
  2. 0AfBBB  Pending     luis.perez      4m  Components 0/0    Tests 0/0
  3. 0AfCCC  Pending     tu usuario      1m  Components 0/0    Tests 0/0  [propio]

  Posicion aproximada de tu job: 3

+------------------------------------------------------------------------------+
| r refrescar   Enter continuar validacion   Esc volver                         |
+------------------------------------------------------------------------------+
```

## Validacion En Vivo

```text
+------------------------------------------------------------------------------+
| DeployDeck > Validando Contra UAT_SANDBOX                                    |
+------------------------------------------------------------------------------+

  Ticket: OTACUPYR-1386
  Branch: deploy/OTACUPYR-1386-to-UAT
  Job Id: 0AfXXXXXXXXXXXX
  Estado: InProgress

  Metadata:
  [############..............] 12 / 34 componentes

  Tests:
  [#################.........] 80 / 142 tests

  Tiempo: 00:18:42
  Ultima consulta: 10:32:17
  Ultimo evento: Ejecutando Apex tests...

+------------------------------------------------------------------------------+
| r refrescar   c cancelar mi job   q salir dejando job activo                  |
+------------------------------------------------------------------------------+
```

## Resultado De Validacion

```text
+------------------------------------------------------------------------------+
| DeployDeck > Resultado De Validacion                                         |
+------------------------------------------------------------------------------+

  Job Id: 0AfXXXXXXXXXXXX
  Estado: Failed

  Errores de metadata:
  - ApexClass AccountService: Method does not exist or incorrect signature

  Tests fallidos:
  - AccountServiceTest.shouldCreateAccount
    System.AssertException: Expected true, got false

  Artefactos guardados:
  .deploydeck/runs/20260702-101400-OTACUPYR-1386-UAT/

+------------------------------------------------------------------------------+
| r reconsultar   h historial   q salir                                         |
+------------------------------------------------------------------------------+
```

## Confirmacion De Cancelacion

```text
+------------------------------------------------------------------------------+
| DeployDeck > Cancelar Validacion                                             |
+------------------------------------------------------------------------------+

  Vas a cancelar tu job:

  Job Id: 0AfXXXXXXXXXXXX
  Org:    UAT_SANDBOX
  Estado: InProgress

  Esta accion no afecta jobs de otros usuarios.

  Escribe CANCELAR para confirmar:
  CANCELAR_

+------------------------------------------------------------------------------+
| Enter confirmar   Esc volver                                                  |
+------------------------------------------------------------------------------+
```

## Historial De Runs

```text
+------------------------------------------------------------------------------+
| DeployDeck > Historial                                                       |
+------------------------------------------------------------------------------+

  > 2026-07-02 10:14  OTACUPYR-1386  UAT  InProgress  0AfXXXXXXXXXXXX
    2026-07-01 17:22  INC000001170030 INT  Succeeded   0AfYYYYYYYYYYYY
    2026-07-01 09:05  OTACUPYR-1301  UAT  Failed      0AfZZZZZZZZZZZZ

  Run seleccionado:
  Branch: deploy/OTACUPYR-1386-to-UAT
  Commits: 2
  Package: .deploydeck/manifest/delta/OTACUPYR-1386-to-UAT/package/package.xml

+------------------------------------------------------------------------------+
| Enter reanudar   d detalle   ↑/↓ navegar   q salir                            |
+------------------------------------------------------------------------------+
```

## Push Y PR

```text
+------------------------------------------------------------------------------+
| DeployDeck > Push Y PR                                                       |
+------------------------------------------------------------------------------+

  Validacion exitosa

  Branch local:  deploy/OTACUPYR-1386-to-UAT
  Target branch: UAT
  Job Id:        0AfXXXXXXXXXXXX

  Comando:
  git push -u origin deploy/OTACUPYR-1386-to-UAT

  PR sugerido:
  base:    UAT
  compare: deploy/OTACUPYR-1386-to-UAT
  title:   OTACUPYR-1386 - Promote changes to UAT

  gh detectado y autenticado. Comando de PR:
  gh pr create --base UAT --head deploy/OTACUPYR-1386-to-UAT \
    --title "OTACUPYR-1386 - Promote changes to UAT"

+------------------------------------------------------------------------------+
| p ejecutar push   g crear PR con gh   c copiar datos PR   q salir             |
+------------------------------------------------------------------------------+
```

## Quick Deploy Opcional

```text
+------------------------------------------------------------------------------+
| DeployDeck > Quick Deploy Opcional                                           |
+------------------------------------------------------------------------------+

  Validacion elegible para quick deploy

  Job Id: 0AfXXXXXXXXXXXX
  Target org sugerida: PROD

  Comando sugerido:
  sf project deploy quick --job-id 0AfXXXXXXXXXXXX --target-org PROD

  Quick deploy esta desactivado por defecto.
  Requiere configuracion explicita y confirmacion fuerte.

+------------------------------------------------------------------------------+
| q salir                                                                       |
+------------------------------------------------------------------------------+
```
