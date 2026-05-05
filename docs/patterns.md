# FFEditor — Design Patterns

## Elm Architecture (via Bubble Tea)

The entire app follows the `Model → Update → View` pattern. Each
component (`browser`, `tagger`, `cmdbar`) is its own Bubble Tea model,
composed under a root model that routes messages and manages focus.

## Finite State Machine

`model.go` manages a `mode` enum (`modeBrowse`, `modeCommand`,
`modeTag`, etc.) as an explicit state machine. Each state determines
which sub-model handles input and what the view renders.

## Message-Passing Concurrency

All I/O runs in `tea.Cmd` goroutines that return typed messages
(`convertDoneMsg`, `tagSavedMsg`, etc.). The `Update` function is
single-threaded — no locks needed. Cancellation is coordinated via
`context.WithCancel`.

## Sequential Work Queue

Conversions run one-at-a-time via a self-chaining Cmd pattern: each
completion message triggers the next item in the queue, rather than
spawning parallel goroutines.

## Null-Object Selection

`selectedEntries()` in `browser.go` returns the cursor entry when
nothing is selected, giving commands a unified single/multi code path
with no special-casing.

## Package-Level Var for Test Injection

`claudeAPIURL` in `claude.go` is a package-level var overridden in
tests to point at an `httptest.Server`, keeping the real request path
exercised without live API calls.

## Centralized Style Registry

All Lip Gloss style constants live in `styles.go` — no inline styling
in component files.
