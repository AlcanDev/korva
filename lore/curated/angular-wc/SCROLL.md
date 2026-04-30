---
id: angular-wc
version: 1.1.0
team: frontend
stack: Angular 20, Angular Elements, TypeScript, Signals, wc-connector
last_updated: 2026-04-30
---

# Scroll: Angular Web Components

## Triggers — load when:
- Files: `*.component.ts`, `app.module.ts`, `*.element.ts`, `main.ts`, `angular.json`
- Keywords: Angular Elements, Web Component, custom element, wc-connector, sendRequest, postMessage, Signals, OnPush, @Input, @Output, createCustomElement
- Tasks: creating a new component, setting up host communication, adding a new screen, configuring change detection, bootstrapping a Web Component

## Context
Micro-frontend Web Components are built with Angular 20 and exposed via Angular Elements. Each Web Component communicates with the host application (the shell or native container) through a `wc-connector` abstraction, which wraps `postMessage` and event-based protocols. Components use the OnPush change detection strategy and Signals for reactive state — no manual `ChangeDetectorRef.detectChanges()` should be necessary.

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
import { ProductListComponent } from './app/screens/product-list/product-list.component';

platformBrowser()
  .bootstrapModule(AppModule)
  .then((ref) => {
    const element = createCustomElement(ProductListComponent, { injector: ref.injector });
    customElements.define('product-list-wc', element);
  })
  .catch((err) => console.error(err));
```

### 3. wc-connector for host communication

Use `sendRequest` for request-response interactions and listen to events for host-initiated messages. Never use `window.postMessage` directly.

```typescript
// screens/product-list/product-list.component.ts
import { Component, Input, OnInit, signal } from '@angular/core';
import { WcConnector } from '@acme/wc-connector';
import { Product } from '../../shared/types/product.type';

@Component({
  selector: 'app-product-list',
  templateUrl: './product-list.component.html',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ProductListComponent implements OnInit {
  @Input() categoryId!: string;
  @Input() userId!: string;

  products = signal<Product[]>([]);
  loading = signal(true);
  error = signal<string | null>(null);

  constructor(private readonly connector: WcConnector) {}

  ngOnInit(): void {
    this.loadProducts();
  }

  private async loadProducts(): Promise<void> {
    try {
      const response = await this.connector.sendRequest<Product[]>({
        action: 'GET_PRODUCTS',
        payload: { categoryId: this.categoryId, userId: this.userId },
      });
      this.products.set(response);
    } catch {
      this.error.set('Could not load products');
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
export class ProductListComponent {
  // Inputs — host passes data in
  @Input() categoryId!: string;
  @Input() userId!: string;
  @Input() region: 'US' | 'EU' | 'APAC' = 'US';

  // Outputs — component emits events out
  @Output() productSelected = new EventEmitter<{ productId: string; productName: string }>();
  @Output() closed = new EventEmitter<void>();
}
```

### 5. Signals for reactive state — no manual CD

Angular 17+ signals eliminate the need for `ChangeDetectorRef` with OnPush components. Use signals for all mutable state.

```typescript
// CORRECT — Signals + OnPush
products   = signal<Product[]>([]);
selected   = signal<Product | null>(null);
isLoading  = signal(false);

selectProduct(product: Product): void {
  this.selected.set(product);  // triggers OnPush CD automatically
  this.productSelected.emit({ productId: product.id, productName: product.name });
}
```

```html
<!-- Template reads signal with () -->
@if (isLoading()) {
  <app-loading-spinner />
} @else {
  @for (product of products(); track product.id) {
    <app-product-card [product]="product" (click)="selectProduct(product)" />
  }
}
```

### 6. OnPush change detection is required on all components

```typescript
@Component({
  selector: 'app-product-card',
  templateUrl: './product-card.component.html',
  changeDetection: ChangeDetectionStrategy.OnPush,  // mandatory
})
export class ProductCardComponent {
  @Input() product!: Product;
}
```

### 7. Never use ViewChild/ElementRef for DOM manipulation

Use Angular template refs (`#ref`) and directives. Direct DOM manipulation bypasses Angular's change detection and breaks SSR compatibility.

```html
<!-- CORRECT — Angular dialog directive -->
<app-dialog #confirmDialog>
  <p>Confirm purchase of {{ selected()?.name }}?</p>
  <button (click)="confirmDialog.close()">Cancel</button>
  <button (click)="confirmPurchase()">Confirm</button>
</app-dialog>

<button (click)="confirmDialog.open()">Buy now</button>
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
  selector: 'app-product-card',
  templateUrl: './product-card.component.html',
})
export class ProductCardComponent { ... }
```

```typescript
// GOOD
@Component({
  selector: 'app-product-card',
  templateUrl: './product-card.component.html',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ProductCardComponent { ... }
```

### BAD: Direct window.postMessage instead of wc-connector
```typescript
// BAD
window.parent.postMessage({ action: 'GET_PRODUCTS', categoryId: this.categoryId }, '*');
```

```typescript
// GOOD
const response = await this.connector.sendRequest({ action: 'GET_PRODUCTS', payload: { categoryId: this.categoryId } });
```

### BAD: Using BehaviorSubject when Signal suffices
```typescript
// BAD — unnecessary RxJS for simple local state
private productsSubject = new BehaviorSubject<Product[]>([]);
products$ = this.productsSubject.asObservable();
```

```typescript
// GOOD — Signal for local component state
products = signal<Product[]>([]);
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
