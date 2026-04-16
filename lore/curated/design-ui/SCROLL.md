---
id: component-design-system
version: 1.1.0
team: frontend
stack: Angular 20, Design System, Sass, CSS custom properties
---

# Scroll: Component Design System Patterns

## Triggers — load when:
- Files: `*.component.ts`, `*.component.html`, `*.component.scss`, `styles.scss`
- Keywords: design system, component library, Button, Dialog, Input, Select, Table, Sass variable, CSS custom property, color token, typography class, spacing utility
- Tasks: adding a UI component, applying colors, using typography classes, spacing elements, building a screen layout

## Context
A design system centralises visual decisions — colors, typography, spacing — into a single source of truth. All front-end components must import UI primitives exclusively from the team's component library and apply styles via the library's Sass variables and utility classes. No Tailwind, no CSS-in-JS, no Styled Components, no inline styles, no hardcoded hex or rgb values are allowed. Design token consistency is enforced by code review and automated linting.

---

## Rules

### 1. Import components from the component library

```typescript
// *.component.ts or a shared module
import { Button, Dialog, Input, Select, Table, Alert, Badge } from '@acme/components';
```

In an Angular module, declare library components inside `imports`:

```typescript
// shared/design-system.module.ts
import { NgModule } from '@angular/core';
import {
  ButtonModule,
  DialogModule,
  InputModule,
  SelectModule,
  TableModule,
  AlertModule,
  BadgeModule,
} from '@acme/components';

const DS_MODULES = [ButtonModule, DialogModule, InputModule, SelectModule, TableModule, AlertModule, BadgeModule];

@NgModule({ imports: DS_MODULES, exports: DS_MODULES })
export class DesignSystemModule {}
```

### 2. CSS global stylesheet — import once at root level

The global stylesheet (resets, font-face, CSS custom properties) must be imported in the root global stylesheet — not in component styles.

```scss
// styles.scss  (global)
@import url('https://cdn.acme.io/design-system/latest/global.css');
```

Component-level `.scss` files import only the design system's Sass tokens:

```scss
// product-card.component.scss
@import '@acme/components/scss/tokens';

.product-card {
  background-color: $neutral-10;
  border: 1px solid $neutral-30;
  padding: 16px;
}
```

### 3. Colors: always design tokens, never hex values

#### Available palette (example token names — match your actual system)

| Variable | Description |
|---|---|
| `$primary` | Brand primary — CTAs, highlights |
| `$neutral` | Grays — text, borders, backgrounds |
| `$danger` | Error / destructive |
| `$warning` | Warning states |
| `$accent` | Secondary accent |

Each color has a numeric scale: `$primary-10` through `$primary-100` (light to dark).

```scss
// CORRECT
.price-highlight { color: $primary-60; }
.error-message   { color: $danger-50; }
.card-border     { border-color: $neutral-30; }
.section-bg      { background-color: $neutral-10; }
.cta-button      { background-color: $primary; }  // base token = main shade

// WRONG
.price-highlight { color: #2d7d32; }              // hardcoded hex
.error-message   { color: #c62828; }              // hardcoded hex
.card-border     { border-color: rgba(0,0,0,.12); } // hardcoded rgba
```

### 4. CSS custom properties in component templates

When applying colors dynamically (e.g., from a runtime variable), use CSS custom properties:

```html
<!-- CORRECT -->
<div [style.color]="'var(--primary-60)'">{{ product.name }}</div>

<!-- For static use, prefer Sass variables in SCSS instead of inline style -->
```

```html
<!-- WRONG — hardcoded hex in template -->
<div [style.color]="'#2d7d32'">{{ product.name }}</div>
```

### 5. Typography: className, not inline font-size

The design system provides typography utility classes. Always use them — never set `font-size`, `font-weight`, or `line-height` manually.

```html
<!-- CORRECT -->
<h2 class="title-m">Choose your plan</h2>
<p class="body-m">Select the option that works best for you.</p>
<span class="label-s">Monthly price</span>
<strong class="title-s">$15.99</strong>

<!-- WRONG -->
<h2 style="font-size: 24px; font-weight: 700;">Choose your plan</h2>
<p style="font-size: 16px;">Select the option...</p>
```

Available typography classes: `display-l`, `display-m`, `title-xl`, `title-l`, `title-m`, `title-s`, `body-l`, `body-m`, `body-s`, `label-l`, `label-m`, `label-s`.

### 6. Spacing: utility classes, not margin/padding inline

```html
<!-- CORRECT -->
<div class="mt-16 pb-8 px-24">
  <app-product-card class="mb-8" *ngFor="let item of items()" [item]="item" />
</div>

<!-- WRONG -->
<div style="margin-top: 16px; padding-bottom: 8px; padding-left: 24px;">
  <app-product-card style="margin-bottom: 8px;" ... />
</div>
```

Spacing scale follows 4px base: `4, 8, 12, 16, 20, 24, 32, 40, 48, 64`. Classes: `mt-{n}`, `mb-{n}`, `ml-{n}`, `mr-{n}`, `pt-{n}`, `pb-{n}`, `px-{n}`, `py-{n}`.

### 7. Use existing components before building custom ones

Always check whether the design system already provides the component you need. Typical libraries include:

Button, Dialog, Input, Select, Textarea, Checkbox, Radio, Toggle, Table, Pagination, Cards, Alerts, Badges, Chips, Tabs, Accordion, Breadcrumb, Stepper, Tooltip, Skeleton, Progress.

Use these before building custom equivalents. If a component is missing, file a request to the design system team — do not improvise an alternative.

```html
<!-- Product listing screen using design system components -->
<ds-alert *ngIf="error()" type="error" [message]="error()!" class="mb-16" />

<div class="products-grid mt-24">
  <ds-card *ngFor="let item of items(); track item.id" class="product-card mb-16">
    <h3 class="title-s">{{ item.name }}</h3>
    <p class="body-m mt-8 mb-16">{{ item.description }}</p>
    <strong class="title-m" [style.color]="'var(--primary-60)'">
      ${{ item.monthlyPrice | number }}/mo
    </strong>
    <ds-button class="mt-16" variant="primary" (click)="selectItem(item)">
      Get started
    </ds-button>
  </ds-card>
</div>
```

---

## Anti-Patterns

### BAD: Utility framework (Tailwind) mixed with design system
```html
<!-- BAD — Tailwind is not the design system -->
<div class="flex flex-col gap-4 p-6 bg-green-700 text-white rounded-lg">
```

```html
<!-- GOOD — design system spacing + components -->
<ds-card class="px-24 py-16">
```

### BAD: Hardcoded hex color in SCSS
```scss
// BAD
.price-highlight { color: #1b5e20; }
.error-text      { color: #b71c1c; }
```

```scss
// GOOD
.price-highlight { color: $primary-70; }
.error-text      { color: $danger-60; }
```

### BAD: Inline font-size and font-weight
```html
<!-- BAD -->
<p style="font-size: 14px; font-weight: 500; color: #333;">Product description</p>
```

```html
<!-- GOOD -->
<p class="body-s" [style.color]="'var(--neutral-80)'">Product description</p>
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
// GOOD — component SCSS with design system tokens
.product-card {
  background: $neutral-0;
  padding: 16px;
  border: 1px solid $neutral-20;
}
```
