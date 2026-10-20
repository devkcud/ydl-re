# Initial v0.3 terminal interface

[Documentation index](README.md)

This is the implementation contract for the first v0.3 slice. It defines a Linux terminal interface for monitoring and controlling jobs owned by an explicitly started daemon. **The TUI is not implemented or included in published `0.2.0-rc.1`.** The [implementation order](#implementation-order) keeps each step testable before the command is advertised.

## Scope

The planned entry point is `yoinkdl tui`, with `-h`/`--help` and `yoinkdl help tui`. It lists existing jobs, shows the selected job's details, and provides pause, resume, cancel, and forget. Closing the interface leaves the daemon and its transfers running.

Use the existing [private IPC contract](daemon-and-queue.md#local-control-protocol), `daemon.Resolve`, `daemon.Call`, `daemon.List`, and `queue.JobView`. The TUI has no transfer contexts, writable queue store, sidecar access, or source-URL cache. It does not start a daemon, create state/runtime directories, recover private snapshots, or change wire/storage versions.

Adding jobs and changing daemon settings remain available through the existing CLI in another terminal. URL entry, daemon start/stop, concurrency editing, profiles, configuration loading, persistent history, hooks, mouse input, and subscriptions are outside this first slice. The displayed terminal jobs are the existing bounded retained queue, not a new history database.

## Entry and terminal ownership

`--help` works without a terminal or daemon on every build target. Invalid arguments exit `2` before terminal or IPC work and never echo supplied values. There are no positional arguments, JSON mode, refresh-rate flag, or configuration file for this slice.

Interactive execution requires Linux, terminal stdin/stdout attached to the same foreground terminal, a nonempty `TERM` other than `dumb`, and readable dimensions of at least 60 columns by 10 rows. Refuse unsupported platforms, redirected input/output, mismatched/background terminals, and unavailable terminal capabilities with a safe diagnostic and exit `1`; no daemon request or terminal-mode change follows a failed preflight. Published native qualification initially remains Linux/amd64.

After preflight, resolve the existing Linux XDG paths and obtain one successful `list` response before entering the alternate screen. Missing/unreachable daemons fail with explicit startup guidance and exit `1`. Path, ownership, peer, protocol, and state-root checks remain authoritative. Startup never waits indefinitely or launches a replacement owner.

One session owns terminal modes, descriptor flags, input, output, and restoration. Save the original state before changing it; leave the alternate screen, restore cursor visibility and terminal/descriptor settings, and join local workers on every ordinary return or recoverable failure. Do not close caller-owned standard descriptors. Test the case where input and output share an underlying open description. Reuse the existing bounded output writer through the CLI adapter; output failure ends only the TUI.

`q` exits normally with `0`. SIGINT/Ctrl+C and SIGTERM restore the terminal and exit `130`. EOF, terminal hangup, output failure, and fatal IPC errors exit `1`. If an unacknowledged unknown outcome remains, or quitting interrupts a possibly sent mutation, report its operation and full job ID after terminal restoration; normal quit then exits `1`, while signal interruption retains `130`. A successful control reply followed by output failure must still be identified as accepted, never rolled back. Diagnostics use the existing bounded stderr path when available; failure to print them never changes the operation's outcome or falls back to screen output.

Support normal job-control suspension: restore terminal state and cancel/join local IO before stopping for Ctrl+Z/SIGTSTP. On SIGCONT, reacquire the foreground terminal, read its current settings/dimensions, and obtain a fresh list before enabling controls. Suspension never pauses daemon jobs or resends a pending mutation. SIGKILL and SIGSTOP cannot provide a terminal-restoration guarantee.

## Screen and navigation

| Area | Information |
|---|---|
| Header | Connected/refreshing/stale state, daemon running/stopping, active jobs, whole-job parallel limit, and shared body cap |
| Job list | Admission order, displayed ID prefix, state, observed bytes/total, workers, and output name as space permits |
| Selected job | Full 32-character ID, escaped output path, state, bytes/total, workers, publication `yes`/`no`/`unknown`, and safe error |
| Footer | Available keys, pending request or confirmation, and the latest safe result/error |

List order follows the daemon's admission order. Selection uses the full job ID, never its row index or shortened display prefix. Keep that selection when rows move; if it disappears, clear selection and require navigation before another job control. Empty queues show an explicit empty state. Navigation scrolls through all retained records up to the current 256-job bound.

Unknown counters/publication remain unknown, and progress may decrease. Display only fields present in the validated views; do not invent speed, ETA, retry counts, start times, or historical totals. Bytes reaching a total never imply completion or successful publication.

The first renderer uses ASCII labels and visibly escapes control characters, invalid UTF-8 bytes, and non-ASCII path/error text before clipping. Keep the original values in the model and never use display text as an IPC identifier or path. This gives deterministic cell widths without a new Unicode-width dependency; existing CLI text rendering remains unchanged. Generated terminal control sequences are the only executable escapes in a frame. Do not split an escape token when clipping, and provide visible truncation for long details.

Reserve the last terminal column/row to avoid wrapping or scrolling. Cap the viewport at 240 columns by 80 rows and each frame at 64 KiB. Resize redraws from the latest snapshot and preserves selection. Below the minimum dimensions, show only a clipped resize/quit notice and disable job controls until the layout and a fresh snapshot are available. Meaning never depends on color.

| Key | Planned action |
|---|---|
| Up/Down or `k`/`j` | Move selection |
| Page Up/Page Down, Home/End | Navigate the retained list |
| Space | Request one immediate refresh |
| `p` | Pause the selected queued/running job |
| `r` | Resume the selected paused/interrupted/failed job |
| `c` | Confirm cancellation of the selected eligible job |
| `f` | Confirm removal of the selected idle queue record |
| `q` | Quit this interface |
| Escape | Dismiss a confirmation without sending a request |

Input decoding must remain bounded and handle fragmented supported key sequences. Unknown escape sequences and bracketed-paste content must not become job controls. Do not retain pasted content. Key repeats cannot create a backlog of mutations.

## Polling and reconnects

Use protocol-v1 `list` polling. Schedule the next automatic read 500 ms after the previous request settles; manual refresh coalesces to one pending refresh. There is at most one IPC request in flight and one retained snapshot. Frames and input/results use bounded queues; slow terminals do not accumulate frames or keep a daemon socket open for display output. The model may retain one pending confirmation/control and one uncertainty notice, not an event history.

Each call has a child context capped at `daemon.Timeout` (currently five seconds), bounding the entire call in addition to the existing transport deadlines. Polls, controls, and terminal IO belong to the TUI's context only. Quitting or suspension cancels and joins local work without canceling a daemon-owned download.

On a connection failure/timeout after startup, retain the last snapshot with an explicit stale indicator and disable controls. Retry reads after one second, without accumulated catch-up polls. A successful read replaces the entire snapshot and revalidates selection. Each reconnect reuses the existing path/owner/peer checks; no alternate endpoint or implicit restart is attempted. Ownership, access, peer, state-root, protocol, or persistence failures stop automatic reconnects and end the interface with a safe diagnostic.

While a request is active, navigation and quit remain available; job-control keys are rejected with a busy indication rather than queued. A daemon reporting `stopping`, a stale snapshot, an unacknowledged uncertainty notice, or an undersized terminal disables controls. After a successful mutation, obtain a fresh list before enabling another control. A mutation reply is an acknowledgment; it does not replace the whole list or prove transfer cleanup has finished.

## Job controls and uncertainty

The existing coordinator owns transition legality and durability. UI eligibility only avoids obviously invalid requests; it is not authorization, a lock, or a second state machine for downloads.

| Control | Offer for the last observed states |
|---|---|
| Pause | `queued`, `running` |
| Resume | `paused`, `interrupted`, `failed` |
| Cancel | `queued`, `running`, `pausing`, `paused`, `interrupted`, `failed` |
| Forget | `queued`, `paused`, `interrupted`, `failed`, `canceled`, `completed` |

Disable redundant no-op controls. Never offer forget for running/pausing/canceling jobs; the daemon still checks its actual active slot. Conflicts, draining work, a forgotten ID, and concurrent changes are normal safe refusals followed by a refresh. A failed job with published output remains visible as such; resume still follows the existing no-overwrite policy.

Cancel and forget require an explicit `y` confirmation. Enter, `n`, Escape, navigation, disconnect, or a changed selected row dismisses the prompt. Pin the prompt to its full ID, admission order, output, and displayed state; a fresh snapshot changing those values cancels it. Explain that cancel keeps files and clears the retained source, and that forget removes the queue entry while preserving files; adding either again requires its URL. Pause/resume need no extra confirmation.

Send only the captured full ID, and verify that every successful control reply names that ID, as the existing queue CLI does. Confirmation cannot silently follow a changed row. The protocol remains ID-based with no atomic compare-and-set guard: another trusted client can race a displayed snapshot, including forgetting/reusing an ID. Do not claim stronger targeting or ordering guarantees than the current CLI/daemon protocol.

Never retry a mutation automatically. A lost/malformed reply, a mismatched returned ID, or interruption after a possible send becomes an unknown outcome. Preserve its operation and full ID in a visible notice, mark the list stale, and allow read-only reconciliation. A later list cannot prove which client caused a transition. Require a successful fresh read and explicit Enter acknowledgment before accepting another control; acknowledgment clears the UI restriction, not the uncertainty about the original operation. Retrying is a new explicit user action.

Use the existing safe failure classification. Never render raw network/parser errors, stored requests, source URLs, hashes, headers, or validators. Local output paths remain visible and may themselves contain sensitive text, as in the existing CLI. Neither a failed frame nor a disconnected TUI undoes an accepted operation.

## Ownership in the implementation

| Responsibility | Existing seam or planned location |
|---|---|
| Dispatch/help/argument errors | Extend [CLI dispatch](../internal/cli/cli.go) only when the interactive path is ready |
| Safe snapshots and controls | [Daemon client](../internal/daemon/client_linux.go), [validated replies](../internal/daemon/replies.go), and [queue views](../internal/queue/views.go) |
| Selection, confirmation, rendering, request scheduling | New `internal/tui` package with pure state/rendering and injected IO boundaries; it does not own queue storage or transfer work |
| CLI/IPC adaptation | New TUI-specific CLI adapter using `daemon.Resolve`/`Call`, decoded safe views, and the existing ID/failure classification rules from [queue CLI](../internal/cli/queue.go) and [queue output](../internal/cli/queue_output.go) |
| Terminal lifetime and bounded output | Linux TUI session helper in `internal/cli`, using [output ownership](../internal/cli/output.go) and the existing [PTY fixture approach](../internal/cli/terminal_linux_test.go); other builds refuse interactive use |

Keep `cli.Run`'s existing call contract. Pass `os.Stdin` only into the new TUI session helper, whose input/output and clock/call boundaries can be supplied by focused tests. Move a small existing helper only when sharing it prevents duplicated behavior; do not add a generic terminal framework, service layer, or progress bus. The standard library is the initial dependency choice.

## Implementation order

| Order | Complete responsibility | Required proof |
|---|---|---|
| F1 | Pure TUI model and bounded renderer over safe snapshots | Selection by full ID, reordered/removed rows, unknown values, all states, empty/full queues, clipped/escaped text, frame bounds, confirmation invalidation; no terminal, IPC, files, or goroutines |
| F2 | Poll/control loop with injected calls, time, input, and frames | One in-flight request, coalesced polls/refresh, bounded frames, stale/reconnect behavior, eligible controls, matching IDs, unknown outcomes, no automatic mutation retry, joined local cancellation |
| F3 | Linux terminal session and CLI entry | Terminal preflight, input decoding, bounded writes, restoration, shared-descriptor ownership, resize, suspend/resume, help/exit codes, explicit unsupported behavior; standalone and queue CLI regressions remain passing |
| F4 | Real daemon and PTY process qualification; final docs | Complete command path, live jobs, pause/resume/cancel/forget, peer/path refusal, dropped acknowledgments, output failure, signals, daemon restart, unchanged artifacts, and no implicit startup |

Update root STATUS and make focused local commits after each step. F1/F2 are internal work; they do not advertise an available `tui` command. F3 can wire the command only with its focused terminal/adapter coverage. Do not claim the complete first slice until F4 and the full gate pass. Profiles, history, and hooks require their own later contracts and ordering.

## Acceptance and validation

1. Opening the planned command against a live daemon shows only validated safe fields; it works with zero through 256 records and preserves selection through FIFO movement. Unknown/rolled-back progress and publication stay honest.
2. Pure renderer fixtures cover width/height extremes, long and hostile paths/errors, colliding display prefixes, every state, and stale/pending/confirmation views. No server-supplied text can become terminal commands, and frame size stays bounded.
3. A fake clock and controlled call completions prove polling bounds, one in-flight call, no mutation backlog/retry, fresh-list requirements, correct IDs, and continued navigation/quit during slow IPC/output. No sleeps are needed for these unit assertions.
4. Real local HTTP transfers and a daemon prove that pause/resume preserves exact bytes, cancellation/forget preserves artifacts, and state changes/unknown replies do not authorize overwrite or falsely claim completion. Terminal failure, quit, suspension, and signals leave accepted daemon work independent.
5. PTY process tests prove the original terminal/descriptor state is restored after normal quit, signals, startup/output failures, resize, and supported suspension/resume. Unsupported/nonterminal invocation emits no screen escapes, creates no daemon state, and leaves CLI compatibility intact.

For implementation, run the new focused package/process tests and relevant existing regressions, then `make check`. Qualify the complete path with CGO disabled on native Linux and repeat the six-target CLI/test compilation gate for new build tags. Record PTY/localhost restrictions and distinguish foreign compilation from native execution. The contract itself requires documentation/link/example checks; it is not runtime evidence.

## Decision boundaries

C08 is settled for this first slice: bounded polling of existing safe snapshots, with no subscription protocol, new event fields, speed/ETA, or retry counters. A later measured need can justify richer events separately.

C09 and S08 remain open for future configuration/profiles work. This TUI reads no configuration file, writes no preferences/history, and uses the already settled Linux daemon state/runtime paths. The illustrative TOML and configuration-root layout in [configuration](configuration.md) are not adopted by this plan. No configuration or profile decision blocks F1 through F4.
