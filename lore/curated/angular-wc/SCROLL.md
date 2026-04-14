---
id: angular-wc
version: 1.0.0
team: frontend
stack: Angular 20, Angular Elements, TypeScript, Signals, host-bridge
---

# Scroll: Angular Web Components

## Triggers — load when:
- Files: `*.component.ts`, `app.module.ts`, `*.element.ts`, `main.ts`, `angular.json`
- Keywords: Angular Elements, Web Component, custom element, wc-connector, sendRequest, postMessage, Signals, OnPush, @Input, @Output, createCustomElement
- Tasks: creating a new component, setting up host communication, adding a new screen, configuring change detection, bootstrapping a Web Component

## Context
Acme Financiero micro-frontend Web Components are built with Angular 20 and exposed via Angular Elements. Each Web Component communicates with the host application (the shell or native container) through `host-bridge`, which abstracts `postMessage` and event-based protocols. Components use the OnPush change detection strategy and Signals for reactive state — no manual `ChangeDetectorRef.detectChanges()` should be necessary.

---

## Rules

### 1. Output project structure

```
src/
  components/     — presentational, reusable UI pieces
  screens/        — full views composed from components
  services/       — injectable application services (no domain logic)
  shared/         — types, constants, pipes, directives shared across the project
  app.module.ts
  main.ts
```

### 2. Angular Elements bootstrap

```typescript
// main.ts
import { platformBrowser } from '@angular/platform-browser';
import { createCustomElement } from '@angular/elements';
import { AppModule } from './app/app.module';
import { InsuranceOffersComponent } from './app/screens/insurance-offers/insurance-offers.component';

platformBrowser()
  .bootstrapModule(AppModule)
  .then((ref) => {
    const element = createCustomElement(InsuranceOffersComponent, { injector: ref.injector });
    customElements.define('insurance-offers-wc', element);
  })
  .catch((err) => console.error(err));
```

### 3. host-bridge for host communication

Use `sendRequest` for request-response interactions and listen to `postMessage` events for host-initiated messages. Never use `window.postMessage` directly.

```typescript
// screens/insurance-offers/insurance-offers.component.ts
import { Component, Input, OnInit, signal } from '@angular/core';
import { WcConnector } from 'host-bridge';
import { InsuranceOffer } from '../../shared/types/insurance-offer.type';

@Component({
  selector: 'app-insurance-offers',
  templateUrl: './insurance-offers.component.html',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class InsuranceOffersComponent implements OnInit {
  @Input() productId!: string;
  @Input() customerId!: string;

  offers = signal<InsuranceOffer[]>([]);
  loading = signal(true);
  error = signal<string | null>(null);

  constructor(private readonly connector: WcConnector) {}

  ngOnInit(): void {
    this.loadOffers();
  }

  private async loadOffers(): Promise<void> {
    try {
      const response = await this.connector.sendRequest<InsuranceOffer[]>({
        action: 'GET_INSURANCE_OFFERS',
        payload: { productId: this.productId, customerId: this.customerId },
      });
      this.offers.set(response);
    } catch {
      this.error.set('No se pudieron cargar los seguros');
    } finally {
      this.loading.set(false);
    }
  }
}
```

### 4. @Input() / @Output() define the public component API

Web Component inputs and outputs are the contract between the component and the host. Keep them minimal and typed.

```typescript
@Component({ ... })
export class InsuranceOffersComponent {
  // Inputs — host passes data in
  @Input() productId!: string;
  @Input() customerId!: string;
  @Input() country: 'CL' | 'PE' | 'CO' = 'CL';

  // Outputs — component emits events out
  @Output() offerSelected = new EventEmitter<{ offerId: string; offerName: string }>();
  @Output() closed = new EventEmitter<void>();
}
```

### 5. Signals for reactive state — no manual CD

Angular 17+ signals eliminate the need for `ChangeDetectorRef` with OnPush components. Use signals for all mutable state.

```typescript
// CORRECT — Signals + OnPush
offers     = signal<InsuranceOffer[]>([]);
selected   = signal<InsuranceOffer | null>(null);
isLoading  = signal(false);

selectOffer(offer: InsuranceOffer): void {
  this.selected.set(offer);  // triggers OnPush CD automatically
  this.offerSelected.emit({ offerId: offer.id, offerName: offer.name });
}
```

```html
<!-- Template reads signal with () -->
@if (isLoading()) {
  <app-loading-spinner />
} @else {
  @for (offer of offers(); track offer.id) {
    <app-offer-card [offer]="offer" (click)="selectOffer(offer)" />
  }
}
```

### 6. OnPush change detection is required on all components

```typescript
@Component({
  selector: 'app-offer-card',
  templateUrl: './offer-card.component.html',
  changeDetection: ChangeDetectionStrategy.OnPush,  // mandatory
})
export class OfferCardComponent {
  @Input() offer!: InsuranceOffer;
}
```

### 7. Never use ViewChild/ElementRef for DOM manipulation

Use Angular template refs (`#ref`) and directives. Direct DOM manipulation bypasses Angular's change detection and breaks SSR compatibility.

```html
<!-- CORRECT — Angular dialog directive -->
<app-dialog #confirmDialog>
  <p>¿Confirmar contratación de {{ selected()?.name }}?</p>
  <button (click)="confirmDialog.close()">Cancelar</button>
  <button (click)="confirmOffer()">Confirmar</button>
</app-dialog>

<button (click)="confirmDialog.open()">Contratar</button>
```

```typescript
// WRONG — direct DOM manipulation
@ViewChild('confirmDialog') dialogRef!: ElementRef;
openDialog() { this.dialogRef.nativeElement.showModal(); }
```

---

## Anti-Patterns

### BAD: Default change detection strategy
```typescript
// BAD — no changeDetection specified (defaults to Default, causes unnecessary checks)
@Component({
  selector: 'app-offer-card',
  templateUrl: './offer-card.component.html',
})
export class OfferCardComponent { ... }
```

```typescript
// GOOD
@Component({
  selector: 'app-offer-card',
  templateUrl: './offer-card.component.html',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class OfferCardComponent { ... }
```

### BAD: Direct window.postMessage instead of wc-connector
```typescript
// BAD
window.parent.postMessage({ action: 'GET_OFFERS', productId: this.productId }, '*');
```

```typescript
// GOOD
const response = await this.connector.sendRequest({ action: 'GET_INSURANCE_OFFERS', payload: { productId: this.productId } });
```

### BAD: Using BehaviorSubject when Signal suffices
```typescript
// BAD — unnecessary RxJS for simple local state
private offersSubject = new BehaviorSubject<InsuranceOffer[]>([]);
offers$ = this.offersSubject.asObservable();
```

```typescript
// GOOD — Signal for local component state
offers = signal<InsuranceOffer[]>([]);
```

### BAD: DOM manipulation via ElementRef
```typescript
// BAD
@ViewChild('container') container!: ElementRef;
ngAfterViewInit() {
  this.container.nativeElement.style.display = 'block';
}
```

```typescript
// GOOD — drive visibility via template binding and signals
visible = signal(false);
// Template: [class.visible]="visible()"
```
