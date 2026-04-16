---
id: payments-stripe
version: 1.0.0
team: backend
stack: Stripe, Node.js, TypeScript, Python, any
---

# Scroll: Stripe Payments — Battle-Tested Patterns

## Triggers — load when:
- Files: `checkout.ts`, `payment.ts`, `stripe.ts`, `billing.ts`, `subscription.ts`, `invoice.ts`, `webhook.ts`, `payment*.ts`, `*stripe*.ts`, `*billing*.ts`
- Keywords: `stripe`, `webhook`, `charge`, `payment_intent`, `paymentIntent`, `checkout.session`, `subscription`, `invoice`, `idempotency`, `card_number`, `cvv`, `pci`
- Tasks: implementing payments, handling Stripe webhooks, processing charges, managing subscriptions, PCI compliance

## Context
Money is the one place where silent bugs become lawsuits. These patterns cover the failure modes that affect real production systems: duplicate charges, webhook replay attacks, race conditions on concurrent payments, floating-point rounding errors, and PCI audit failures from inadvertent card data logging. Every rule here has a corresponding production incident somewhere in the industry.

---

## Rules

### 1. Always verify webhook signatures before processing

Never trust the content of a webhook request without first verifying Stripe's HMAC-SHA256 signature. An attacker who can POST to your webhook endpoint can fabricate `payment_intent.succeeded` events and trigger fulfillment without paying.

```typescript
import Stripe from 'stripe';
import { Request, Response } from 'express';

const stripe = new Stripe(process.env.STRIPE_SECRET_KEY!, { apiVersion: '2024-06-20' });

export async function stripeWebhookHandler(req: Request, res: Response) {
  const sig = req.headers['stripe-signature'] as string;
  const webhookSecret = process.env.STRIPE_WEBHOOK_SECRET!;

  let event: Stripe.Event;
  try {
    // req.body MUST be the raw buffer — do NOT parse JSON before this step
    event = stripe.webhooks.constructEvent(req.body, sig, webhookSecret);
  } catch (err) {
    // Return 400 so Stripe retries — do not return 200 on verification failure
    return res.status(400).send(`Webhook signature verification failed: ${(err as Error).message}`);
  }

  await processWebhookEvent(event);
  res.json({ received: true });
}
```

```python
# Python equivalent
import stripe
from flask import request, abort

@app.route('/webhooks/stripe', methods=['POST'])
def stripe_webhook():
    payload = request.get_data()
    sig_header = request.headers.get('Stripe-Signature')
    webhook_secret = os.environ['STRIPE_WEBHOOK_SECRET']

    try:
        event = stripe.Webhook.construct_event(payload, sig_header, webhook_secret)
    except stripe.error.SignatureVerificationError:
        abort(400)

    process_event(event)
    return '', 200
```

**Critical:** Configure Express to expose the raw body for the webhook route only:
```typescript
// Mount raw body parser BEFORE the JSON parser for /webhooks/stripe
app.use('/webhooks/stripe', express.raw({ type: 'application/json' }));
app.use(express.json()); // all other routes
```

---

### 2. Idempotency keys on every mutating Stripe API call

Every charge, payment intent creation, refund, or subscription modification must carry an idempotency key. Without one, a network timeout followed by a retry creates a duplicate charge.

```typescript
import { randomUUID } from 'crypto';

// Derive the key from a stable business identifier — NOT a random UUID generated at call time
// (a random UUID on retry = a new key = a new charge)
async function createPaymentIntent(orderId: string, amountCents: number, currency: string) {
  // Key must be deterministic for the same logical operation
  const idempotencyKey = `pi-${orderId}-${currency}`;

  const intent = await stripe.paymentIntents.create(
    {
      amount: amountCents,
      currency,
      metadata: { orderId },
    },
    { idempotencyKey }
  );

  return intent;
}

// For refunds, tie the key to the charge + refund reason
async function refundCharge(chargeId: string, refundReason: string) {
  const idempotencyKey = `refund-${chargeId}-${refundReason}`;
  return stripe.refunds.create({ charge: chargeId }, { idempotencyKey });
}
```

**Rule of thumb:** `idempotencyKey = <resource-type>-<stable-business-id>`. Never `randomUUID()` at call time.

---

### 3. Never use floating-point arithmetic for money

IEEE 754 doubles cannot represent most decimal fractions exactly. `0.1 + 0.2 === 0.30000000000000004`. A single rounding error compounded across thousands of transactions produces invisible discrepancies in financial reports and reconciliation failures.

```typescript
import Decimal from 'decimal.js';

// BAD — float arithmetic
const subtotal = 19.99;
const tax = subtotal * 0.08; // 1.5992000000000002
const total = subtotal + tax; // 21.589200000000002

// GOOD — integer cents for Stripe (Stripe always works in the smallest currency unit)
// Store and compute in cents as integers
const subtotalCents = 1999; // $19.99
const taxCents = Math.round(subtotalCents * 0.08); // 160 — round once, at the end
const totalCents = subtotalCents + taxCents; // 2159

// When you need decimal display / multi-step calculations, use Decimal.js
const price = new Decimal('19.99');
const taxRate = new Decimal('0.08');
const tax2 = price.mul(taxRate).toDecimalPlaces(2); // Decimal('1.60')
const total2 = price.plus(tax2); // Decimal('21.59')

// Convert to cents for Stripe
const stripeAmount = total2.mul(100).toInteger().toNumber(); // 2159
```

**Rule:** Accept `string` or `Decimal` from APIs/databases for monetary values. Convert to integer cents only at the Stripe API boundary.

---

### 4. Idempotent webhook handlers — always

Stripe retries webhook delivery up to 25 times over 3 days when your endpoint returns a non-2xx response or times out. Your handler will receive the same event multiple times. Processing `payment_intent.succeeded` twice = charging twice, fulfilling twice, or sending duplicate receipts.

```typescript
import { db } from '../db';

async function processWebhookEvent(event: Stripe.Event) {
  // Step 1: deduplicate by event ID
  const processed = await db.webhookEvents.findUnique({ where: { stripeEventId: event.id } });
  if (processed) {
    // Already handled — acknowledge without re-processing
    return;
  }

  // Step 2: process atomically — mark as processed AND fulfill in the same transaction
  await db.$transaction(async (tx) => {
    // Insert first — if this transaction is replayed, the unique constraint fires
    await tx.webhookEvents.create({
      data: { stripeEventId: event.id, type: event.type, processedAt: new Date() }
    });

    switch (event.type) {
      case 'payment_intent.succeeded': {
        const intent = event.data.object as Stripe.PaymentIntent;
        await fulfillOrder(tx, intent.metadata.orderId);
        break;
      }
      case 'payment_intent.payment_failed': {
        const intent = event.data.object as Stripe.PaymentIntent;
        await markOrderFailed(tx, intent.metadata.orderId, intent.last_payment_error?.message);
        break;
      }
    }
  });
}
```

**Schema requirement:** `webhook_events` table with a `UNIQUE` constraint on `stripe_event_id`.

---

### 5. Distributed lock for race conditions on concurrent payment attempts

When the same order can be paid from two browser tabs simultaneously (or a user double-taps "Pay"), two threads can both pass the "order is unpaid" check and both call `stripe.paymentIntents.create`. Result: double charge.

```typescript
import { Redis } from 'ioredis';
import Redlock from 'redlock';

const redlock = new Redlock([new Redis(process.env.REDIS_URL!)], {
  retryCount: 3,
  retryDelay: 200,
});

async function initiatePayment(orderId: string, amountCents: number) {
  const lockKey = `payment-lock:${orderId}`;
  const lockTtl = 10_000; // 10 seconds — enough for the Stripe API call

  let lock;
  try {
    lock = await redlock.acquire([lockKey], lockTtl);
  } catch {
    throw new ConflictError('Payment already in progress for this order. Please wait.');
  }

  try {
    // Check idempotently inside the lock
    const existing = await db.paymentIntents.findUnique({ where: { orderId } });
    if (existing) return existing; // second caller gets the already-created intent

    const intent = await stripe.paymentIntents.create(
      { amount: amountCents, currency: 'usd', metadata: { orderId } },
      { idempotencyKey: `pi-${orderId}` }
    );

    await db.paymentIntents.create({ data: { orderId, stripeId: intent.id } });
    return intent;
  } finally {
    await lock.release();
  }
}
```

---

### 6. PCI-DSS: never log, store, or transmit raw card data

If your code ever touches a raw PAN (Primary Account Number), CVV, or full track data you are in PCI scope. The moment any of these values appears in a log line, your PCI audit fails and you face potential fines.

```typescript
// These fields must NEVER appear in logs, databases, or error reports
const FORBIDDEN_CARD_FIELDS = ['card_number', 'cvv', 'cvc', 'track_data', 'pan'];

// CORRECT approach: let Stripe.js tokenize card data client-side
// Your server never sees the raw card number — only a payment method ID
async function confirmPayment(paymentMethodId: string, amountCents: number) {
  // paymentMethodId = "pm_xxxxx" — a token, not card data
  const intent = await stripe.paymentIntents.create({
    amount: amountCents,
    currency: 'usd',
    payment_method: paymentMethodId,
    confirm: true,
  }, { idempotencyKey: `confirm-${paymentMethodId}-${amountCents}` });

  // Safe to log — no card data
  logger.info('payment_intent_created', {
    intentId: intent.id,
    status: intent.status,
    amountCents,
    // DO NOT log: paymentMethodId details that could expose card BIN/last4 in bulk
  });

  return intent;
}
```

**Server-side rule:** Your server should only ever see `pm_*` (PaymentMethod ID), `pi_*` (PaymentIntent ID), `cs_*` (CheckoutSession ID). If you see a 16-digit number, something is wrong.

---

### 7. Exponential backoff with jitter on Stripe API 429s

Stripe rate-limits at the account level. During payment spikes (flash sales, subscription renewals), hitting 429s and retrying immediately amplifies the problem. Exponential backoff with jitter spreads the load.

```typescript
import Stripe from 'stripe';

const stripe = new Stripe(process.env.STRIPE_SECRET_KEY!, {
  apiVersion: '2024-06-20',
  maxNetworkRetries: 3, // Stripe SDK built-in retry with exponential backoff
  timeout: 10_000,       // 10s timeout — Stripe SLA is 99.99% under 5s
});

// For custom retry logic (e.g., in a queue worker):
async function withStripeRetry<T>(
  operation: () => Promise<T>,
  maxAttempts = 5
): Promise<T> {
  for (let attempt = 1; attempt <= maxAttempts; attempt++) {
    try {
      return await operation();
    } catch (err) {
      if (err instanceof Stripe.errors.StripeRateLimitError && attempt < maxAttempts) {
        const baseDelay = 1000 * Math.pow(2, attempt - 1); // 1s, 2s, 4s, 8s
        const jitter = Math.random() * 500;                 // 0–500ms random jitter
        await new Promise(resolve => setTimeout(resolve, baseDelay + jitter));
        continue;
      }
      throw err; // non-retryable error — propagate immediately
    }
  }
  throw new Error('Max retry attempts exceeded');
}

// Usage
const charge = await withStripeRetry(() =>
  stripe.paymentIntents.confirm(intentId, { idempotencyKey: `confirm-${orderId}` })
);
```

---

### 8. Webhook event ordering — do not assume sequence

Stripe delivers webhooks in best-effort order but does NOT guarantee ordering. `payment_intent.succeeded` can arrive before `payment_intent.created`. Design your handlers to be resilient to out-of-order events.

```typescript
async function handlePaymentSucceeded(intent: Stripe.PaymentIntent) {
  // Do NOT assume the order record exists — upsert defensively
  await db.orders.upsert({
    where: { stripePaymentIntentId: intent.id },
    create: {
      stripePaymentIntentId: intent.id,
      status: 'paid',
      amountCents: intent.amount,
      orderId: intent.metadata.orderId,
      paidAt: new Date(),
    },
    update: {
      status: 'paid',
      paidAt: new Date(),
    },
  });
}
```

---

## Anti-Patterns

### BAD: Parsing webhook JSON before signature verification
```typescript
// BAD — JSON.parse destroys the raw buffer Stripe needs for HMAC verification
app.use(express.json()); // applied globally
app.post('/webhooks/stripe', (req, res) => {
  const event = req.body; // body is already a parsed object — verification impossible
  stripe.webhooks.constructEvent(req.body, sig, secret); // throws always
});
```

```typescript
// GOOD — raw body middleware scoped to the webhook route only
app.use('/webhooks/stripe', express.raw({ type: 'application/json' }));
app.post('/webhooks/stripe', (req, res) => {
  const event = stripe.webhooks.constructEvent(req.body, sig, secret); // works
});
```

### BAD: Random idempotency key per attempt
```typescript
// BAD — new UUID on every call = new charge on every retry
const intent = await stripe.paymentIntents.create(
  { amount: 999, currency: 'usd' },
  { idempotencyKey: randomUUID() } // each timeout + retry = double charge
);
```

```typescript
// GOOD — deterministic key from stable business ID
const intent = await stripe.paymentIntents.create(
  { amount: 999, currency: 'usd' },
  { idempotencyKey: `pi-${orderId}` } // retries return the same intent
);
```

### BAD: Float arithmetic for tax/total calculation
```typescript
// BAD — 0.1 + 0.2 !== 0.3 in IEEE 754
const price = 29.99;
const tax = price * 0.07; // 2.0993000000000004
const total = price + tax; // 32.089300000000005
await stripe.paymentIntents.create({ amount: Math.round(total * 100) }); // 3209 — off by 1 cent
```

```typescript
// GOOD — integer cents throughout, or Decimal.js for multi-step calculations
const priceCents = 2999;
const taxCents = Math.round(priceCents * 0.07); // 210
const totalCents = priceCents + taxCents;        // 3209 — deterministic
await stripe.paymentIntents.create({ amount: totalCents });
```

### BAD: Logging card data "for debugging"
```typescript
// BAD — card.number will appear in every log aggregator, visible to SREs
logger.debug('payment attempt', {
  userId,
  cardNumber: paymentMethod.card?.last4, // even last4 bulk logs can be PCI-problematic
  cvv: body.cvv,  // NEVER — immediate PCI violation
});
```

```typescript
// GOOD — log only opaque identifiers
logger.info('payment_method_attached', {
  userId,
  paymentMethodId, // pm_xxxxx — an opaque token, no card data
  brand: paymentMethod.card?.brand,  // 'visa' — safe
});
```

### BAD: Non-idempotent webhook handler
```typescript
// BAD — creates a second fulfillment on Stripe retry
app.post('/webhooks/stripe', async (req, res) => {
  const event = parseAndVerify(req);
  if (event.type === 'payment_intent.succeeded') {
    await fulfillOrder(event.data.object.metadata.orderId); // runs twice on retry
  }
  res.json({ ok: true });
});
```

```typescript
// GOOD — deduplicate before fulfilling
app.post('/webhooks/stripe', async (req, res) => {
  const event = parseAndVerify(req);
  const alreadyProcessed = await db.processedEvents.exists(event.id);
  if (!alreadyProcessed) {
    await db.$transaction(async (tx) => {
      await tx.processedEvents.create({ data: { id: event.id } });
      await fulfillOrder(tx, event.data.object.metadata.orderId);
    });
  }
  res.json({ ok: true });
});
```

---

## Quick Reference

| Pattern | Rule |
|---|---|
| Webhook verification | `stripe.webhooks.constructEvent(rawBody, sig, secret)` |
| Idempotency key | `<resource>-<stable-id>` — never `randomUUID()` |
| Money arithmetic | Integer cents or `Decimal.js` — never `number` |
| Card data | Never in logs, DBs, or error payloads — tokenize client-side |
| Rate limit retries | Exponential backoff with jitter — or use `maxNetworkRetries` |
| Race conditions | Distributed lock (Redlock) + idempotent DB upsert |
| Webhook idempotency | `UNIQUE(stripe_event_id)` + transactional insert |
| Event ordering | Upsert, never insert-only |

---

## Community Skills

| Skill | Install command |
|---|---|
| [Stripe Best Practices](https://skills.sh/stripe/ai/stripe-best-practices) | `npx skills add stripe/ai --skill stripe-best-practices -a claude-code` |
| [Stripe Upgrade Guide](https://skills.sh/stripe/ai/upgrade-stripe) | `npx skills add stripe/ai --skill upgrade-stripe -a claude-code` |
| [Neon Postgres](https://skills.sh/neondatabase/agent-skills/neon-postgres) | `npx skills add neondatabase/agent-skills --skill neon-postgres -a claude-code` |
| [Supabase Postgres](https://skills.sh/supabase/agent-skills/supabase-postgres-best-practices) | `npx skills add supabase/agent-skills --skill supabase-postgres-best-practices -a claude-code` |
| [Prisma Setup](https://skills.sh/prisma/skills/prisma-database-setup) | `npx skills add prisma/skills --skill prisma-database-setup -a claude-code` |
| [Prisma Client API](https://skills.sh/prisma/skills/prisma-client-api) | `npx skills add prisma/skills --skill prisma-client-api -a claude-code` |
