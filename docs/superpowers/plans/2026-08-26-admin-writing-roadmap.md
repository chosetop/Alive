# Alive Admin Writing Experience Roadmap

> **For agentic workers:** Execute the linked plans in order. Each plan has its own test and review gate. Do not start a later plan while an earlier plan is failing.

**Goal:** Deliver the approved personal writing workspace without combining four independently reviewable subsystems into one unsafe change.

**Spec:** `docs/superpowers/specs/2026-08-26-admin-writing-experience-design.md`

## Execution order

1. [Drafts, revisions, recovery, and autosave](2026-08-26-draft-autosave-foundation.md)
2. [Immersive writing workspace](2026-08-26-immersive-writing-workspace.md)
3. [Shared site themes](2026-08-26-shared-site-themes.md)
4. [Admin iOS visual redesign](2026-08-27-admin-ios-visual-redesign.md)
5. [Aliyun OSS media upload](2026-08-26-media-upload.md)

## Cross-plan contracts

- Plan 1 adds `revision: number` to every owner-side entry response and requires `revision` on entry updates and state transitions.
- Plan 1 produces `createSaveCoordinator` and `EntryRecoveryStore`; Plan 2 owns their final workspace integration.
- Plan 2 produces the writing shell, settings panel, publish panel, and editor extension points consumed by Plans 3 and 4.
- Plan 3 produces `@alive/theme`, the shared theme manifest, site-default API, admin preview store, and frontend visitor-preference resolution.
- Plan 4 produces the admin iOS visual language, extended semantic theme variables, optional 夜航 theme, icon boundary, and responsive shell/page restyling consumed by Plan 5.
- Plan 5 produces presigned Aliyun OSS upload, media registration, current-entry media reuse, and Milkdown image insertion.

## Gate between plans

At the end of every plan:

```bash
cd backend && make check
cd ../admin && npm test -- --run && npm run build
cd ../frontend && npm run typecheck && npm run build
```

Run only the commands for projects changed by that plan. Before starting the next plan, inspect the real UI in a browser at desktop width and 375px when the plan changes rendering.

## Spec coverage

| Design requirement | Owning plan |
|---|---|
| Incomplete drafts, revision conflicts, local recovery, autosave | Plan 1 |
| Article directory, centered canvas, contextual controls | Plan 2 |
| Settings drawer, preview, publish blockers and reminders | Plan 2 |
| Desktop, iPad, mobile drawers, keyboard and focus | Plan 2 |
| Shared semantic tokens, admin preview, site default, visitor override | Plan 3 |
| Paper, lamp, and Codex Lavender with B prose purple | Plan 3 |
| Admin iOS visual language, theme extensions, responsive shell and page restyling | Plan 4 |
| Paste, drag, upload progress, retry, article reuse, cover | Plan 5 |
| Alt text, caption, normal/wide figures, published rendering | Plan 5 |
| No scheduling, collaboration, full media library, or batch deletion | Enforced by every plan's global constraints |
