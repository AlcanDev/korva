---
id: react-nextjs
version: 1.0.0
team: frontend
stack: React 19, Next.js 15, TypeScript, App Router
---

# Scroll: React & Next.js — App Router Patterns

## Triggers — load when:
- Files: `page.tsx`, `layout.tsx`, `loading.tsx`, `error.tsx`, `not-found.tsx`, `route.ts`, `middleware.ts`, `*.component.tsx`, `*.hook.ts`, `use-*.ts`
- Keywords: `useEffect`, `useState`, `Server Component`, `Client Component`, `use client`, `use server`, `App Router`, `next/navigation`, `next/image`, `Suspense`, `revalidate`, `generateStaticParams`
- Tasks: building pages, managing state, data fetching, performance optimization, routing, caching

## Context
React 19 and Next.js 15 App Router represent a fundamental shift in how React applications are built. Server Components are the default — they run only on the server, have direct database access, and ship zero JavaScript to the client. The most common mistake is reaching for `use client` out of habit, unnecessarily bloating the JS bundle. These patterns encode the decisions that are easy to get wrong: when to use Server vs Client Components, how to fetch data without waterfalls, state that doesn't cause re-render cascades, and caching strategies that actually work.

---

## Rules

### 1. Server Components by default — `use client` is opt-in

Server Components run on the server only: no hydration, no JS bundle cost, direct DB/filesystem access. Make a component Client only when it actually needs interactivity.

```tsx
// ✅ Server Component (default) — no directive needed
// Runs on server. Direct DB access. Zero JS to the client.
async function ProductPage({ params }: { params: { id: string } }) {
  // Direct DB query — no useEffect, no fetch round trip
  const product = await db.product.findUnique({ where: { id: params.id } });

  if (!product) notFound();

  return (
    <article>
      <h1>{product.name}</h1>
      <p>{product.description}</p>
      {/* Pass serializable data down to Client Components */}
      <AddToCartButton productId={product.id} price={product.price} />
    </article>
  );
}
```

```tsx
// ✅ Client Component — only when you need browser APIs, state, or event handlers
'use client';

import { useState } from 'react';

interface AddToCartButtonProps {
  productId: string;
  price: number;
}

export function AddToCartButton({ productId, price }: AddToCartButtonProps) {
  const [loading, setLoading] = useState(false);

  async function handleAddToCart() {
    setLoading(true);
    await addToCart(productId);
    setLoading(false);
  }

  return (
    <button onClick={handleAddToCart} disabled={loading}>
      {loading ? 'Adding...' : `Add to Cart — $${price}`}
    </button>
  );
}
```

**When to use `'use client'`:**
- Browser APIs (`window`, `localStorage`, `navigator`)
- Event handlers (`onClick`, `onChange`, `onSubmit`)
- `useState`, `useReducer`, `useEffect`, `useRef`
- Real-time subscriptions (WebSocket, EventSource)
- Third-party libraries that require a DOM

**Never use `'use client'` just because**: you're fetching data (use Server Components), you need async (Server Components are async by default), or you want to show a loading state (use `Suspense`).

---

### 2. Data fetching in Server Components — no waterfall

Fetch data in the component that needs it. Parallelize with `Promise.all`. Never chain sequential awaits when requests are independent.

```tsx
// ✅ Parallel data fetching — both requests start simultaneously
async function DashboardPage() {
  // Promise.all starts both fetches at the same time
  const [user, recentOrders] = await Promise.all([
    fetchUser(),
    fetchRecentOrders(),
  ]);

  return (
    <div>
      <UserProfile user={user} />
      <OrderHistory orders={recentOrders} />
    </div>
  );
}
```

```tsx
// ❌ Waterfall — orders waits for user to finish before starting
async function DashboardPageBad() {
  const user = await fetchUser();           // waits
  const orders = await fetchRecentOrders(); // only starts after user finishes
  // Total time: user.time + orders.time (unnecessary)
}
```

```tsx
// ✅ Streaming with Suspense — show content as it arrives
import { Suspense } from 'react';

async function DashboardPage() {
  return (
    <div>
      {/* User data is fast, renders first */}
      <Suspense fallback={<UserSkeleton />}>
        <UserProfile />
      </Suspense>

      {/* Heavy data streamed in after — doesn't block the page */}
      <Suspense fallback={<OrderHistorySkeleton />}>
        <OrderHistory />
      </Suspense>
    </div>
  );
}
```

---

### 3. Route Handlers — typed, validated, safe

```typescript
// app/api/products/route.ts
import { NextRequest, NextResponse } from 'next/server';
import { z } from 'zod';

const CreateProductSchema = z.object({
  name: z.string().min(1).max(200),
  price: z.number().positive(),
  categoryId: z.string().uuid(),
});

export async function POST(request: NextRequest) {
  // Always validate — never trust request.json() directly
  const body = await request.json();
  const result = CreateProductSchema.safeParse(body);

  if (!result.success) {
    return NextResponse.json(
      { error: 'Validation failed', issues: result.error.issues },
      { status: 400 }
    );
  }

  const product = await db.product.create({ data: result.data });
  return NextResponse.json(product, { status: 201 });
}
```

```typescript
// ✅ Type-safe route parameters
// app/api/products/[id]/route.ts
export async function GET(
  request: NextRequest,
  { params }: { params: { id: string } }
) {
  const product = await db.product.findUnique({ where: { id: params.id } });

  if (!product) {
    return NextResponse.json({ error: 'Not found' }, { status: 404 });
  }

  return NextResponse.json(product);
}
```

---

### 4. Server Actions — forms without API routes

```tsx
// app/actions/product.actions.ts
'use server';

import { revalidatePath } from 'next/cache';
import { redirect } from 'next/navigation';
import { z } from 'zod';

const UpdateProductSchema = z.object({
  name: z.string().min(1),
  price: z.coerce.number().positive(),
});

export async function updateProduct(productId: string, formData: FormData) {
  // Validate input — Server Actions receive raw FormData
  const result = UpdateProductSchema.safeParse({
    name: formData.get('name'),
    price: formData.get('price'),
  });

  if (!result.success) {
    return { error: result.error.flatten() };
  }

  await db.product.update({
    where: { id: productId },
    data: result.data,
  });

  // Invalidate the cached page
  revalidatePath(`/products/${productId}`);
  redirect(`/products/${productId}`);
}
```

```tsx
// app/products/[id]/edit/page.tsx
// Using the Server Action in a form — no API route needed
export default function EditProductPage({ params }: { params: { id: string } }) {
  const updateProductWithId = updateProduct.bind(null, params.id);

  return (
    <form action={updateProductWithId}>
      <input name="name" required />
      <input name="price" type="number" step="0.01" required />
      <button type="submit">Save</button>
    </form>
  );
}
```

---

### 5. Caching strategies — explicit and intentional

```typescript
// Opt-out of caching for always-fresh data (user-specific, real-time)
const userData = await fetch('/api/user/profile', {
  cache: 'no-store',
});

// Revalidate on a schedule (product catalog, blog posts)
const products = await fetch('/api/products', {
  next: { revalidate: 3600 }, // revalidate every hour
});

// Cache indefinitely, revalidate on demand via tag
const config = await fetch('/api/config', {
  next: { tags: ['site-config'] },
});
```

```typescript
// Trigger revalidation from a Server Action or API route
import { revalidateTag, revalidatePath } from 'next/cache';

// After updating site config in the CMS:
revalidateTag('site-config');

// After publishing a blog post:
revalidatePath('/blog');
revalidatePath('/blog/[slug]', 'page');
```

```typescript
// Database query caching via React's cache() — deduplicates within a render
import { cache } from 'react';

// This function is deduplicated: calling it 10x in one render = 1 DB query
export const getProduct = cache(async (id: string) => {
  return db.product.findUnique({ where: { id } });
});
```

---

### 6. State management — choose the right layer

```tsx
// ✅ URL state — shareable, bookmarkable, survives refresh (use for filters, search, pagination)
'use client';
import { useSearchParams, useRouter, usePathname } from 'next/navigation';

export function ProductFilters() {
  const searchParams = useSearchParams();
  const router = useRouter();
  const pathname = usePathname();

  function setCategory(category: string) {
    const params = new URLSearchParams(searchParams.toString());
    params.set('category', category);
    router.push(`${pathname}?${params.toString()}`);
  }

  return <CategorySelector onChange={setCategory} />;
}
```

```tsx
// ✅ Zustand for global client state (cart, user preferences, UI state)
// stores/cart.store.ts
import { create } from 'zustand';
import { persist } from 'zustand/middleware';

interface CartStore {
  items: CartItem[];
  addItem: (item: CartItem) => void;
  removeItem: (id: string) => void;
  total: () => number;
}

export const useCartStore = create<CartStore>()(
  persist(
    (set, get) => ({
      items: [],
      addItem: (item) =>
        set((state) => ({
          items: [...state.items, item],
        })),
      removeItem: (id) =>
        set((state) => ({
          items: state.items.filter((i) => i.id !== id),
        })),
      total: () => get().items.reduce((sum, item) => sum + item.price, 0),
    }),
    { name: 'cart-storage' }
  )
);
```

```
State layer decision tree:
─────────────────────────────────────────────────────
Is the state derived from server data?     → React Query / SWR (server state)
Should it survive navigation/refresh?      → URL params (useSearchParams)
Is it global UI state (cart, auth, theme)? → Zustand
Is it local to one component?              → useState / useReducer
Is it just a ref with no re-renders?       → useRef
─────────────────────────────────────────────────────
```

---

### 7. Environment variables — typed and safe

```typescript
// lib/env.ts — validate at build time with Zod
import { z } from 'zod';

const envSchema = z.object({
  // Server-only (no NEXT_PUBLIC_ prefix — never sent to client)
  DATABASE_URL: z.string().url(),
  STRIPE_SECRET_KEY: z.string().startsWith('sk_'),
  NEXTAUTH_SECRET: z.string().min(32),

  // Public (exposed to client — prefix required)
  NEXT_PUBLIC_APP_URL: z.string().url(),
  NEXT_PUBLIC_STRIPE_PUBLISHABLE_KEY: z.string().startsWith('pk_'),
});

// Throws at build time if any required variable is missing or invalid
export const env = envSchema.parse(process.env);
```

```typescript
// ❌ NEVER: accessing server-only secrets in Client Components
'use client';
// This will expose STRIPE_SECRET_KEY to the browser bundle!
const stripe = new Stripe(process.env.STRIPE_SECRET_KEY!);
```

```typescript
// ✅ CORRECT: server-only access stays in Server Components and Route Handlers
// app/api/checkout/route.ts (server-only)
import { env } from '@/lib/env';
const stripe = new Stripe(env.STRIPE_SECRET_KEY);
```

---

### 8. Next.js file conventions — App Router structure

```
app/
├── layout.tsx           — Root layout (wraps all pages, persistent UI)
├── page.tsx             — Route segment page
├── loading.tsx          — Automatic Suspense boundary during navigation
├── error.tsx            — Error boundary (must be 'use client')
├── not-found.tsx        — 404 for this segment
├── template.tsx         — Like layout but re-mounts on navigation (rare)
├── (marketing)/         — Route group — groups without URL segment
│   ├── layout.tsx       — Layout for this group only
│   └── about/page.tsx   → /about
├── blog/
│   ├── page.tsx         → /blog
│   └── [slug]/
│       └── page.tsx     → /blog/anything
├── products/
│   └── [...slug]/
│       └── page.tsx     → /products/a/b/c (catch-all)
└── api/
    └── webhooks/
        └── stripe/
            └── route.ts → POST /api/webhooks/stripe
```

```tsx
// error.tsx — must be 'use client'
'use client';
import { useEffect } from 'react';

export default function Error({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    // Log to your error tracking (Sentry, etc.)
    console.error(error);
  }, [error]);

  return (
    <div>
      <h2>Something went wrong</h2>
      <button onClick={() => reset()}>Try again</button>
    </div>
  );
}
```

---

### 9. Image and font optimization

```tsx
import Image from 'next/image';

// ✅ Always use next/image for automatic optimization:
// - WebP/AVIF conversion
// - Lazy loading by default
// - Prevents layout shift (requires width + height or fill)
export function ProductImage({ src, alt }: { src: string; alt: string }) {
  return (
    <div className="relative aspect-square">
      <Image
        src={src}
        alt={alt}
        fill
        sizes="(max-width: 768px) 100vw, (max-width: 1200px) 50vw, 33vw"
        className="object-cover"
        priority={false} // set true only for above-the-fold hero images
      />
    </div>
  );
}
```

```typescript
// app/layout.tsx — fonts loaded once, zero layout shift
import { Inter, JetBrains_Mono } from 'next/font/google';

const inter = Inter({
  subsets: ['latin'],
  variable: '--font-inter',
  display: 'swap',
});

const mono = JetBrains_Mono({
  subsets: ['latin'],
  variable: '--font-mono',
  display: 'swap',
});

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" className={`${inter.variable} ${mono.variable}`}>
      <body>{children}</body>
    </html>
  );
}
```

---

### 10. Type-safe navigation

```typescript
// lib/routes.ts — single source of truth for all app routes
export const Routes = {
  home: '/',
  products: '/products',
  product: (id: string) => `/products/${id}`,
  productEdit: (id: string) => `/products/${id}/edit`,
  blog: '/blog',
  blogPost: (slug: string) => `/blog/${slug}`,
  checkout: '/checkout',
} as const;
```

```tsx
// Usage — no magic strings scattered everywhere
import { Routes } from '@/lib/routes';
import Link from 'next/link';

<Link href={Routes.product(product.id)}>View Product</Link>
```

---

## Anti-Patterns

### BAD: `useEffect` for data fetching in Server Component context

```tsx
// ❌ NEVER — This is the old React pattern. In App Router, fetch in Server Components.
'use client';
export function ProductList() {
  const [products, setProducts] = useState([]);
  useEffect(() => {
    fetch('/api/products').then(r => r.json()).then(setProducts);
  }, []);
  // Problems: loading flash, no SSR, client roundtrip, SEO impact
}
```

```tsx
// ✅ CORRECT — Server Component, direct fetch, no JS needed
async function ProductList() {
  const products = await db.product.findMany();
  return <ul>{products.map(p => <ProductCard key={p.id} product={p} />)}</ul>;
}
```

### BAD: Putting everything in a single Client Component

```tsx
// ❌ Forces the entire component tree to hydrate on the client
'use client';
export default function ProductPage({ params }) {
  // You only needed 'use client' for the button!
  // But now the entire page, including static content, ships as JS
  const [inCart, setInCart] = useState(false);
  const product = use(fetchProduct(params.id)); // unnecessary client fetch
  // ... entire page renders on client
}
```

```tsx
// ✅ CORRECT — push 'use client' to the leaves
// Server Component renders the page, only the interactive button is a Client Component
async function ProductPage({ params }) {
  const product = await db.product.findUnique({ where: { id: params.id } });
  return (
    <article>
      <h1>{product.name}</h1>           {/* static, no JS */}
      <p>{product.description}</p>      {/* static, no JS */}
      <AddToCartButton id={product.id} /> {/* 'use client' only here */}
    </article>
  );
}
```

### BAD: Ignoring layout nesting for shared UI

```tsx
// ❌ Repeating the same header/sidebar in every page
export default function DashboardPage() {
  return (
    <>
      <DashboardHeader />  {/* duplicated everywhere */}
      <DashboardSidebar /> {/* duplicated everywhere */}
      <main>...</main>
    </>
  );
}
```

```tsx
// ✅ CORRECT — persistent UI in layout.tsx
// app/dashboard/layout.tsx
export default function DashboardLayout({ children }) {
  return (
    <>
      <DashboardHeader />
      <DashboardSidebar />
      <main>{children}</main>
    </>
  );
}
// Sidebar and header render once — they don't unmount between navigations
```

---

## Community Skills

| Skill | Install command |
|---|---|
| [Vercel React Best Practices](https://skills.sh/vercel-labs/agent-skills/vercel-react-best-practices) | `npx skills add vercel-labs/agent-skills --skill vercel-react-best-practices -a claude-code` |
| [React Composition Patterns](https://skills.sh/vercel-labs/agent-skills/vercel-composition-patterns) | `npx skills add vercel-labs/agent-skills --skill vercel-composition-patterns -a claude-code` |
| [Next.js Best Practices](https://skills.sh/vercel-labs/next-skills/next-best-practices) | `npx skills add vercel-labs/next-skills --skill next-best-practices -a claude-code` |
| [Next.js Caching & Components](https://skills.sh/vercel-labs/next-skills/next-cache-components) | `npx skills add vercel-labs/next-skills --skill next-cache-components -a claude-code` |
| [Next.js Upgrade Guide](https://skills.sh/vercel-labs/next-skills/next-upgrade) | `npx skills add vercel-labs/next-skills --skill next-upgrade -a claude-code` |
| [Vitest](https://skills.sh/antfu/skills/vitest) | `npx skills add antfu/skills --skill vitest -a claude-code` |
| [Tailwind v4 + shadcn/ui (combo)](https://skills.sh/secondsky/claude-skills/tailwind-v4-shadcn) | `npx skills add secondsky/claude-skills --skill tailwind-v4-shadcn -a claude-code` |
| [Tailwind CSS Patterns](https://skills.sh/giuseppe-trisciuoglio/developer-kit/tailwind-css-patterns) | `npx skills add giuseppe-trisciuoglio/developer-kit --skill tailwind-css-patterns -a claude-code` |
