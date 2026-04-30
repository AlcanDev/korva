# Changelog

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
