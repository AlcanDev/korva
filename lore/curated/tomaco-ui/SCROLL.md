---
id: tomaco-ui
version: 1.0.0
team: frontend
stack: Angular 20, Tomaco Components, Sass, CSS custom properties
---

# Scroll: Tomaco UI Design System

## Triggers — load when:
- Files: `*.component.ts`, `*.component.html`, `*.component.scss`, `styles.scss`
- Keywords: tomaco, tomaco-components, Button, Dialog, Input, Select, Table, $avocado, $neutral, $cherry, var(--avocado), title-m, mt-16, pb-8, design system, color variable, Sass variable
- Tasks: adding a UI component, applying colors, using typography classes, spacing elements, building a screen layout

## Context
Falabella Financiero uses Tomaco as its official design system. All front-end Web Components must import UI primitives exclusively from `tomaco-components` and apply styles via Tomaco's Sass variables and utility classes. No Tailwind, no CSS-in-JS, no Styled Components, no inline styles, no hardcoded hex or rgb values are allowed. Design token consistency is enforced by code review and automated linting.

---

## Rules

### 1. Import components from tomaco-components

```typescript
// *.component.ts or a shared module
import { Button, Dialog, Input, Select, Table, Alert, Badge } from 'tomaco-components';
```

In an Angular module, declare Tomaco components inside `imports`:

```typescript
// shared/tomaco.module.ts
import { NgModule } from '@angular/core';
import {
  ButtonModule,
  DialogModule,
  InputModule,
  SelectModule,
  TableModule,
  AlertModule,
  BadgeModule,
} from 'tomaco-components';

const TOMACO_MODULES = [ButtonModule, DialogModule, InputModule, SelectModule, TableModule, AlertModule, BadgeModule];

@NgModule({ imports: TOMACO_MODULES, exports: TOMACO_MODULES })
export class TomacoModule {}
```

### 2. CSS global stylesheet — import from CDN separately

The global Tomaco stylesheet (resets, font-face, CSS custom properties) must be imported in the global stylesheet — not in component styles.

```scss
// styles.scss  (global)
@import url('https://cdn.fif.tech/tomaco/latest/tomaco.global.css');
```

Component-level `.scss` files import only Tomaco's Sass tokens:

```scss
// insurance-offers.component.scss
@import 'tomaco-components/scss/tokens';

.offer-card {
  background-color: $neutral-10;
  border: 1px solid $neutral-30;
  padding: 16px;
}
```

### 3. Colors: always Sass variables, never hex values

#### Available palette

| Variable | Description |
|---|---|
| `$avocado` | Primary green — brand, CTAs |
| `$neutral` | Grays — text, borders, backgrounds |
| `$cherry` | Error / destructive |
| `$orange` | Warning |
| `$raspberry` | Secondary accent |

Each color has a numeric scale: `$avocado-10` through `$avocado-100` (light to dark).

```scss
// CORRECT
.offer-price   { color: $avocado-60; }
.offer-error   { color: $cherry-50; }
.card-border   { border-color: $neutral-30; }
.section-bg    { background-color: $neutral-10; }
.cta-button    { background-color: $avocado; }  // base token = main shade

// WRONG
.offer-price   { color: #2d7d32; }   // hardcoded hex
.offer-error   { color: #c62828; }   // hardcoded hex
.card-border   { border-color: rgba(0,0,0,.12); }  // hardcoded rgba
```

### 4. CSS custom properties in component templates

When applying Tomaco colors dynamically (e.g., from a variable), use CSS custom properties:

```html
<!-- CORRECT -->
<div [style.color]="'var(--avocado-60)'">{{ offer.name }}</div>

<!-- For static use, prefer Sass variables in SCSS instead of inline style -->
```

```html
<!-- WRONG — hardcoded hex in template -->
<div [style.color]="'#2d7d32'">{{ offer.name }}</div>
```

### 5. Typography: className, not inline font-size

Tomaco provides typography utility classes. Always use them — never set `font-size`, `font-weight`, or `line-height` manually.

```html
<!-- CORRECT -->
<h2 class="title-m">Seguros para tu hogar</h2>
<p class="body-m">Elige el plan que mejor se adapte a ti.</p>
<span class="label-s">Precio mensual</span>
<strong class="title-s">$15.990</strong>

<!-- WRONG -->
<h2 style="font-size: 24px; font-weight: 700;">Seguros para tu hogar</h2>
<p style="font-size: 16px;">Elige el plan...</p>
```

Available typography classes: `display-l`, `display-m`, `title-xl`, `title-l`, `title-m`, `title-s`, `body-l`, `body-m`, `body-s`, `label-l`, `label-m`, `label-s`.

### 6. Spacing: utility classes, not margin/padding inline

```html
<!-- CORRECT -->
<div class="mt-16 pb-8 px-24">
  <app-offer-card class="mb-8" *ngFor="let offer of offers()" [offer]="offer" />
</div>

<!-- WRONG -->
<div style="margin-top: 16px; padding-bottom: 8px; padding-left: 24px;">
  <app-offer-card style="margin-bottom: 8px;" ... />
</div>
```

Tomaco spacing scale follows 4px base: `4, 8, 12, 16, 20, 24, 32, 40, 48, 64`. Classes: `mt-{n}`, `mb-{n}`, `ml-{n}`, `mr-{n}`, `pt-{n}`, `pb-{n}`, `px-{n}`, `py-{n}`.

### 7. Available Tomaco components (21 total)

Button, Dialog, Input, Select, Textarea, Checkbox, Radio, Toggle, Table, Pagination, Cards, Alerts, Badges, Chips, Tabs, Accordion, Breadcrumb, Stepper, Tooltip, Skeleton, Progress.

Use these before building custom equivalents. If a component is missing, file a request to the design system team — do not improvise an alternative.

```html
<!-- Insurance offer screen using Tomaco components -->
<tmc-alert *ngIf="error()" type="error" [message]="error()!" class="mb-16" />

<div class="offers-grid mt-24">
  <tmc-card *ngFor="let offer of offers(); track offer.id" class="offer-card mb-16">
    <h3 class="title-s">{{ offer.name }}</h3>
    <p class="body-m mt-8 mb-16">{{ offer.description }}</p>
    <strong class="title-m" [style.color]="'var(--avocado-60)'">
      ${{ offer.monthlyPrice | number }}/mes
    </strong>
    <tmc-button class="mt-16" variant="primary" (click)="selectOffer(offer)">
      Contratar
    </tmc-button>
  </tmc-card>
</div>
```

---

## Anti-Patterns

### BAD: Tailwind utility classes
```html
<!-- BAD — Tailwind is not part of the design system -->
<div class="flex flex-col gap-4 p-6 bg-green-700 text-white rounded-lg">
```

```html
<!-- GOOD — Tomaco spacing + components -->
<tmc-card class="px-24 py-16">
```

### BAD: Hardcoded hex color in SCSS
```scss
// BAD
.offer-price { color: #1b5e20; }
.error-text  { color: #b71c1c; }
```

```scss
// GOOD
.offer-price { color: $avocado-70; }
.error-text  { color: $cherry-60; }
```

### BAD: Inline font-size and font-weight
```html
<!-- BAD -->
<p style="font-size: 14px; font-weight: 500; color: #333;">Descripción del seguro</p>
```

```html
<!-- GOOD -->
<p class="body-s" [style.color]="'var(--neutral-80)'">Descripción del seguro</p>
```

### BAD: CSS-in-JS or Styled Components
```typescript
// BAD — no CSS-in-JS pattern in Angular projects
const StyledCard = styled.div`
  background: #fff;
  padding: 16px;
`;
```

```scss
// GOOD — component SCSS with Tomaco tokens
.insurance-card {
  background: $neutral-0;
  padding: 16px;
  border: 1px solid $neutral-20;
}
```
