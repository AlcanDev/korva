---
id: community-skills
version: 1.1.0
team: all
stack: React, Next.js, Vue, Nuxt, Angular, Astro, TypeScript, NestJS, Go, Python, Flutter, Swift, and 60+ more
skills_registry: https://skills.sh
---

# Scroll: Korva Community Skills Registry

## Triggers — load when:
- Files: `package.json`, `go.mod`, `go.work`, `pyproject.toml`, `requirements.txt`, `pubspec.yaml`, `Cargo.toml`, `Gemfile`, `composer.json`, `pom.xml`, `*.gradle`, `Package.swift`
- Keywords: skills.sh, skill registry, community skills, stack detection, korva lore skills
- Tasks: setting up a new project, finding best practices for a stack, discovering available skills for a technology

## Context
The Korva Community Skills Registry maps technologies to curated AI agent skills hosted on [skills.sh](https://skills.sh). Each skill is a markdown file with best practices, patterns, and guidelines for a specific technology — exactly like Korva scrolls, but sourced from the community. This scroll is the bridge: it tells Korva which community skills exist for which stacks, so you can install them alongside your team's private scrolls.

Install skills individually for Claude Code:
```bash
npx skills add owner/repo --skill skill-id -a claude-code
```

Or use `korva lore skills` (coming in v0.2.0) to detect your stack and install all matching skills automatically.

---

## Skills Registry

Skills are referenced as `owner/repo/skill-id` from [skills.sh](https://skills.sh).
Use `npx skills add owner/repo --skill skill-id` to install any of them.

---

### Frontend Frameworks

| Technology | Detects via | Skills |
|---|---|---|
| **React** | `react`, `react-dom` in package.json | `vercel-labs/agent-skills/vercel-react-best-practices` · `vercel-labs/agent-skills/vercel-composition-patterns` |
| **Next.js** | `next` package, `next.config.*` | `vercel-labs/next-skills/next-best-practices` · `vercel-labs/next-skills/next-cache-components` · `vercel-labs/next-skills/next-upgrade` |
| **Vue** | `vue` package | `hyf0/vue-skills/vue-best-practices` · `hyf0/vue-skills/vue-debug-guides` · `antfu/skills/vue` · `antfu/skills/vue-best-practices` |
| **Nuxt** | `nuxt` package, `nuxt.config.*` | `antfu/skills/nuxt` |
| **Pinia** | `pinia` package | `vuejs-ai/skills/vue-pinia-best-practices` |
| **Svelte / SvelteKit** | `svelte`, `@sveltejs/kit` package | `ejirocodes/agent-skills/svelte5-best-practices` · `sveltejs/ai-tools/svelte-code-writer` |
| **Angular** | `@angular/core`, `angular.json` | `angular/skills/angular-developer` · `angular/angular/reference-core` · `angular/angular/reference-signal-forms` · `angular/angular/reference-compiler-cli` |
| **Astro** | `astro` package, `astro.config.*` | `astrolicious/agent-skills/astro` |
| **TanStack Start** | `@tanstack/react-start` package | `tanstack-skills/tanstack-skills/tanstack-start` |
| **React Router** | `react-router`, `@react-router/*` packages | — (no skills yet) |

---

### Styling & UI

| Technology | Detects via | Skills |
|---|---|---|
| **Tailwind CSS** | `tailwindcss`, `@tailwindcss/vite` package, `tailwind.config.*` | `giuseppe-trisciuoglio/developer-kit/tailwind-css-patterns` |
| **shadcn/ui** | `components.json` in project root | `shadcn/ui/shadcn` |
| **GSAP** | `gsap` package | `greensock/gsap-skills/gsap-core` · `greensock/gsap-skills/gsap-scrolltrigger` · `greensock/gsap-skills/gsap-performance` · `greensock/gsap-skills/gsap-plugins` · `greensock/gsap-skills/gsap-timeline` · `greensock/gsap-skills/gsap-utils` · `greensock/gsap-skills/gsap-frameworks` |
| **Three.js** | `three` package | `cloudai-x/threejs-skills/threejs-fundamentals` · `cloudai-x/threejs-skills/threejs-animation` · `cloudai-x/threejs-skills/threejs-shaders` · `cloudai-x/threejs-skills/threejs-geometry` · `cloudai-x/threejs-skills/threejs-interaction` · `cloudai-x/threejs-skills/threejs-materials` · `cloudai-x/threejs-skills/threejs-postprocessing` · `cloudai-x/threejs-skills/threejs-lighting` · `cloudai-x/threejs-skills/threejs-textures` · `cloudai-x/threejs-skills/threejs-loaders` |
| **Remotion** | `remotion`, `@remotion/cli` packages | `remotion-dev/skills/remotion-best-practices` |

---

### Languages & Runtimes

| Technology | Detects via | Skills |
|---|---|---|
| **TypeScript** | `typescript` package, `tsconfig.json` | `wshobson/agents/typescript-advanced-types` |
| **Node.js** | `package-lock.json`, `yarn.lock`, `pnpm-lock.yaml`, `.nvmrc` | `wshobson/agents/nodejs-backend-patterns` · `sickn33/antigravity-awesome-skills/nodejs-best-practices` |
| **Go** | `go.mod`, `go.work` | `affaan-m/everything-claude-code/golang-patterns` · `affaan-m/everything-claude-code/golang-testing` |
| **Bun** | `bun.lockb`, `bun.lock`, `bunfig.toml` | `https://bun.sh/docs` |
| **Deno** | `deno.json`, `deno.jsonc`, `deno.lock` | `denoland/skills/deno-expert` · `denoland/skills/deno-guidance` · `denoland/skills/deno-frontend` · `denoland/skills/deno-deploy` · `denoland/skills/deno-sandbox` · `mindrally/skills/deno-typescript` |
| **Rust** | `Cargo.toml` | `apollographql/skills/rust-best-practices` |
| **PHP** | `composer.json`, `composer.lock` | `jeffallan/claude-skills/php-pro` |

---

### Backend Frameworks

| Technology | Detects via | Skills |
|---|---|---|
| **NestJS** | `@nestjs/core` package | `kadajett/agent-nestjs-skills/nestjs-best-practices` |
| **Hono** | `hono` package | `yusukebe/hono-skill/hono` |
| **Express** | `express` package | — (use combo: node-express) |
| **Laravel** | `artisan`, `bootstrap/app.php`, `"laravel/framework"` in composer.json | `jeffallan/claude-skills/laravel-specialist` · `affaan-m/everything-claude-code/laravel-patterns` |
| **Spring Boot** | `spring-boot-starter` in pom.xml | `github/awesome-copilot/java-springboot` |
| **Java** | `pom.xml`, Gradle files | `github/awesome-copilot/java-docs` · `affaan-m/everything-claude-code/java-coding-standards` |

---

### Python Ecosystem

| Technology | Detects via | Skills |
|---|---|---|
| **Python** | `pyproject.toml`, `requirements.txt`, `setup.py`, `Pipfile` | `inferen-sh/skills/python-executor` · `wshobson/agents/python-testing-patterns` |
| **FastAPI** | `fastapi` in requirements | `wshobson/agents/fastapi-templates` · `mindrally/skills/fastapi-python` · `jezweb/claude-skills/fastapi` |
| **Django** | `django` in requirements, `manage.py` | `vintasoftware/django-ai-plugins/django-expert` · `affaan-m/everything-claude-code/django-patterns` · `affaan-m/everything-claude-code/django-security` |
| **Flask** | `flask` in requirements | `jezweb/claude-skills/flask` · `aj-geddes/useful-ai-prompts/flask-api-development` |
| **Pydantic** | `pydantic` in requirements | `bobmatnyc/claude-mpm-skills/pydantic` |
| **SQLAlchemy** | `sqlalchemy` in requirements | `bobmatnyc/claude-mpm-skills/sqlalchemy-orm` · `wispbit-ai/skills/sqlalchemy-alembic-expert-best-practices-code-review` |
| **Pytest** | `pytest` in requirements | `wshobson/agents/python-testing-patterns` |
| **Pandas** | `pandas` in requirements | `jeffallan/claude-skills/pandas-pro` · `pluginagentmarketplace/custom-plugin-python/pandas-data-analysis` |
| **NumPy** | `numpy` in requirements | `pluginagentmarketplace/custom-plugin-python/machine-learning` |
| **Scikit-Learn** | `scikit-learn` in requirements | `davila7/claude-code-templates/scikit-learn` · `davila7/claude-code-templates/senior-data-scientist` |
| **Celery** | `celery` in requirements | `wshobson/agents/python-background-jobs` |

---

### Ruby Ecosystem

| Technology | Detects via | Skills |
|---|---|---|
| **Ruby** | `Gemfile`, `.ruby-version` | `lucianghinda/superpowers-ruby/ruby` |
| **Rails** | `config/routes.rb`, `rails` gem | `sergiodxa/agent-skills/ruby-on-rails-best-practices` · `lucianghinda/superpowers-ruby/rails-guides` · `igmarin/rails-agent-skills/rails-stack-conventions` · `igmarin/rails-agent-skills/rails-code-review` · `igmarin/rails-agent-skills/rails-migration-safety` · `igmarin/rails-agent-skills/rails-security-review` · `ombulabs/claude-code_rails-upgrade-skill/rails-upgrade` |
| **RSpec** | `rspec` gem, `.rspec` config | `igmarin/rails-agent-skills/rspec-best-practices` · `igmarin/rails-agent-skills/rspec-service-testing` · `lucianghinda/superpowers-ruby/test-driven-development` |
| **Sidekiq** | `sidekiq` gem | `igmarin/rails-agent-skills/rails-background-jobs` |
| **Sorbet** | `sorbet` gem, `sorbet/config` | `DmitryPogrebnoy/ruby-agent-skills/generating-sorbet` · `DmitryPogrebnoy/ruby-agent-skills/generating-sorbet-inline` |
| **Redis (Ruby)** | `redis`, `sidekiq` gems | `redis/agent-skills/redis-development` |

---

### Mobile & Desktop

| Technology | Detects via | Skills |
|---|---|---|
| **Expo** | `expo` package | `expo/skills/building-native-ui` · `expo/skills/native-data-fetching` · `expo/skills/upgrading-expo` · `expo/skills/expo-tailwind-setup` · `expo/skills/expo-dev-client` · `expo/skills/expo-deployment` · `expo/skills/expo-cicd-workflows` · `expo/skills/expo-api-routes` · `expo/skills/use-dom` |
| **React Native** | `react-native` package | `sleekdotdesign/agent-skills/sleek-design-mobile-apps` |
| **Flutter** | `flutter:` in `pubspec.yaml` | `jeffallan/claude-skills/flutter-expert` · `madteacher/mad-agents-skills/flutter-animations` · `madteacher/mad-agents-skills/flutter-testing` |
| **Dart** | `pubspec.yaml` | `kevmoo/dash_skills/dart-best-practices` |
| **SwiftUI** | `Package.swift` | `avdlee/swiftui-agent-skill/swiftui-expert-skill` · `avdlee/swift-concurrency-agent-skill` · `avdlee/xcode-build-optimization-agent-skill` · `avdlee/swift-testing-agent-skill` · `avdlee/core-data-agent-skill` |
| **Android** | Gradle files with `com.android.application` | `krutikJain/android-agent-skills/android-kotlin-core` · `krutikJain/android-agent-skills/android-compose-foundations` · `krutikJain/android-agent-skills/android-architecture-clean` · `krutikJain/android-agent-skills/android-di-hilt` · `krutikJain/android-agent-skills/android-gradle-build-logic` · `krutikJain/android-agent-skills/android-coroutines-flow` · `krutikJain/android-agent-skills/android-networking-retrofit-okhttp` · `krutikJain/android-agent-skills/android-testing-unit` |
| **Kotlin Multiplatform** | `kotlin("multiplatform")` in Gradle | `Kotlin/kotlin-agent-skills/kotlin-tooling-cocoapods-spm-migration` · `Kotlin/kotlin-agent-skills/kotlin-tooling-agp9-migration` |
| **Tauri** | `@tauri-apps/api` package, `src-tauri/tauri.conf.json` | `nodnarbnitram/claude-code-extensions/tauri-v2` |
| **Electron** | `electron` package, `electron-vite.config.*` | `vercel-labs/agent-skills/electron-best-practices` |
| **Chrome Extension** | `manifest_version` in `manifest.json` | `mindrally/skills/chrome-extension-development` |

---

### Data & Storage

| Technology | Detects via | Skills |
|---|---|---|
| **Supabase** | `@supabase/supabase-js` package | `supabase/agent-skills/supabase-postgres-best-practices` |
| **Neon Postgres** | `@neondatabase/serverless` package | `neondatabase/agent-skills/neon-postgres` |
| **Prisma** | `prisma`, `@prisma/client` packages | `prisma/skills/prisma-database-setup` · `prisma/skills/prisma-client-api` · `prisma/skills/prisma-cli` · `prisma/skills/prisma-postgres` |
| **Drizzle ORM** | `drizzle-orm`, `drizzle-kit` packages | `bobmatnyc/claude-mpm-skills/drizzle-orm` |
| **Zod** | `zod` package | `pproenca/dot-skills/zod` |
| **React Hook Form** | `react-hook-form` package | `pproenca/dot-skills/react-hook-form` |

---

### Auth & Payments

| Technology | Detects via | Skills |
|---|---|---|
| **Clerk** | `@clerk/*` packages | `clerk/skills/clerk` · `clerk/skills/clerk-setup` · `clerk/skills/clerk-custom-ui` · `clerk/skills/clerk-backend-api` · `clerk/skills/clerk-orgs` · `clerk/skills/clerk-webhooks` · `clerk/skills/clerk-testing` |
| **Better Auth** | `better-auth` package | `better-auth/skills/better-auth-best-practices` · `better-auth/skills/email-and-password-best-practices` · `better-auth/skills/organization-best-practices` · `better-auth/skills/two-factor-authentication-best-practices` |
| **Stripe** | `stripe`, `@stripe/stripe-js` packages | `stripe/ai/stripe-best-practices` · `stripe/ai/upgrade-stripe` |

---

### Testing

| Technology | Detects via | Skills |
|---|---|---|
| **Playwright** | `@playwright/test`, `playwright.config.*` | `currents-dev/playwright-best-practices-skill/playwright-best-practices` |
| **Vitest** | `vitest` package, `vitest.config.*` | `antfu/skills/vitest` |
| **oxlint** | `oxlint` package, `.oxlintrc.json` | `delexw/claude-code-misc/oxlint` |

---

### Cloud & Infrastructure

| Technology | Detects via | Skills |
|---|---|---|
| **Vercel** | `vercel.json`, `.vercel`, `vercel` package | `vercel-labs/agent-skills/deploy-to-vercel` |
| **Vercel AI SDK** | `ai`, `@ai-sdk/*` packages | `vercel/ai/ai-sdk` |
| **Cloudflare** | `wrangler` package, `wrangler.toml/json` | `cloudflare/skills/cloudflare` · `cloudflare/skills/wrangler` · `cloudflare/skills/workers-best-practices` · `cloudflare/skills/web-perf` |
| **Cloudflare Durable Objects** | `durable_objects` in wrangler config | `cloudflare/skills/durable-objects` |
| **Cloudflare Agents** | `agents` package | `cloudflare/skills/agents-sdk` · `cloudflare/skills/building-mcp-server-on-cloudflare` · `cloudflare/skills/sandbox-sdk` |
| **Cloudflare AI** | `@cloudflare/ai` package | `cloudflare/skills/building-ai-agent-on-cloudflare` |
| **Azure** | `@azure/*` packages | `microsoft/github-copilot-for-azure/azure-deploy` · `microsoft/github-copilot-for-azure/azure-ai` · `microsoft/github-copilot-for-azure/azure-cost-optimization` · `microsoft/github-copilot-for-azure/azure-diagnostics` |
| **Terraform** | `main.tf`, `.terraform.lock.hcl` | `hashicorp/agent-skills/terraform-style-guide` · `hashicorp/agent-skills/refactor-module` · `hashicorp/agent-skills/terraform-stacks` · `wshobson/agents/terraform-module-library` |

---

### Build Tools

| Technology | Detects via | Skills |
|---|---|---|
| **Vite** | `vite` package, `vite.config.*` | `antfu/skills/vite` |
| **Turborepo** | `turbo` package, `turbo.json` | `vercel/turborepo/turborepo` |
| **WordPress** | `wp-config.php`, `@wordpress/*` packages | `wordpress/agent-skills/wp-plugin-development` · `wordpress/agent-skills/wp-rest-api` · `wordpress/agent-skills/wp-block-themes` · `wordpress/agent-skills/wp-block-development` · `wordpress/agent-skills/wp-performance` |

---

### AI & Media

| Technology | Detects via | Skills |
|---|---|---|
| **ElevenLabs** | `elevenlabs` package | `inferen-sh/skills/elevenlabs-tts` · `inferen-sh/skills/elevenlabs-music` |

---

## Cross-Technology Combo Skills

Some skills only fire when multiple technologies are detected together:

| Combo | Requires | Skills |
|---|---|---|
| React + shadcn/ui | react + shadcn | `shadcn/ui/shadcn` · `vercel-labs/agent-skills/vercel-react-best-practices` |
| Tailwind + shadcn | tailwind + shadcn | `secondsky/claude-skills/tailwind-v4-shadcn` |
| Next.js + Supabase | nextjs + supabase | `supabase/agent-skills/supabase-postgres-best-practices` |
| Next.js + Vercel AI | nextjs + vercel-ai | `vercel/ai/ai-sdk` · `vercel-labs/next-skills/next-best-practices` |
| Next.js + Playwright | nextjs + playwright | `currents-dev/playwright-best-practices-skill/playwright-best-practices` |
| Next.js + Clerk | nextjs + clerk | `clerk/skills/clerk-nextjs-patterns` |
| React + Three Fiber | threejs + react | `vercel-labs/json-render/react-three-fiber` |
| GSAP + React | gsap + react | `greensock/gsap-skills/gsap-react` |
| React Hook Form + Zod | react-hook-form + zod | `jezweb/claude-skills/react-hook-form-zod` · `pproenca/dot-skills/zod` |
| Expo + Tailwind | expo + tailwind | `expo/skills/expo-tailwind-setup` |
| Expo + React Native | expo + react-native | `expo/skills/building-native-ui` · `sleekdotdesign/agent-skills/sleek-design-mobile-apps` |
| Node.js + Express | node + express | `aj-geddes/useful-ai-prompts/nodejs-express-server` |
| Cloudflare + Vite | cloudflare + vite | `cloudflare/vinext/migrate-to-vinext` |
| Rails + RSpec | rails + rspec | `igmarin/rails-agent-skills/rails-tdd-slices` · `igmarin/rails-agent-skills/rails-bug-triage` |
| Nuxt + Clerk | nuxt + clerk | `clerk/skills/clerk-nuxt-patterns` |
| Vue + Clerk | vue + clerk | `clerk/skills/clerk-vue-patterns` |

---

## Community Skills vs. Korva Lore

Both deliver knowledge to your AI — from different layers:

| | **Community Skills** | **Korva Lore** |
|---|---|---|
| **Source** | Community (skills.sh) | Community + your private team scrolls |
| **Detection** | Installed once per project | Auto on file open + always-on via MCP |
| **Persistence** | Local files in `.claude/skills/` | Injected dynamically at session time |
| **Team knowledge** | Not supported | Core feature (Team Profiles) |
| **Customization** | Not supported | Full (custom scrolls, triggers) |

**The recommended workflow** — use both layers:

```bash
# 1. Install community skills for your stack (one-time per project)
npx skills add vercel-labs/next-skills --skill next-best-practices -a claude-code
npx skills add stripe/ai --skill stripe-best-practices -a claude-code

# 2. Add your team's private scrolls via Korva (synced across the team)
korva init --profile git@github.com:YOUR-ORG/korva-team-profile.git

# 3. Start a session — Korva injects team scrolls, community skills provide static best practices
# Your AI has both layers: community knowledge + team-specific patterns
```

**`korva lore skills` (coming in v0.2.0):**
```bash
# Detect your stack and install matching skills as Korva community scrolls
korva lore skills

# Dry run — show what would be installed
korva lore skills --dry-run

# Install skills for a specific technology
korva lore skills --tech nextjs,stripe,playwright
```

This command will detect your stack, pull matching skills from skills.sh, and store them as Korva community scrolls — searchable via `vault_search` and auto-loaded by the Lore engine when you open relevant files.
