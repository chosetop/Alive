# SDD ledger — plan: docs/superpowers/plans/2026-08-26-draft-autosave-foundation.md

Workspace: /Users/p30huiwei/Desktop/Alive/.worktrees/admin-writing-experience
Branch: codex/admin-writing-experience
Branch start: b44cd05

## Baseline

- Backend: `go test -race ./...` passed.
- Admin: `npm ci && npm run build` passed with bundled Node v24.19.0.
- Frontend: `npm ci && npm run typecheck && npm run build` passed with bundled Node v24.19.0; existing `/fonts/chillkai.woff2` build warning remains.
- Controller runtime note: all Node commands must prepend `/Users/p30huiwei/.cache/codex-runtimes/codex-primary-runtime/dependencies/bin/override:/Users/p30huiwei/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/bin` to PATH.

## Preflight dependency and consistency scan

| Scope | Produces | Consumes / later touch | Finding |
|---|---|---|---|
| Task 1 → Task 2 | SQL revision column, revision-aware generated methods | Domain repository maps generated params/results | Consistent; Task 2 must use generated signatures from Task 1. |
| Task 1 → Task 3 | Owner queries expose revision and reject stale mutations | HTTP DTOs and handlers expose revision/409 | Consistent; public query shapes must stay revision-free. |
| Task 2 → Task 3 | `ErrVersionConflict`, publish validation, revision-aware service signatures | HTTP error mapping and transition bodies | Consistent. |
| Task 3 → Task 4 | `EntryPatchFields` | IndexedDB `RecoveryRecord.fields` | Consistent; Task 4 cannot precede Task 3. |
| Task 3 → Task 5 | Revision-aware entry API and `ApiClientError` | Save coordinator network contract | Consistent. |
| Task 4 → Task 5 | `EntryRecoveryStore` | Local-first save state machine | Consistent; recovery write must precede each network save. |
| Tasks 3–5 → Task 6 | API, recovery, coordinator, Vue adapter | Existing editor integration | Consistent; Task 6 must not mutate `admin/src/App.vue`. |
| Task 6 → Task 7 | Autosave and conflict UI | Browser verification scenarios | Consistent; Task 7 may only fix files owned by this plan. |
| Task 1 internal | Integration test, migration, query regeneration | Exact expected revision behavior | Consistent; test is expected to fail first at compile/schema level. |
| Task 2 internal | Draft validators and publish validator | Service/repository tests | Consistent; draft validation remains permissive only for allowed empty fields. |
| Task 3 internal | Required revision on mutations | HTTP tests and admin contracts | Consistent; create remains revision-free and always creates a draft. |
| Task 4 internal | Native IndexedDB wrapper | Recovery tests | Consistent; dependencies and DB identifiers are exact. |
| Task 5 internal | Serialized coordinator and Vue lifecycle adapter | Fake-timer and lifecycle tests | Consistent; conflict stops automatic retry. |
| Task 6 internal | Remove manual save, integrate autosave | Component tests and full build | Consistent; existing user-owned MilkdownProvider change is preserved by leaving `admin/src/App.vue` untouched. |
| Task 7 internal | Local browser scenarios and evidence | `docs/progress.md` | Consistent; use disposable local data and never record credentials. |

No plan/spec contradictions found before Task 1.

Task 1: started (base b44cd05, implementer /root/draft_schema, model gpt-5.6-terra high)
Task 1: reviewer approved spec and quality; HTTP 409 is intentionally produced by Task 3, so the cross-task warning is not a Task 1 gap.
Task 1: minor (deferred): down migration guard skips soft-deleted incomplete rows, so restoring the old all-row CHECK can fail with a less explicit error.
Task 1: complete (commits b44cd05..af93ed4, review clean)
Task 2: started (base af93ed4, implementer /root/entry_domain_revision, model gpt-5.6-terra high)
Task 2: initial review found fake-store empty-slug uniqueness mismatch.
Task 2: fix round 1/5 (1 addressed, 0 open — empty slugs excluded from fake uniqueness checks; commits 9de1834..3f0c78e)
Task 2: controller resolved integration warning with `TEST_DATABASE_URL=...alive_test go test ./internal/entry -run 'TestRepositoryUpdateReportsVersionConflict|TestRepositoryStateTransitionsReportAnAbsentEntry' -count=1` (pass).
Task 2: complete (commits af93ed4..3f0c78e, review clean)
Task 3: started (base 3f0c78e, implementer /root/revision_http_contract, model gpt-5.6-sol high)
Task 3: initial review found duplicate empty-draft creation after POST success followed by PATCH failure.
Task 3: fix round 1/5 (1 addressed, 0 open — retain created id/revision before fallible PATCH; commits 1ab4801..4d7e974)
Task 3: controller resolved cross-task warning from Task 1/2 evidence: partial slug uniqueness and atomic revision CAS are covered by migrated `alive_test` integration runs.
Task 3: complete (commits 3f0c78e..4d7e974, review clean)
Task 4: started (base 4d7e974, implementer /root/indexeddb_recovery, model gpt-5.6-terra high)
Task 4: initial review found older recovery records could overwrite newer revisions and blocked IndexedDB open could hang.
Task 4: fix round 1/5 (2 addressed, 0 open — conditional same-transaction put and blocked-open rejection with late close; commits ca65ad2..4eb71b6)
Task 4: local-write-before-network warning is intentionally consumed and tested by Task 5, not a Task 4 gap.
Task 4: complete (commits 4d7e974..4eb71b6, review clean)
Task 5: started (base 4eb71b6, implementer /root/save_coordinator, model gpt-5.6-sol high)
Task 5: implementation committed at ada6cf7; initial review found two Important issues: terminal subscriber reentrancy can strand a queued patch, and recovery `remove()` rejection can leave the coordinator stuck at `saving` / leak an unhandled rejection.
Task 5: fix round 1/5 paused before changes at user request (HEAD ada6cf7, working tree clean). Resume by re-dispatching `/root/save_coordinator` if available, otherwise a fresh implementer, with the two findings above; append fixes to `task-5-report.md`, then generate diff from ada6cf7 and run scoped re-review.
Session pause: Tasks 1-4 complete and reviewed; Task 5 implementation exists but is not review-clean; Tasks 6-7 not started.
Session resume: Task 5 fix round 1 restarted from clean ada6cf7 with fresh implementer /root/save_coordinator_fix1 (original implementer context unavailable; model gpt-5.6-sol high).
Task 5: fix round 1/5 (2 addressed, 0 open — saved-listener reentrancy and recovery cleanup rejection; commits ada6cf7..fbe629d)
Task 5: prior terminal-dispose minor also addressed; public methods are inert after dispose while dispose itself can flush pending work.
Task 5: complete (commits 4eb71b6..fbe629d, review clean)
Task 6: started (base fbe629d, implementer /root/integrate_autosave_editor, model gpt-5.6-sol high)
Task 6: initial implementation committed at 32f665c; task review found 1 Critical and 4 Important issues: conflict edits not persisted, stale async load can bind after navigation/unmount, delete can collide with leave flush, failed recovery-draft PATCH leaks duplicate empty drafts, and the required local-overwrite conflict action is missing.
Task 6: fix round 1/5 paused mid-implementation at user request. HEAD remains 32f665c with uncommitted changes in `admin/src/editor/save-coordinator.ts`, `admin/src/editor/save-coordinator.test.ts`, `admin/src/views/EntryEditor.vue`, and `admin/src/views/EntryEditor.test.ts` (407 insertions, 24 deletions; `git diff --check` clean).
Task 6: fix progress before pause: findings 1-4 reached GREEN with `EntryEditor.test.ts` 16/16; finding 5 local-overwrite tests were added and were at expected RED (18 tests total, 3 failing because the action was not implemented yet). Report fix section has not yet been appended; no fix commit exists.
Task 6: resume instructions: re-dispatch `/root/integrate_autosave_editor` if available, preserve the existing WIP, complete finding 5 with GREEN, rerun focused/full admin tests, build, backend `make check`, append fix round 1 evidence to `task-6-report.md`, commit, generate review package from 32f665c, then run scoped re-review of all five findings.
Session pause 2: Tasks 1-5 complete and reviewed; Task 6 initial commit exists and fix round 1 is mid-WIP; Task 7 not started.
Session resume 2: original Task 6 implementer was unavailable and thread slot briefly blocked; fresh implementer /root/integrate_editor_fix1_resume has now taken over the preserved WIP to finish finding 5 and the full fix-round verification.
Task 6: fix round 1/5 review result: findings 1, 2, and 4 addressed; findings 3 and 5 remained open, plus a new Important taxonomy/create `Promise.all` orphan-draft race.
Task 6: fix round 2/5 in progress on /root/integrate_editor_fix1_resume (delete-in-flight input guard, await conflict local persistence before recovery actions, and create-before-taxonomy-failure handling).
Task 6: fix round 2/5 committed c9109f7; scoped re-review addressed all three prior findings but found a new Important load-transition window where old entry remains editable during slow route loads.
Task 6: fix round 3/5 in progress on /root/integrate_editor_fix1_resume (lock/show loading during route loads and add delayed-switch regression tests).
Task 6: fix round 3/5 committed cb63d23; scoped re-review addressed the loading race but found a new Important route-reuse bug where returning to entry A after entry B load failure leaves B's error page stuck.
Task 6: fix round 4/5 in progress on fresh /root/editor_route_error_fix4, model gpt-5.6-sol xhigh, to clear load error and reload/reveal the prior entry on route return with regression coverage.
Task 6: fix round 4/5 committed 8b0a434; scoped re-review addressed route-reuse error recovery with no new Critical/Important breakage.
Task 6: complete (commits fbe629d..8b0a434, review clean)
Task 7: started (base 8b0a434, implementer /root/browser_acceptance, model gpt-5.6-terra high)
Task 7: browser retry by controller — backend and admin shell checks returned 200 after clean restart; in-app browser still returned ERR_CONNECTION_REFUSED for localhost/127.0.0.1, so UI acceptance remains unverified and blocked by browser runtime isolation.
