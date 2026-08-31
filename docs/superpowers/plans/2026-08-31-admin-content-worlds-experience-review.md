# Admin Content Worlds Experience Review Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Resolve the dated admin content-worlds UX review, with special attention to avoiding empty drafts, making worlds discoverable, and explaining publish state in context.

**Architecture:** Keep the existing Vue router, registry, API clients, and immersive writing shell. Make new-entry creation client-first, derive list filters and page context from route metadata/query state, and pass world settings into publish/editor surfaces only where the existing API already supports the behavior.

**Tech Stack:** Vue 3, TypeScript, Vue Router, Pinia, Vitest, existing CSS tokens.

**Spec:** `docs/ux-reviews/2026-08-31-admin-content-worlds-experience-review.md`

## Global Constraints

- Preserve the existing immersive writing direction and revision-aware save behavior.
- Do not delete unrelated dirty or untracked user work.
- Keep status semantics explicit: unopened, open, and hidden are distinct.
- Keep interactive targets at least 40px where touched and verify 375px layout.
- Use existing API contracts; do not add speculative backend behavior.

### Task 1: Stop empty server drafts and clarify world creation choices

**Files:** `admin/src/views/EntryEditor.vue`, `admin/src/views/SayingEditor.vue`, `admin/src/views/NewEntry.vue`, related tests.

- [ ] Add client-first draft state for journal/video and create the server record on first meaningful edit or explicit save/publish.
- [ ] Add the same lazy-create behavior to saying editor, including a safe leave path for untouched content.
- [ ] Add structure and availability copy to world cards, and make unavailable-world publish limitations visible before editing.
- [ ] Test no POST on untouched entry/saying routes and POST on first save/input.

### Task 2: Make worlds a first-class admin information architecture

**Files:** `admin/src/layouts/AdminLayout.vue`, `admin/src/router/index.ts`, `admin/src/views/Entries.vue`, `admin/src/views/Categories.vue`, tests.

- [ ] Add 世界 to desktop and mobile navigation and route metadata.
- [ ] Replace the fixed journal list query with URL-backed all/journal/saying/video world filtering.
- [ ] Load categories for the selected world and make empty drafts identifiable.
- [ ] Show current world/status context on categories and worlds surfaces.

### Task 3: Close world-state and publish-loop gaps

**Files:** `admin/src/views/Worlds.vue`, `admin/src/components/writing/PublishPanel.vue`, `admin/src/views/EntryEditor.vue`, tests.

- [ ] Add explanations for each world status and row-level dirty/save feedback.
- [ ] Pass the relevant world setting to the publish panel and explain unopened/hidden publishing next steps.
- [ ] Restrict default-view options to the world capabilities already represented by the registry.

### Task 4: Polish dashboard, login, editors, media, and mobile density

**Files:** `admin/src/views/Dashboard.vue`, `admin/src/views/Login.vue`, `admin/src/views/SayingEditor.vue`, `admin/src/views/EntryEditor.vue`, `admin/src/components/writing/VideoUpload.vue` or existing media surface, related styles/tests.

- [ ] Unify visible admin copy in Chinese, add a real continue-writing/drafts action, and improve login focus without adding marketing decoration.
- [ ] Give editors a stable status strip, clearer empty-editor anchors, saying-link guidance, and an explicit video media slot.
- [ ] Reduce small-screen row metadata and strengthen the mobile writing action.

### Task 5: Verify the review fixes

- [ ] Run admin unit tests under Node 20.19.4+ or Node 24.
- [ ] Run typecheck/build and `git diff --check`.
- [ ] Verify desktop and 375px browser states, especially untouched new routes, world filter URLs, publish-panel status copy, and mobile drawer navigation.

