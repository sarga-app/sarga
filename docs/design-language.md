# Sarga — Design Language

> *A single-binary music server for people who own their library. The UI is the listening room.*

- [Sarga — Design Language](#sarga--design-language)
  - [The vibe](#the-vibe)
  - [Anti-references](#anti-references)
  - [Surfaces — four, paired by mode and role](#surfaces--four-paired-by-mode-and-role)
    - [Surface scales — same direction across modes](#surface-scales--same-direction-across-modes)
  - [Text — one family per surface](#text--one-family-per-surface)
  - [Type — three families, three jobs](#type--three-families-three-jobs)
  - [Color — restraint as a feature](#color--restraint-as-a-feature)
  - [Album influence — the one chrome rule that bends](#album-influence--the-one-chrome-rule-that-bends)
  - [Layout — structural sidebar, layered floating chrome](#layout--structural-sidebar-layered-floating-chrome)
  - [Components](#components)
  - [Motion — one curve, three durations](#motion--one-curve-three-durations)
  - [Voice](#voice)
  - [Iconography](#iconography)
  - [What ships](#what-ships)


## The vibe

Editorial, not corporate. Operator-grade, not toy. Reverent of music, not loud about it. Late-night, vinyl-crate, cassette-case — physical media of music as the visual anchor. A magazine masthead designed by someone who runs Logic Pro and reads Pitchfork.

Tight density, generous typography. The serif breathes for the music; the mono labels do the operator work. Album art is the only color the user ever supplied — chrome stays out of the way.

If it looks like a SaaS dashboard, an AI tool, or a Material You demo, throw it out.

---

## Anti-references

| Don't | Why |
|---|---|
| Rounded album covers | Records are square. Album art is square. |
| Drop shadows on dark surfaces | Invisible noise. Use hairlines + bg lift. |
| Pure `#FFF` or pure `#000` | Both feel cheap. Both fatigue the eye. |
| Entrance animations on page load | A magazine doesn't fade in. Things appear. |
| Saturated red on buttons | Vinyl-red is for accents only. Buttons are ink-black. |

---

## Surfaces — four, paired by mode and role

| Role | Light mode | Dark mode |
|---|---|---|
| Browse / general UI | **Paper** (cool off-white) | **Nocturne** (neutral near-black) |
| Listening room (Now Playing) | **Lacquer** (near-black, violet undertone) | **Vellum** (clean off-white, faint warmth) |

The listening-room surface inverts brightness from the general UI. Light-mode browse on Paper → tap Now Playing → enter Lacquer (dark, intimate). Dark-mode browse on Nocturne → tap Now Playing → enter Vellum (light, contrasting). The room is always *different* from where you came.

### Surface scales — same direction across modes

Chrome (sidebar, topbar) is one stop *darker* than body — in both modes. "Elevation" is `bg-1` lift + hairlines, never shadow.

```css
/* Paper */
--paper-0: #FCFCFD;  /* body */
--paper-1: #F4F5F7;  /* chrome — darker than body */
--paper-2: #ECEEF1;  /* hover */
--paper-3: #DFE2E7;  /* pressed */

/* Nocturne — same direction: chrome darker than body */
--nocturne-0: #181818;  /* body */
--nocturne-1: #0F0F0F;  /* chrome */
--nocturne-2: #080808;  /* hover */
--nocturne-3: #030303;  /* pressed */

/* Lacquer — light-mode now-playing room ONLY */
--lacquer-0: #0F0E13;
--lacquer-1: #16151B;
--lacquer-2: #1F1D26;
--lacquer-3: #2A2734;

/* Vellum — dark-mode now-playing room ONLY. Off-white, faint warmth, not cream */
--vellum-0: #ECECEA;
--vellum-1: #E2E2DF;
--vellum-2: #D5D5D1;
--vellum-3: #C4C4BF;
```

**Lacquer and Vellum are reserved.** Now Playing surface and player bar only. Using them anywhere else creates two competing dark themes.

---

## Text — one family per surface

| Surface | Text | Character |
|---|---|---|
| Paper | **Ink** `#15161A` | Cool dark with hint of blue |
| Nocturne | **Cream** `#F5F5F5` | Neutral whites |
| Lacquer | **Bone** `#F0EDE5` | Warm whites (warm-on-cool atmospheric harmony) |
| Vellum | **Umber** `#1C1610` | Sepia darks (warm-on-warm liner-notes feel) |

Never `#FFF`. Never `#000`. Each surface gets its own text family because the surface IS the room — its tonal character demands its own ink.

---

## Type — three families, three jobs

| Family | Used for |
|---|---|
| **IBM Plex Sans** | UI chrome — buttons, labels, body, nav |
| **Newsreader** (serif) | Editorial display — album titles, hero, H2 section heads, prose |
| **IBM Plex Mono** | Metadata — eyebrows, chips, IDs, paths, format specs, durations |

All three are SIL-OFL. These *are* the Sarga typefaces — no proprietary brand fonts to swap in.

- Display sizes go large on editorial moments — 64–96px, Newsreader 500, letter-spacing tightened (-0.015 to -0.02em).
- UI body is **15px** — not 14, not 16.
- Eyebrows are uppercase mono with tracked-out letter-spacing — typographic device, not shouting.
- Tabular numerals (`font-variant-numeric: tabular-nums`) on tracklists, durations, counts.

---

## Color — restraint as a feature

**Accent — used sparingly, one at a time:**

```css
--accent-vinyl: #B0301A;  /* light surfaces */
--accent-ember: #C8501F;  /* dark surfaces (Lacquer/Nocturne) — warmer for contrast */
--danger-fg:    #9F2C1F;  /* destructive-button fill */
```

> *"Not crimson, not scarlet, not blood. The color of a record sleeve that's been on the shelf for fifty years."*

The accent appears *only* for: brand stripes, the 2px left-bar active-state indicator, focus rings, scrubber fill. **Never as a button background.** Primary buttons are ink-black. The only saturated red rectangle in the system is the destructive-action button — meaning "red" unambiguously means "this deletes things."

**Status — same on every surface:**

```css
--success: #2F7B57;
--warning: #B97615;
--danger:  #B23B2D;
--info:    #2A5A8C;
```

**Hairlines** are alpha-on-color, never solid grays — they tint with whatever surface is underneath.

```css
--hair-on-paper:    rgba(21, 22, 26, 0.07);
--hair-on-nocturne: rgba(242, 239, 231, 0.08);
--hair-on-lacquer:  rgba(240, 237, 229, 0.10);
--hair-on-vellum:   rgba(40, 28, 14, 0.10);
```

Two strengths per surface: `--border` (default ~7-10%), `--border-strong` (~13-18%).

---

## Album influence — the one chrome rule that bends

The album palette (extracted server-side at scan time) drives four CSS variables while a track plays:

```css
--album-c1      /* most-saturated mid-tone */
--album-mood    /* full-image average */
--album-deep    /* darkened mood, near-black album tint */
--album-accent  /* same as c1, used for scrubber fill */
```

**Two intensities — opacity-identical in light and dark mode:**

| | Body overlay | Player overlay |
|---|---|---|
| **Wash** (flat) | `--album-mood` flat tint, 12% opacity, whole viewport. Material-You-style even tint. | Flat mood tint, 30% opacity over Lacquer/Vellum. |
| **Bloom** (gradient) | Vertical linear: mood at top → transparent middle → deep at bottom. 28% opacity. Frames the page top-and-bottom. | Vertical mood→deep gradient, 55% opacity. |

**Forbidden patterns:**
- Radial gradient centered at bottom-center — reads as a weird halo from the player.
- Mode-specific opacities or `mix-blend-mode` switches that make light-mode and dark-mode feel different.

**Now Playing room is heavier, intentionally.** The atmospheric background is a composed set of three radial gradients in `c1 / mood / deep` at distinct positions (top-left, bottom-right, center) forming an organic color field. Edge vignette focuses center. The album dominates — it's why you opened the room.

---

## Layout — structural sidebar, layered floating chrome

| Element | Treatment |
|---|---|
| **Sidebar** (left, 232px) | Structural column. Full height (`top: 0; bottom: 0`). `bg-1` fill, hairline right border. Brand wordmark (Newsreader 30px, vinyl-bar accent) at top, aligned with nav-item padding. Internal content has `padding-bottom: player-h + player-gap + 16px` so nothing scrolls behind the player. |
| **Topbar** (inside main column, 56px) | Floating card. Starts at `left: sidebar-w`. `bg-1` fill, hairline border, `--r-sheet` radius. Search (flex-1, max 480px) left; notifications, settings, avatar right. |
| **Player** (bottom, 76px, full width) | Floating card. `left/right: 16px; bottom: 16px; border-radius: 10px`. Layers visually over the sidebar's bottom-right area. Light mode = Lacquer surface; dark mode = Vellum (inverted contrast in both modes — the player is always "the other surface"). |
| **Now Playing** (overlay, z-index above everything) | The atmospheric room. Lacquer in light, Vellum in dark. Composed album-palette radials fill the background; album cover (square, ~460px) left; 72px Newsreader track title + 24px italic artist + technical chips right. No margins. Cinema-mode immersive. |

**Clash resolution:** sidebar is *structural*; topbar and player are *floating chrome* within the main column. They don't share design language — sidebar is the panel, topbar/player are the cards. Stop fighting it.

---

## Components

**Album art** — always square (`--r-art: 0`). Hairline border, no shadow at rest. Hover lifts 1px and deepens border. The card *is* the artwork; no chrome around it.

**Buttons:**
- **Primary**: `bg: var(--ink-1)`, paper text. Subtle inset highlight + bottom shadow (pressed-nameplate feel). Hover/press darken 4%. **No scale-down** — operators click fast.
- **Secondary**: surface fill, hairline border.
- **Ghost**: transparent, hover → `bg-2`.
- **Danger**: `bg: var(--danger-fg)`. Only saturated red rectangle in the system.

**Inputs** — 32px tall, `bg: var(--bg)`, hairline, 6px radius. Focus = `border-color: var(--fg-3)`. No outline glow.

**Active nav** — 2px left bar in `--accent`, color from `fg-2` to `fg-1`, weight 600. Background unchanged.

**Hairlines, always**. Cards / dividers / inputs share one 1px hairline weight. No double borders. No `box-shadow: inset 0 0 0 1px` plus `border` — pick one.

**Radii — three, never more:**
- `--r-input: 6px` (buttons + inputs share)
- `--r-card: 8px`
- `--r-sheet: 12px` (modals, floating topbar)

**Shadows** — `--shadow-rest` (subtle 1px), `--shadow-float` (8/24 for menus). Both tuned for paper. Effectively invisible on dark — don't rely on them; use `bg-1` + hairlines for dark-mode elevation.

**Vinyl-groove texture** — single permitted skeuomorphism. `repeating-radial-gradient(circle, transparent, rgba(0,0,0,0.06), transparent)` overlay on album art only. Subtle enough to feel like a printing artifact, not a Photoshop layer.

---

## Motion — one curve, three durations

```css
--ease:       cubic-bezier(0.2, 0, 0, 1);  /* slightly snappy in, slow out */
--dur-micro:  120ms;  /* hovers, button states */
--dur-ui:     200ms;  /* page chrome, modals */
--dur-route:  320ms;  /* route transitions, album switches */
```

**Crossfades, not slides.** Album art crossfades between tracks. Page transitions are opacity, never translate. **No bounces, no springs, no entrance animations.**

---

## Voice

Plain, confident, slightly literary. Sarga is a personal project written by someone who cares about music. Not corporate, not chirpy, not winking.

- **Sentence case** everywhere — headings, labels, buttons. Never Title Case.
- **Eyebrows are uppercase** *only* because they're tracked-out mono — typographic device.
- **First person** in long-form docs ("I'm building this..."). **Second person** in tooltips, errors, empty states ("Couldn't read this folder.").
- **Specific over general.** "12,000 tracks indexed in 4.2 seconds" beats "Lightning-fast indexing!"

| Surface | Pattern | Example |
|---|---|---|
| Empty state | What's missing + one action | "No libraries yet. **Add a folder**." |
| Loading | Concrete, not "Loading…" | "Indexing 412 of 12,008 tracks." |
| Error | What broke + what to do | "Couldn't read `/mnt/music`. Check the path and permissions." |
| Confirmation | Past tense, no exclamation | "Library added." |
| Section eyebrow | One or two words, tracked out | "RECENTLY ADDED", "OPERATOR", "SECURITY" |

**No emoji in product chrome.** No `♪ ♫` glyphs — they look stamped-on. The **Devanagari double-bar `॥`** is the *only* unicode glyph used — the sarga mark, brand element, used in the wordmark and as section divider.

---

## Iconography

**Lucide.** 1.5px stroke weight — never 2px (too heavy against our type), never 1px (too wispy). Icons inherit `currentColor`. Active and primary states get `--accent`. Destructive actions get `--danger`. Never multi-color, never decorative.

---

## What ships

Anything that ships ought to read like it was made by someone who runs the music server it's playing on. Editorial, dense, restrained. The next person who looks at it should think *"this person cared"* — not *"this looks like every other app."*

If you're about to add a glow, a gradient on chrome, a rounded album cover, a saturated button fill, or a "modern" entrance animation — stop, and read this file again.
