# Changelog

## [1.0.0](https://github.com/AlcanDev/korva/compare/v1.0.0...v1.0.0) (2026-04-30)


### release

* v1.0.0 prep — docs rewrite + new scrolls + cleanup ([#7](https://github.com/AlcanDev/korva/issues/7)) ([e92d016](https://github.com/AlcanDev/korva/commit/e92d016759015b5cc1ea4b6240918a5cda8edddb))


### 🚀 Features

* add Makefile for build, test, lint, and sync commands; update privacy model in README and roadmap targets ([66b047f](https://github.com/AlcanDev/korva/commit/66b047f7177095c9e75717c89d1172bc4fdff9a3))
* **beacon:** private admin panel at /admin ([09e2d69](https://github.com/AlcanDev/korva/commit/09e2d69cf91ffffc10dc7299a52937d83b11ce2e))
* **beacon:** v1.0.0 — i18n, KorvaLogo, tests, security hardening ([c596d6f](https://github.com/AlcanDev/korva/commit/c596d6f8501b20103067d88c4aaf9a0571228cdf))
* **behavior:** adopt Karpathy-style behavioral guidelines across all IDEs ([e46e7d4](https://github.com/AlcanDev/korva/commit/e46e7d49ad57e3dc40e40a28b65d0de03b472ee8))
* **ci:** automated release pipeline + korva update self-install ([6730365](https://github.com/AlcanDev/korva/commit/6730365e04c9db2643151a88d7e56b35bd417488))
* **cli:** add cmd/korva entry point ([110f3ef](https://github.com/AlcanDev/korva/commit/110f3ef55218f7e0eaee4bf8205c9d78658c1031))
* **cli:** korva setup command ([09e2d69](https://github.com/AlcanDev/korva/commit/09e2d69cf91ffffc10dc7299a52937d83b11ce2e))
* **deploy:** Dockerfile + docker-compose.yml ([09e2d69](https://github.com/AlcanDev/korva/commit/09e2d69cf91ffffc10dc7299a52937d83b11ce2e))
* **deploy:** production-ready Docker build for Coolify ([ec18f52](https://github.com/AlcanDev/korva/commit/ec18f52b109c909087fad1b0ad5c979f59e2655e))
* enhance CI configuration with concurrency, improved testing, and add golangci-lint setup ([fd08ee3](https://github.com/AlcanDev/korva/commit/fd08ee39df277b112824043541c8a9e8fa63b6aa))
* **enterprise:** Business tier, license-gated MCP tools, BEHAVIOR.md integrations ([6ad1134](https://github.com/AlcanDev/korva/commit/6ad113463cec155ed468d86975352ae47152c4e5))
* **hooks:** post-commit auto-sync ([09e2d69](https://github.com/AlcanDev/korva/commit/09e2d69cf91ffffc10dc7299a52937d83b11ce2e))
* initial Korva release — AI ecosystem for enterprise teams ([725ebff](https://github.com/AlcanDev/korva/commit/725ebff2eb6fb644b188a7f77758296b0325367d))
* Korva Hive + Teams + vault operational improvements ([f34f776](https://github.com/AlcanDev/korva/commit/f34f7763fe57f9534f4be5f35ea7b25d96391bbf))
* korva setup + server deployment + admin panel + auto-sync ([09e2d69](https://github.com/AlcanDev/korva/commit/09e2d69cf91ffffc10dc7299a52937d83b11ce2e))
* **license:** activate + deactivate endpoints — vault API + Beacon UI ([32cedf3](https://github.com/AlcanDev/korva/commit/32cedf3b0372128dbd2aa24ec58edb4fed55afb1))
* **teams:** complete Teams admin flow — RBAC, team-scoped scrolls, MCP session context ([5e3b41f](https://github.com/AlcanDev/korva/commit/5e3b41ffc31c57eed3fba2f9caf5958d6d83c33f))
* update testing guidelines for Jest and NestJS, enhance TypeScript practices, and introduce token efficiency principles ([fb80fd5](https://github.com/AlcanDev/korva/commit/fb80fd576cea6de54edc6a0b3094a476da18593e))
* **v1.0:** license activate/deactivate + Beacon UI v1.0 ([88caf56](https://github.com/AlcanDev/korva/commit/88caf56c08986f4b7b8516e9e9d0deff0c5658c3))
* **v1.0:** quality gates, hybrid context, security hardening ([40cf9ab](https://github.com/AlcanDev/korva/commit/40cf9ab54576e39d5b701a7093dcba4e77bc7df6))
* **v1.1:** smart skill auto-loader + caveman compression + multi-IDE manifests ([640ff1a](https://github.com/AlcanDev/korva/commit/640ff1ad85dfaa2d6224e278fc4bec25d71bd093))
* **v1.1:** store improvements, MCP profiles, hive sync, IDE integrations ([9a150c9](https://github.com/AlcanDev/korva/commit/9a150c9f2adcb25659db042ce34d75e9121d681f))
* **vault:** /api/v1/sessions/all endpoint for admin panel ([09e2d69](https://github.com/AlcanDev/korva/commit/09e2d69cf91ffffc10dc7299a52937d83b11ce2e))


### 🐛 Bug Fixes

* **ci:** regenerate package-lock.json using public npm registry ([5c8d43a](https://github.com/AlcanDev/korva/commit/5c8d43aeb11c7a1dd8353e1ae0eca71b2721cab4))
* **ci:** remove manual release-patch/minor/major targets from Makefile ([fc033dd](https://github.com/AlcanDev/korva/commit/fc033dd769b68b00b241b5394f19cd61889f8f2e))
* **ci:** resolve all CI failures (npm E401, gitleaks false positives) ([8adf41f](https://github.com/AlcanDev/korva/commit/8adf41f423e565c671216648b2fcb17ae1cd4ed9))
* **cli:** preserve int() cast on syscall.Stdin for Windows builds ([581bb53](https://github.com/AlcanDev/korva/commit/581bb530550b0495d721b30ff0374f9d1f81d53d))
* **install:** correct archive naming in scripts/ installers ([76ab1dd](https://github.com/AlcanDev/korva/commit/76ab1dd6f0acf78ea64cfe4f3089898e884f45f7))
* **lint:** gofmt alignment in integrations_test manifest map ([c18c9ce](https://github.com/AlcanDev/korva/commit/c18c9ce3d7be2a674d6f077ec3c4d615f8c43364))
* **lint:** resolve all golangci-lint v2 violations across workspace ([4661417](https://github.com/AlcanDev/korva/commit/4661417fa71d2f7a0cb79862c0a278a116d151c6))
* **mcp+beacon:** auto-load session token + pass team_id on scroll save ([be38fda](https://github.com/AlcanDev/korva/commit/be38fda6cbdcc59e06087716f0b880a2d17093d1))
* **release:** drop empty 'go generate ./...' hook from goreleaser ([6586d1f](https://github.com/AlcanDev/korva/commit/6586d1f9ede1406a2021950e0c8d92d0824ac22a))
* **release:** make Homebrew tap publishing optional ([b235370](https://github.com/AlcanDev/korva/commit/b23537095bc3171559066829f706c4e90a22e243))
* **security:** remove all company-specific internal references ([#5](https://github.com/AlcanDev/korva/issues/5)) ([71958ef](https://github.com/AlcanDev/korva/commit/71958ef8073d0bada47295aa0946d3e16d7d0a75))
* **ui:** unify DistFS type across embedui and stub builds ([49cb95c](https://github.com/AlcanDev/korva/commit/49cb95cdf0a4fb1fe039a83af069cbb117a8e02c))

## [1.0.0](https://github.com/AlcanDev/korva/compare/v0.1.2...v1.0.0) (2026-04-30)


### release

* v1.0.0 prep — docs rewrite + new scrolls + cleanup ([#7](https://github.com/AlcanDev/korva/issues/7)) ([0334902](https://github.com/AlcanDev/korva/commit/033490236ea2efafdd8975c13845c7f6be4e379f))

## [0.1.2](https://github.com/AlcanDev/korva/compare/v0.1.1...v0.1.2) (2026-04-30)


### 🐛 Bug Fixes

* **security:** remove all company-specific internal references ([#5](https://github.com/AlcanDev/korva/issues/5)) ([56933b8](https://github.com/AlcanDev/korva/commit/56933b8d18fbef366f0889d4093af8f2bf14a1b7))

## [0.1.1](https://github.com/AlcanDev/korva/compare/v0.1.0...v0.1.1) (2026-04-30)


### 🚀 Features

* add Makefile for build, test, lint, and sync commands; update privacy model in README and roadmap targets ([be13d14](https://github.com/AlcanDev/korva/commit/be13d145d1a0169c9a9658bc1af0ee43f1f5af7c))
* **beacon:** private admin panel at /admin ([1faf15d](https://github.com/AlcanDev/korva/commit/1faf15deed2ad45186b871daf645a340e6350792))
* **beacon:** v1.0.0 — i18n, KorvaLogo, tests, security hardening ([b5b0a40](https://github.com/AlcanDev/korva/commit/b5b0a40e91b8b2e519c80b3f03efcaf032a191e7))
* **behavior:** adopt Karpathy-style behavioral guidelines across all IDEs ([0b568cb](https://github.com/AlcanDev/korva/commit/0b568cbe30b24cf90a5908471dc3277636a57ea9))
* **ci:** automated release pipeline + korva update self-install ([3177413](https://github.com/AlcanDev/korva/commit/3177413af93fdbfe898f39f77b34d3b7ce20a35b))
* **cli:** add cmd/korva entry point ([6badb47](https://github.com/AlcanDev/korva/commit/6badb47908de944ea0a46eeaf6b752d31d88a038))
* **cli:** korva setup command ([1faf15d](https://github.com/AlcanDev/korva/commit/1faf15deed2ad45186b871daf645a340e6350792))
* **deploy:** Dockerfile + docker-compose.yml ([1faf15d](https://github.com/AlcanDev/korva/commit/1faf15deed2ad45186b871daf645a340e6350792))
* **deploy:** production-ready Docker build for Coolify ([ed5d4c9](https://github.com/AlcanDev/korva/commit/ed5d4c9948d0eaeb035b24a7fbd05e4f1a0d8680))
* enhance CI configuration with concurrency, improved testing, and add golangci-lint setup ([f96f54b](https://github.com/AlcanDev/korva/commit/f96f54beb7be22df85c3a14d533094f3aad2dc60))
* **enterprise:** Business tier, license-gated MCP tools, BEHAVIOR.md integrations ([9428286](https://github.com/AlcanDev/korva/commit/942828666459b8c0ab119c37b2b6bd1d60acb506))
* **hooks:** post-commit auto-sync ([1faf15d](https://github.com/AlcanDev/korva/commit/1faf15deed2ad45186b871daf645a340e6350792))
* Korva Hive + Teams + vault operational improvements ([5b1546b](https://github.com/AlcanDev/korva/commit/5b1546bdd2bf291996b37720741e1caa74b7089e))
* korva setup + server deployment + admin panel + auto-sync ([1faf15d](https://github.com/AlcanDev/korva/commit/1faf15deed2ad45186b871daf645a340e6350792))
* **license:** activate + deactivate endpoints — vault API + Beacon UI ([528c555](https://github.com/AlcanDev/korva/commit/528c5550477838cf82d682695c95acded64bf5c2))
* **teams:** complete Teams admin flow — RBAC, team-scoped scrolls, MCP session context ([5f0313a](https://github.com/AlcanDev/korva/commit/5f0313a63f85ec208c7fa3d707c248922ee5c770))
* update testing guidelines for Jest and NestJS, enhance TypeScript practices, and introduce token efficiency principles ([bc2cd1e](https://github.com/AlcanDev/korva/commit/bc2cd1ecb7b1683325b3be8af05fe2e8b07829b7))
* **v1.0:** license activate/deactivate + Beacon UI v1.0 ([f8de4ea](https://github.com/AlcanDev/korva/commit/f8de4ea26f37b0e005af112d8d4bd57813c409b6))
* **v1.0:** quality gates, hybrid context, security hardening ([98b215c](https://github.com/AlcanDev/korva/commit/98b215ccb4c5408a37d02d440e3fcb4be5f22c09))
* **v1.1:** smart skill auto-loader + caveman compression + multi-IDE manifests ([ba8e517](https://github.com/AlcanDev/korva/commit/ba8e5174808ef847ec582cd2cd05a0d4e550bb98))
* **v1.1:** store improvements, MCP profiles, hive sync, IDE integrations ([f1e8894](https://github.com/AlcanDev/korva/commit/f1e8894caaf6621702d60724f9e2d5d5dadde789))
* **vault:** /api/v1/sessions/all endpoint for admin panel ([1faf15d](https://github.com/AlcanDev/korva/commit/1faf15deed2ad45186b871daf645a340e6350792))


### 🐛 Bug Fixes

* **ci:** regenerate package-lock.json using public npm registry ([19bcbba](https://github.com/AlcanDev/korva/commit/19bcbba45266eecc0fa850c8d3fd055dd69f4fec))
* **ci:** remove manual release-patch/minor/major targets from Makefile ([d49ab10](https://github.com/AlcanDev/korva/commit/d49ab10bf3f54cea1c1bd20183229408407331ed))
* **ci:** resolve all CI failures (npm E401, gitleaks false positives) ([751885e](https://github.com/AlcanDev/korva/commit/751885ead7ba95e47737df95576b97f85d57d8aa))
* **install:** correct archive naming in scripts/ installers ([d708d97](https://github.com/AlcanDev/korva/commit/d708d9760279edb723c823384e493f0c2bf485dc))
* **lint:** gofmt alignment in integrations_test manifest map ([dda6b47](https://github.com/AlcanDev/korva/commit/dda6b47aeb303da96f4ddd3a83196c494dc27271))
* **mcp+beacon:** auto-load session token + pass team_id on scroll save ([d7ac6e3](https://github.com/AlcanDev/korva/commit/d7ac6e38909e0053c305cb674fce28ae6ac880d2))
