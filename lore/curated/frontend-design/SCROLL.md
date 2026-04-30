---
id: frontend-design
version: 1.0.0
team: frontend
stack: React, Next.js, HTML/CSS, TypeScript, Tailwind CSS, Motion
last_updated: 2026-04-30
---

# Scroll: Frontend Design — Production-Grade UI

## Triggers — load when:
- Files: `*.tsx`, `*.css`, `*.scss`, `tailwind.config.*`, `app/layout.tsx`, `components/**`
- Keywords: UI, design, component, landing page, dashboard, layout, typography, animation, theme, aesthetic, dark mode
- Tasks: building a UI component, designing a page, creating a landing page, styling a dashboard, writing CSS

## Context
Production-grade frontend code is visually distinctive, not generic. AI-generated interfaces default to safe, predictable choices: Inter font, purple gradients, standard card layouts. These choices look like every other AI-generated UI. The goal is interfaces that feel genuinely designed — committed to a specific aesthetic, executed with precision. Generic is not neutral: it reads as low-effort. Every design decision communicates intent.

---

## Rules

### 1. Commit to a bold aesthetic direction before writing code

Before touching a component, define the aesthetic. Pick one direction and execute it fully. Hedging between styles produces mediocre output.

**Aesthetic directions to choose from:**
```
Brutally minimal    — near-zero decoration, maximum whitespace, monospace
Maximalist          — dense layering, multiple typefaces, rich texture
Retro-futuristic    — grid patterns, neon, scanlines, CRT effects
Organic/natural     — curved shapes, earthy palettes, hand-drawn elements
Luxury/refined      — tight kerning, generous spacing, muted tones
Playful/toy-like    — bold colors, rounded corners, exaggerated scale
Editorial/magazine  — asymmetry, drop caps, full-bleed images
Brutalist/raw       — visible structure, unstyled elements used deliberately
Art deco/geometric  — symmetry, gold accents, angular motifs
Industrial          — concrete textures, utility typefaces, no ornament
```

Document the choice before writing code:
```typescript
// Aesthetic: Editorial/Magazine
// Palette: Off-white (#F5F0E8), ink (#1A1A1A), accent red (#D4371C)
// Typography: display → "Playfair Display", body → "Source Serif 4"
// Motion: staggered reveal on scroll, no hover micro-interactions
```

### 2. Typography: distinctive, not default

Never use Inter, Roboto, Arial, or system fonts as the primary typeface. These communicate "I used the default." Choose fonts with character.

```typescript
// ❌ BAD — generic
fontFamily: { sans: ['Inter', 'system-ui'] }

// ✅ GOOD — editorial aesthetic
fontFamily: {
  display: ['"Playfair Display"', 'Georgia', 'serif'],
  body:    ['"Source Serif 4"', 'Palatino', 'serif'],
  mono:    ['"JetBrains Mono"', 'monospace'],
}
```

**Pairing principle:** One display font (headlines, hero text) + one body font (paragraphs, UI text). They should contrast in personality, not just weight.

**Good source:** [Google Fonts](https://fonts.google.com) — filter by category, not popularity.

### 3. Color: dominant + sharp accent

Timid, evenly-distributed palettes read as uncommitted. A dominant neutral + one sharp accent creates visual hierarchy.

```css
/* ✅ GOOD — dominant with accent */
:root {
  --bg:      #0A0A0A;    /* dominant: near-black */
  --surface: #141414;   /* subtle elevation */
  --text:    #F0F0F0;   /* primary text */
  --muted:   #666666;   /* secondary text */
  --accent:  #FF4500;   /* single accent, used sparingly */
}

/* ❌ BAD — equally weighted rainbow */
:root {
  --blue:   #4F46E5;
  --green:  #10B981;
  --yellow: #F59E0B;
  --red:    #EF4444;
  /* no hierarchy, no dominant tone */
}
```

### 4. Layout: break the grid intentionally

Symmetric grid layouts read as generic. Introduce asymmetry, overlap, and diagonal flow at key moments.

```css
/* ✅ GOOD — asymmetric hero with intentional tension */
.hero {
  display: grid;
  grid-template-columns: 1fr 1.6fr;
  gap: 0;
}

.hero-text {
  padding: 8rem 4rem;
  position: relative;
  z-index: 2;
}

.hero-image {
  margin-top: -4rem;  /* intentional overlap */
  clip-path: polygon(8% 0, 100% 0, 100% 100%, 0 100%); /* diagonal cut */
}
```

### 5. Motion: high-impact, not scattered

One well-orchestrated entrance animation creates more impact than scattered micro-interactions.

```typescript
// ✅ GOOD — staggered page load with Motion (React)
import { motion } from 'motion/react';

export function HeroSection() {
  return (
    <motion.div
      initial={{ opacity: 0, y: 32 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.6, ease: [0.22, 1, 0.36, 1] }}
    >
      <motion.h1
        initial={{ opacity: 0, y: 24 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ delay: 0.1, duration: 0.5 }}
      >
        {headline}
      </motion.h1>
      <motion.p
        initial={{ opacity: 0 }}
        animate={{ opacity: 1 }}
        transition={{ delay: 0.3, duration: 0.5 }}
      >
        {subheadline}
      </motion.p>
    </motion.div>
  );
}
```

For HTML/CSS only — use `animation-delay` for stagger:

```css
.hero-title   { animation: fadeUp 0.6s ease forwards; }
.hero-sub     { animation: fadeUp 0.6s ease 0.15s forwards; opacity: 0; }
.hero-cta     { animation: fadeUp 0.6s ease 0.30s forwards; opacity: 0; }

@keyframes fadeUp {
  from { opacity: 0; transform: translateY(20px); }
  to   { opacity: 1; transform: translateY(0); }
}
```

### 6. Backgrounds: atmosphere over solid color

Solid backgrounds read as unfinished. Add depth with texture, gradient, or geometry — matched to the aesthetic direction.

```css
/* Noise texture (works for brutalist, industrial, editorial) */
.bg-textured {
  background-color: #F5F0E8;
  background-image: url("data:image/svg+xml,..."); /* SVG noise */
  background-blend-mode: multiply;
}

/* Gradient mesh (works for modern, tech, luxury) */
.bg-gradient-mesh {
  background:
    radial-gradient(ellipse at 20% 50%, rgba(120,40,200,0.15) 0%, transparent 60%),
    radial-gradient(ellipse at 80% 20%, rgba(40,80,200,0.1) 0%, transparent 60%),
    #0A0A0A;
}

/* Geometric pattern (works for art deco, retro-futuristic) */
.bg-geometric {
  background-color: #1A1A2E;
  background-image: repeating-linear-gradient(
    45deg,
    transparent,
    transparent 20px,
    rgba(255,215,0,0.05) 20px,
    rgba(255,215,0,0.05) 21px
  );
}
```

### 7. Match complexity to aesthetic

Maximalist designs require elaborate code. Minimal designs require restraint. The implementation must serve the vision.

```
Maximalist   → extensive animations, layered backgrounds, multiple typefaces,
               dense grid, decorative elements, rich hover states

Minimalist   → near-zero decoration, precise spacing (every value intentional),
               single typeface, dominant whitespace, one accent color used once

Industrial   → visible structure, utility spacing scale, weight used as decoration,
               no curves, border-box everything
```

---

## Anti-Patterns

### BAD: Generic AI aesthetics

```css
/* ❌ — purple gradient on white = most common AI-generated UI */
background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
font-family: 'Inter', sans-serif;

/* ✅ — commit to a direction */
background: #0F0F0F;
font-family: '"DM Serif Display"', Georgia, serif;
```

### BAD: Symmetric everything

```css
/* ❌ — equal columns, equal padding, no tension */
.grid { grid-template-columns: 1fr 1fr; gap: 2rem; padding: 2rem; }

/* ✅ — intentional asymmetry */
.grid { grid-template-columns: 1.2fr 0.8fr; gap: 0; }
.col-left  { padding: 6rem 4rem 6rem 6rem; }
.col-right { padding: 8rem 6rem 4rem 4rem; }
```

### BAD: Scattered micro-interactions

```css
/* ❌ — every element has hover, every card bounces, nothing stands out */
.card:hover    { transform: scale(1.02); }
.button:hover  { transform: translateY(-1px); }
.link:hover    { text-decoration: underline; }
.icon:hover    { rotate: 15deg; }

/* ✅ — one deliberate effect at the right moment */
.hero-cta:hover {
  background: var(--accent);
  color: var(--bg);
  transition: background 0.2s ease, color 0.2s ease;
}
```

### BAD: No defined color variables

```css
/* ❌ — hardcoded values everywhere, no system */
h1 { color: #1a1a1a; }
p  { color: #4a4a4a; }
a  { color: #2563eb; }
button { background: #2563eb; }

/* ✅ — systematic variables, one place to change */
:root {
  --text-primary:   #1A1A1A;
  --text-secondary: #4A4A4A;
  --accent:         #2563EB;
}
```

---

## Community Skills

| Skill | Install command |
|---|---|
| [Frontend Design](https://skills.sh/anthropics/skills/frontend-design) | `npx skills add anthropics/skills --skill frontend-design -a claude-code` |
| [Web Artifacts Builder](https://skills.sh/anthropics/skills/web-artifacts-builder) | `npx skills add anthropics/skills --skill web-artifacts-builder -a claude-code` |
