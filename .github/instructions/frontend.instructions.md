---
applyTo: "src/**/*.component.ts,src/**/*.component.html,src/**/*.tsx,src/**/*.scss,src/**/*.css,apps/*/src/**"
---

# Frontend — Angular Web Components + React (Tomaco UI)

## Angular Web Components (primary pattern)

```typescript
// Encapsulation: always ShadowDom for Web Components
@Component({
  selector: 'app-insurance-card',
  encapsulation: ViewEncapsulation.ShadowDom,
  templateUrl: './insurance-card.component.html',
  styleUrls: ['./insurance-card.component.scss'],
})
export class InsuranceCardComponent implements OnInit, OnDestroy {
  // Public inputs — part of the component contract
  @Input() insuranceId!: string;
  @Input() country: 'CL' | 'PE' | 'CO' = 'CL';

  // Public outputs — DOM events for consumers
  @Output() selected = new EventEmitter<string>();

  private destroy$ = new Subject<void>();

  ngOnDestroy(): void {
    this.destroy$.next();
    this.destroy$.complete();
  }
}
```

## Tomaco UI — design system (mandatory)

```typescript
// Imports — always from tomaco-components
import { Button, Dialog, Input, Table, Select, Cards, Alerts } from 'tomaco-components';

// ❌ Never raw hex or px values
color: #3b82f6;
font-size: 16px;

// ✅ Always Sass variables or CSS custom properties
color: $avocado-40;          // var(--avocado-40)
color: $neutral-80;          // var(--neutral-80)
color: $cherry-20;           // var(--cherry-20)

// ✅ Typography — always classes, never inline font-size
className="title-m body-s caption-s"

// ✅ Spacing — utility classes only
className="mt-16 pb-8 ma-24 px-12"
```

**Tomaco color semantics:**
- `$avocado` — green, primary actions, success
- `$neutral` — grays, text, borders
- `$cherry` — red, errors, destructive actions
- `$banana` — yellow, warnings
- `$blueberry` — blue, informational, links

## State management (Angular)

Prefer **signals** (Angular 17+) for local component state:
```typescript
// ✅ Signals for local state
readonly offers = signal<InsuranceOffer[]>([]);
readonly isLoading = signal(false);
readonly error = signal<string | null>(null);

// Computed state
readonly hasOffers = computed(() => this.offers().length > 0);
```

Use NgRx only for shared cross-component state that is persisted or complex.
Never use BehaviorSubject where signals suffice.

## Performance rules

```typescript
// ✅ Async pipe — automatic subscription management
<div *ngIf="offers$ | async as offers">

// ✅ TrackBy for all *ngFor
<div *ngFor="let offer of offers; trackBy: trackById">
trackById = (_: number, item: InsuranceOffer) => item.id;

// ✅ OnPush change detection for performance-sensitive components
@Component({ changeDetection: ChangeDetectionStrategy.OnPush })

// ❌ Never subscribe manually without unsubscribing
// ✅ takeUntil(this.destroy$) or async pipe
```

## Accessibility (WCAG 2.1 AA — mandatory)

```html
<!-- Every interactive element needs a label -->
<button aria-label="Select insurance plan" (click)="select(offer)">
  <tomaco-icon name="check" aria-hidden="true" />
</button>

<!-- Images need alt text -->
<img [src]="offer.logoUrl" [alt]="offer.name + ' logo'" />

<!-- Forms need associated labels -->
<label for="rut">RUT</label>
<input id="rut" type="text" aria-required="true" aria-describedby="rut-error" />
<span id="rut-error" role="alert">{{ rutError }}</span>

<!-- Focus management for dialogs -->
<!-- Use Dialog from tomaco-components — it handles focus trap automatically -->
```

**Color contrast:** All text must meet 4.5:1 (normal) or 3:1 (large text).
Use Tomaco tokens — they are pre-validated for contrast.

## i18n

All user-facing strings must go through the i18n pipe or `$localize`:
```html
<p>{{ 'INSURANCE.SELECT_PLAN' | translate }}</p>
<!-- Never hardcode Spanish: <p>Selecciona un plan</p> -->
```

## Error UI patterns

```html
<!-- Loading state -->
<tomaco-skeleton [rows]="3" *ngIf="isLoading()" />

<!-- Error state -->
<tomaco-alerts type="error" [message]="errorMessage()" *ngIf="error()" />

<!-- Empty state -->
<div class="empty-state" *ngIf="!isLoading() && !hasOffers()">
  <tomaco-icon name="inbox" size="48" />
  <p class="body-m neutral-60">No hay planes disponibles</p>
</div>
```

## Forbidden patterns

```typescript
// ❌ Direct DOM manipulation — use Angular template bindings
document.getElementById('btn').style.color = 'red';

// ❌ Inline styles — use Tomaco utility classes or Sass variables
[style.color]="'#ff0000'"

// ❌ console.log in component code

// ❌ Unsubscribed observables (memory leaks)

// ❌ any type on component inputs/outputs

// ❌ Non-tomaco components when tomaco equivalent exists
<button class="btn"> → use <tomaco-button>
```
