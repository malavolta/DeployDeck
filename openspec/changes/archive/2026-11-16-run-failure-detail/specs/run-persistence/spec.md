# Delta for Run Persistence

## ADDED Requirements

### Requirement: Latest Report Read-Back With Fallback

The writer MUST expose `LatestReport(runID)` returning the raw bytes and path of the highest-numbered `report-NNN.json`. A parser (`ParseDeployReport`) MUST decode the first JSON value of the raw output, tolerating trailing non-JSON text, using the same mapping as `ReportDeploy`. If the latest report is unparseable, the reader MUST fall back to the previous parseable report. No format or retention change.

#### Scenario: Latest report wins
- GIVEN `report-001.json` and `report-003.json` exist and parse
- WHEN `LatestReport` is called
- THEN it returns `report-003.json`

#### Scenario: Trailing CLI text tolerated
- GIVEN a report file with a JSON envelope followed by warning text
- WHEN it is parsed
- THEN the report is returned without error

#### Scenario: Unparseable latest falls back
- GIVEN `report-002.json` is corrupt and `report-001.json` parses
- WHEN the report is read back
- THEN `report-001.json` is used and the corrupt path is reported

#### Scenario: No reports
- GIVEN a run directory with no `report-NNN.json`
- WHEN `LatestReport` is called
- THEN a not-found result is returned, not a panic
