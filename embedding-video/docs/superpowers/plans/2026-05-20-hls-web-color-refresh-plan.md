# HLS Web Color Refresh Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the current dark tech-style `hls-web` frontend theme with a bright, youthful, school-appropriate light theme while preserving structure and behavior.

**Architecture:** The implementation stays inside the existing styling model: one global stylesheet loaded by `src/main.js` and applied to the `App.vue` UI. The change is a visual refactor centered on semantic color variables in `src/style.css`, followed by targeted updates to hard-coded surface, button, input, status, and chart colors so the whole page reads as a single light-theme system.

**Tech Stack:** Vue 3, Vite, plain CSS

---

## File Map

- Modify: `hls-web/src/style.css`
  - Holds the full global theme, layout, and component styling for the page.
- Reference only: `hls-web/src/main.js`
  - Confirms that `style.css` is the global style entry.
- Reference only: `hls-web/src/App.vue`
  - Confirms structure and the UI states affected by the theme.

## Task 1: Rebuild Global Theme Tokens

**Files:**
- Modify: `hls-web/src/style.css`
- Reference: `hls-web/src/main.js`

- [ ] **Step 1: Confirm the global style entry before editing**

Read `hls-web/src/main.js` and verify it still contains:

```js
import { createApp } from 'vue'
import './style.css'
import App from './App.vue'

createApp(App).mount('#app')
```

This confirms the theme work should remain in `hls-web/src/style.css` only.

- [ ] **Step 2: Replace the root color system with the approved light palette**

In `hls-web/src/style.css`, replace the existing dark-theme `:root` block with a light-theme token set using the approved palette:

```css
:root {
  font-family: 'Trebuchet MS', 'Segoe UI', 'PingFang SC', 'Microsoft YaHei', sans-serif;
  line-height: 1.5;
  font-weight: 400;
  color: #1f2a44;
  background:
    radial-gradient(circle at top left, rgba(96, 165, 250, 0.2), transparent 30%),
    radial-gradient(circle at top right, rgba(52, 211, 153, 0.16), transparent 24%),
    radial-gradient(circle at 50% 0%, rgba(255, 138, 101, 0.1), transparent 20%),
    linear-gradient(180deg, #f8fbff 0%, #f3f8ff 45%, #eef5ff 100%);
  color-scheme: light;
  font-synthesis: none;
  text-rendering: optimizeLegibility;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;

  --bg-page: #f6f9ff;
  --bg-panel: rgba(255, 255, 255, 0.9);
  --bg-panel-strong: rgba(255, 255, 255, 0.98);
  --bg-field: rgba(248, 251, 255, 0.98);
  --bg-muted: rgba(59, 130, 246, 0.07);
  --line-soft: rgba(96, 122, 168, 0.16);
  --line-strong: rgba(59, 130, 246, 0.24);
  --text-main: #1f2a44;
  --text-muted: #60708f;
  --accent: #3b82f6;
  --accent-strong: #60a5fa;
  --accent-green: #34d399;
  --danger: #ef6b81;
  --warning: #f5c451;
  --accent-peach: #ff8a65;
  --shadow-panel: 0 22px 48px rgba(74, 104, 162, 0.12);
  --shadow-soft: 0 10px 24px rgba(92, 122, 176, 0.1);
}
```

- [ ] **Step 3: Update the page background to match the light theme**

Keep the existing `body` rule structure, but ensure the background uses the new root variables and remains fixed:

```css
body {
  margin: 0;
  min-height: 100vh;
  background: var(--bg-page);
  color: var(--text-main);
  background-attachment: fixed;
}
```

- [ ] **Step 4: Verify the stylesheet still parses and the app still builds**

Run: `npm run build`

Expected: Vite build completes successfully and emits `dist/` output without CSS parse errors.

- [ ] **Step 5: Commit the token reset**

```bash
git add hls-web/src/style.css
git commit -m "feat: refresh hls-web theme tokens"
```

## Task 2: Convert Hero and Surface Layers to a Light Visual System

**Files:**
- Modify: `hls-web/src/style.css`

- [ ] **Step 1: Restyle the hero shell from dark glass to bright feature card**

Update the `.hero-shell` rule to a light-card treatment:

```css
.hero-shell {
  position: relative;
  overflow: hidden;
  border: 1px solid var(--line-soft);
  border-radius: 34px;
  background:
    radial-gradient(circle at top left, rgba(96, 165, 250, 0.18), transparent 34%),
    radial-gradient(circle at bottom right, rgba(52, 211, 153, 0.14), transparent 30%),
    linear-gradient(180deg, rgba(255, 255, 255, 0.98), rgba(246, 250, 255, 0.95));
  box-shadow: var(--shadow-panel);
  padding: 30px 30px 24px;
  display: grid;
  gap: 20px;
}
```

- [ ] **Step 2: Lighten the decorative grid and bubbles**

Update the hero overlay and bubble colors so they support the new palette:

```css
.hero-shell::after {
  content: '';
  position: absolute;
  inset: 0;
  background-image:
    linear-gradient(rgba(59, 130, 246, 0.04) 1px, transparent 1px),
    linear-gradient(90deg, rgba(59, 130, 246, 0.04) 1px, transparent 1px);
  background-size: 18px 18px;
  pointer-events: none;
  mask-image: linear-gradient(180deg, rgba(0, 0, 0, 0.72), transparent 100%);
}

.hero-bubble-a {
  background: radial-gradient(circle, rgba(96, 165, 250, 0.3), rgba(96, 165, 250, 0.02));
}

.hero-bubble-b {
  background: radial-gradient(circle, rgba(52, 211, 153, 0.24), rgba(52, 211, 153, 0.02));
}

.hero-bubble-c {
  background: radial-gradient(circle, rgba(255, 138, 101, 0.18), rgba(255, 138, 101, 0.02));
}
```

- [ ] **Step 3: Convert shared panels from dark translucent blocks to white cards**

Ensure the shared `.panel` rule uses a white-card style:

```css
.panel {
  border: 1px solid var(--line-soft);
  border-radius: 30px;
  background: var(--bg-panel);
  box-shadow: var(--shadow-soft);
  backdrop-filter: blur(14px);
  padding: 22px;
  display: grid;
  gap: 18px;
}
```

- [ ] **Step 4: Rebuild any remaining dark section surfaces in the file**

Search `hls-web/src/style.css` for these dark remnants and convert them to the new white/soft-blue surface family:

```text
rgba(17, 26, 52, 0.9)
rgba(13, 21, 43, 0.78)
rgba(18, 28, 55, 0.88)
rgba(13, 21, 44, 0.78)
rgba(4, 8, 18, 0.5)
```

Replace them with light-theme equivalents, for example:

```css
background: linear-gradient(180deg, rgba(255, 255, 255, 0.94), rgba(244, 249, 255, 0.9));
```

or

```css
background: rgba(238, 245, 255, 0.72);
```

Choose the lighter option that preserves hierarchy without reintroducing dark panels.

- [ ] **Step 5: Run build verification again**

Run: `npm run build`

Expected: successful build and no broken CSS from the hero/panel refactor.

- [ ] **Step 6: Commit the surface refresh**

```bash
git add hls-web/src/style.css
git commit -m "feat: lighten hls-web surfaces"
```

## Task 3: Recolor Interactive Controls and Status States

**Files:**
- Modify: `hls-web/src/style.css`

- [ ] **Step 1: Restyle form labels and fields for light backgrounds**

Update label and field rules to match the new contrast model:

```css
.field-label {
  font-size: 13px;
  font-weight: 700;
  color: #35507a;
  letter-spacing: 0.02em;
}

.field {
  width: 100%;
  border: 1px solid var(--line-soft);
  border-radius: 18px;
  background: var(--bg-field);
  color: var(--text-main);
  padding: 14px 16px;
  outline: none;
  transition: border-color 120ms ease, box-shadow 120ms ease, background 120ms ease;
}

.field:focus {
  border-color: rgba(59, 130, 246, 0.55);
  box-shadow: 0 0 0 4px rgba(96, 165, 250, 0.18);
}
```

- [ ] **Step 2: Rebuild primary, secondary, ghost, and danger button colors**

Update the button rules so the main action is bright and the rest stay consistent:

```css
.primary-btn {
  background: linear-gradient(135deg, #3b82f6, #60a5fa 60%, #8bd3ff 120%);
  color: #ffffff;
  font-weight: 700;
  box-shadow: 0 12px 24px rgba(59, 130, 246, 0.22);
}

.secondary-btn,
.ghost-link,
.tiny-btn.ghost {
  background: rgba(255, 255, 255, 0.8);
  color: var(--text-main);
  border-color: var(--line-soft);
}

.danger-btn {
  background: rgba(239, 107, 129, 0.12);
  color: #c44763;
  border-color: rgba(239, 107, 129, 0.2);
}
```

- [ ] **Step 3: Update hover, active, and focus-visible affordances**

Preserve the existing interaction rules, but ensure any hard-coded dark hover shadows or yellow focus outlines match the new theme. Replace patterns like these:

```text
rgba(7, 11, 28, 0.18)
rgba(255, 211, 110, 0.6)
```

with light-theme values such as:

```css
box-shadow: 0 10px 20px rgba(59, 130, 246, 0.14);
outline: 3px solid rgba(96, 165, 250, 0.35);
```

- [ ] **Step 4: Recolor status ribbons, badges, warnings, and progress bars**

Search for success, warning, and error colors in `style.css` and migrate them into the approved palette family:

```text
#d6ffec
rgba(41, 196, 137, 0.35)
rgba(41, 196, 137, 0.12)
#ffe1e6
rgba(255, 107, 122, 0.35)
rgba(255, 107, 122, 0.12)
#ffe9b3
rgba(246, 199, 96, 0.22)
rgba(246, 199, 96, 0.1)
linear-gradient(90deg, #7d8cff, #59d6a3, #ffd36e)
```

Use these target semantics:

```css
.status-ribbon.ok {
  color: #157a5d;
  border-color: rgba(52, 211, 153, 0.28);
  background: rgba(52, 211, 153, 0.12);
}

.status-ribbon.bad {
  color: #be4e63;
  border-color: rgba(239, 107, 129, 0.24);
  background: rgba(239, 107, 129, 0.12);
}
```

and for active progress/highlight bars:

```css
background: linear-gradient(90deg, #3b82f6, #34d399, #ff8a65);
```

- [ ] **Step 5: Restyle any remaining hard-coded text colors that assume a dark theme**

Search `hls-web/src/style.css` for these dark-theme text leftovers and convert them to the new type palette:

```text
#dce6ff
#dfe9ff
#ffd9df
#ffb3be
```

Replace them with values derived from:

```css
color: var(--text-main);
color: var(--text-muted);
color: #c44763;
color: #35507a;
```

- [ ] **Step 6: Run build verification**

Run: `npm run build`

Expected: successful build with the updated control and status styling.

- [ ] **Step 7: Commit the interactive/state refresh**

```bash
git add hls-web/src/style.css
git commit -m "feat: update hls-web control colors"
```

## Task 4: Final Theme Consistency Pass and Visual Validation

**Files:**
- Modify: `hls-web/src/style.css`

- [ ] **Step 1: Scan for old theme colors that should no longer remain**

Run the search below and inspect each result:

```bash
rg "#0f1631|#131d3d|#0b1228|#7d8cff|#a98cff|#ffd36e|rgba\(18, 28, 57|rgba\(15, 24, 50|rgba\(255, 255, 255, 0\.08\)|rgba\(255, 255, 255, 0\.09\)" hls-web/src/style.css
```

Expected: either no results, or only intentional survivors that still fit the new theme and have been consciously reviewed.

- [ ] **Step 2: Normalize any leftover mismatched styles**

If the search reveals old deep-blue or old neon-purple values in buttons, cards, tags, progress bars, or chart lines, replace them with the approved palette:

```text
Primary blue: #3B82F6
Blue highlight: #60A5FA
Mint green: #34D399
Coral accent: #FF8A65
Main text: #1F2A44
Muted text: #60708F
```

- [ ] **Step 3: Run production build**

Run: `npm run build`

Expected: successful Vite production build.

- [ ] **Step 4: Run local preview for manual inspection**

Run: `npm run dev`

Then inspect the page in a browser and confirm:

```text
1. The page background is light and no longer reads as a dark operations console.
2. Hero, cards, and forms use one consistent light-surface system.
3. Primary buttons are bright blue and visually dominant.
4. Success, warning, and error states are distinguishable on light backgrounds.
5. Text remains readable across hero, cards, inputs, and badges.
```

- [ ] **Step 5: Commit the finished color refresh**

```bash
git add hls-web/src/style.css
git commit -m "feat: apply youthful color refresh to hls-web"
```

## Self-Review

- Spec coverage check:
  - Light theme direction: covered by Tasks 1 and 2.
  - Approved palette application: covered by Tasks 1 and 3.
  - Background, hero, panel, input, button, status, and progress recoloring: covered by Tasks 2 and 3.
  - No business-logic changes: respected by only touching `hls-web/src/style.css`.
  - Build verification: covered by every task and finalized in Task 4.
- Placeholder scan: no `TBD`, `TODO`, or undefined “appropriate handling” steps remain.
- Type and path consistency: all tasks target the existing file `hls-web/src/style.css`; validation commands use the `hls-web` Vite project context.
