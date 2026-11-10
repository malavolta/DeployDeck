# Update Notification Specification

## Purpose

Non-blocking startup check that informs the user when a newer DeployDeck
version exists, without ever delaying or gating the TUI.

### Requirement: Non-Blocking Startup Check

The system MUST run the update-availability check concurrently with TUI
startup. The check MUST NOT delay, block, or gate the TUI's initial render
or its readiness for user interaction, regardless of how long the check
takes.

(HU-019 Tarea, docs/HISTORIAS.md:1174; AC5, docs/HISTORIAS.md:1183)

#### Scenario: Check runs concurrently with startup

- GIVEN the update endpoint is reachable and responds normally
- WHEN the TUI starts
- THEN the TUI becomes interactive without waiting for the update check to
  complete

#### Scenario: Slow or unreachable endpoint never blocks startup

- GIVEN the update endpoint is down, slow, unreachable, or times out
- WHEN the TUI starts
- THEN startup proceeds normally with no delay attributable to the check

(Test E2E variant, docs/HISTORIAS.md:1194)

### Requirement: Semver-Based Newer-Version Detection

The system MUST determine whether a newer version is available by comparing the running version against the latest known release using semantic-version comparison (`HasNewer`). The running version is `version.Version`. Pre-release tags MUST NOT be reported as newer.

(Previously: pre-release handling was unspecified in the spec.)

(Test E2E, docs/HISTORIAS.md:1191,1193)

#### Scenario: Newer version available shows a notice

- GIVEN the latest known release version is semver-greater than the
  running version
- WHEN the update check completes
- THEN a non-blocking notice/banner is shown to the user

#### Scenario: Same or older version shows no notice

- GIVEN the latest known release version is equal to or semver-older than
  the running version
- WHEN the update check completes
- THEN no notice is shown

#### Scenario: Pre-release tag never triggers a notice

- GIVEN the latest known release tag is a pre-release such as `v2.0.0-rc1`
- WHEN the update check completes
- THEN no notice is shown

### Requirement: Silent Skip on Check Failure

The system MUST treat any check failure — timeout, network error,
unreachable host, non-2xx response, or unauthorized (401) response —
identically: it MUST skip the notice silently, without surfacing an error
to the user and without distinguishing the failure cause in the UI.

(AC5 + Nota decision pendiente, docs/HISTORIAS.md:1183,1185; Test E2E
variant, docs/HISTORIAS.md:1194)

#### Scenario: Down or slow endpoint yields no notice and no error

- GIVEN the update endpoint is down, slow past its timeout, or returns a
  non-2xx/401 response
- WHEN the update check completes or times out
- THEN no notice is shown and no error message is surfaced to the user

### Requirement: Real Release Repository

The update check MUST query the latest-release endpoint of the repository `malavolta/DeployDeck`. The placeholder value `OWNER/REPO` MUST NOT be used for any request.

#### Scenario: Request targets the real repository

- GIVEN a checker built with its default repository and an injected HTTP test server recording request paths
- WHEN the checker fetches the latest release
- THEN the recorded request path is `/repos/malavolta/DeployDeck/releases/latest`

### Requirement: 24-Hour Check Cache

The system MUST cache the latest-release tag together with the check timestamp in a JSON file named `update-check.json` under `<os.UserCacheDir()>/deploydeck/`. While the cached entry is younger than 24 hours, the system MUST use the cached tag and MUST NOT perform a network request. When the entry is 24 hours old or older, the system MUST perform a live check and rewrite the cache on success.

A failed live check (timeout, network error, non-2xx, malformed body) MUST NOT be cached. Any cache problem (missing directory, unreadable, corrupt JSON, unwritable) MUST degrade to a live check and MUST NOT surface an error to the user. Cache writes SHOULD be atomic.

#### Scenario: Fresh cache avoids the network

- GIVEN a cache entry with `latest = "v1.2.0"` written 1 hour ago (fake clock) and an HTTP server that fails the test if called
- WHEN the update check runs
- THEN the result is `v1.2.0` and no HTTP request is made

#### Scenario: Stale cache triggers a live check and refresh

- GIVEN a cache entry written 25 hours ago and a server answering `v1.3.0`
- WHEN the update check runs
- THEN one HTTP request is made, the result is `v1.3.0`, and the cache file now holds `v1.3.0` with the current timestamp

#### Scenario: Failed check is not cached

- GIVEN no cache entry and a server answering HTTP 500
- WHEN the update check runs
- THEN no notice is produced, no error is surfaced, and no cache file is created

#### Scenario: Corrupt cache degrades to a live check

- GIVEN a cache file containing invalid JSON and a server answering `v1.3.0`
- WHEN the update check runs
- THEN the result is `v1.3.0` and the cache file is overwritten with valid content

#### Scenario: Unwritable cache directory does not break the check

- GIVEN a cache directory that cannot be created or written
- WHEN the update check runs against a server answering `v1.3.0`
- THEN the result is `v1.3.0` and no error is surfaced

### Requirement: Update Check Opt-Out

When the environment variable `DEPLOYDECK_NO_UPDATE_CHECK` is set to a non-empty value, the system MUST NOT perform any update check (no network request, no cache read or write) and MUST NOT show any update banner. When the variable is unset or empty, the check MUST behave normally.

#### Scenario: Opt-out disables the check

- GIVEN `DEPLOYDECK_NO_UPDATE_CHECK=1` and a server that fails the test if called
- WHEN the TUI composition wires the update check
- THEN no update check function is wired, no request is made, and no banner is shown

#### Scenario: Empty value does not opt out

- GIVEN `DEPLOYDECK_NO_UPDATE_CHECK` set to the empty string
- WHEN the TUI composition wires the update check
- THEN the update check is wired as usual

### Requirement: Spanish Update Banner With Action Hint

The update banner MUST be rendered in Spanish and MUST include the latest version. On the main menu (`StateMainMenu`), when an update is available, the banner MUST include an action hint, and the hint depends on the install method:

- Install method with a known upgrade command (Brew, Scoop, GoInstall): the hint MUST tell the user to press `u` to update.
- Manual install: the hint MUST show the releases URL `https://github.com/malavolta/DeployDeck/releases/latest` and MUST NOT advertise the `u` key.

On any state other than `StateMainMenu`, the banner MUST NOT show the `u` hint. When no update is available, no banner is rendered. The exact Spanish wording is a design decision; tests MUST assert the pinned fragments: the latest version string, the `u` key hint for upgradable methods, and the releases URL for manual installs. The previous English banner text MUST no longer be rendered, and tests pinning it MUST be updated.

#### Scenario: Banner on main menu for an upgradable install

- GIVEN an update to `v1.3.0` is available and the install method has an upgrade command
- WHEN the main menu is rendered
- THEN the output contains `v1.3.0` and a Spanish message with a hint to press `u`

#### Scenario: Banner on main menu for a manual install

- GIVEN an update to `v1.3.0` is available and the install method is Manual
- WHEN the main menu is rendered
- THEN the output contains `v1.3.0` and `https://github.com/malavolta/DeployDeck/releases/latest`, and does not contain a hint to press `u`

#### Scenario: No u hint outside the main menu

- GIVEN an update is available and the model is in a state other than `StateMainMenu`
- WHEN the view is rendered
- THEN the output does not contain the `u` key hint

#### Scenario: No banner without an update

- GIVEN no update is available
- WHEN any view is rendered
- THEN no update banner text appears
