# Guía de instalación y primera prueba en Windows

Esta guía explica cómo instalar DeployDeck en Windows y hacer una primera
validación sin ejecutar despliegues reales.

## 1. Requisitos

Necesitas acceso a las organizaciones Salesforce y al repositorio de GitHub
Enterprise, además de estas herramientas:

- [Git]
- [Salesforce CLI (`sf`)]
- [Scoop]
- [GitHub CLI (`gh`)]

Comprueba las instalaciones:

```powershell
git --version
sf --version
scoop --version
gh --version
```

## 2. Instalar Scoop

En PowerShell, ejecuta:

```powershell
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
irm get.scoop.sh | iex
```

Reinicia PowerShell o VS Code y comprueba la instalación:

```powershell
scoop --version
```

## 3. Instalar DeployDeck

```powershell
scoop bucket add deploydeck https://github.com/malavolta/scoop-bucket
scoop install deploydeck
deploydeck --version
```

Si el bucket responde `already exists`, ya estaba configurado. Para actualizar
DeployDeck posteriormente:

```powershell
scoop update deploydeck
```

## 4. Instalar el plugin de delta

DeployDeck utiliza `sfdx-git-delta`:

```powershell
sf plugins install sfdx-git-delta
sf plugins
```

`sfdx-git-delta` debe aparecer en el listado.

## 5. Autenticarse en Salesforce

Consulta las organizaciones disponibles:

```powershell
sf org list
```

Los alias deben coincidir con los configurados en `deploydeck.yaml`. Si falta
una organización, autentícala con `sf org login web --alias <ALIAS>`.

## 6. Autenticarse en GitHub Enterprise

Estar autenticado en Git o en el navegador no implica que `gh` lo esté.
Primero, comprueba dónde está alojado el repositorio:

```powershell
git remote -v
```

Para GitHub Enterprise de IBM:

```powershell
gh auth login --hostname github.ibm.com
```

Selecciona HTTPS y autenticación mediante navegador. Introduce en la página
abierta el código que muestra la terminal y completa el SSO corporativo.

Comprueba el resultado:

```powershell
gh auth status --hostname github.ibm.com
```

## 7. Crear `deploydeck.yaml`

Crea `deploydeck.yaml` junto a `sfdx-project.json`, este es el contenido que tiene que llevar, siendo los alias de las branches el alias que tengas puesto tú a la org:

```yaml
projectDir: up_saln0001_giss_salesforce

branches:
  integration: INT
  uat: UAT

sandboxes:
  INT:
    alias: INT
    testLevel: RunRelevantTests
  UAT:
    alias: UAT
    testLevel: RunRelevantTests

ticketPatterns:
  - "DEMO-[0-9]+"

branchFormat: "deploy/{{ticket}}-to-{{target}}"

delta:
  sourceDirs:
    - up_saln0001_giss_salesforce/force-app
  outputDir: .deploydeck/manifest/delta

quickDeploy:
  allowExecution: false

ai:
  enabled: false
```

`delta.sourceDirs` siempre es relativa a la raíz Git. Mantén `allowExecution: false` para impedir despliegues
reales durante las pruebas.

## 8. Ignorar archivos temporales

Añade esta entrada a `.gitignore`:

```gitignore
.deploydeck/
```

El archivo `.deploydeck/lock` es temporal y nunca debe subirse al repositorio.

## 9. Validar la instalación

Desde la carpeta que contiene `deploydeck.yaml`, ejecuta:

```powershell
deploydeck doctor
```

Debe finalizar sin elementos `[blocking]`. El árbol Git debe estar limpio, así
que confirma o guarda temporalmente cualquier cambio pendiente.

## 10. Preparar una prueba

Crea una rama de prueba:

```powershell
git checkout -b feature/probar-deploydeck
```

En una clase Apex sencilla, añade únicamente:

```apex
// Prueba DeployDeck
```

Revisa y confirma el cambio:

```powershell
git diff
git status --short
git commit -m "DEMO-123 prueba DeployDeck"
```

## 11. Ejecutar DeployDeck

Con el árbol limpio:

```powershell
deploydeck doctor
deploydeck
```

En la interfaz:

1. Selecciona la promoción hacia INT.
2. Revisa la rama y el commit propuestos.
3. Ejecuta la validación.
4. Confirma la creación de la PR.

DeployDeck puede proponer comandos como:

```powershell
git fetch origin
git checkout -b deploy/DEMO-123-to-INT origin/INT
git cherry-pick -x <HASH_DEL_COMMIT>
```

Un informe completo de validación no significa automáticamente que la PR se
haya creado: completa la acción de creación de PR.

## 12. Comprobar la PR

```powershell
gh pr list
```