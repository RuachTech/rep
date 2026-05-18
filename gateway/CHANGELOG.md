# Changelog

## [0.1.7](https://github.com/RuachTech/rep/compare/gateway/v0.1.6...gateway/v0.1.7) (2026-05-18)


### Bug Fixes

* sync gateway/version.txt to 0.1.6 ([#52](https://github.com/RuachTech/rep/issues/52)) ([6dcf724](https://github.com/RuachTech/rep/commit/6dcf72442b3b360c3a46345ed01d51a8a57fd44e))

## [0.1.6](https://github.com/RuachTech/rep/compare/gateway/v0.1.5...gateway/v0.1.6) (2026-04-29)


### Features

* **gateway:** gzip injected HTML and cache when no SENSITIVE vars ([#49](https://github.com/RuachTech/rep/issues/49)) ([83b5e58](https://github.com/RuachTech/rep/commit/83b5e583ac8280431c560a82556ce6d6f58cb69e))

## [0.1.5](https://github.com/RuachTech/rep/compare/gateway/v0.1.4...gateway/v0.1.5) (2026-03-16)


### Bug Fixes

* **gateway:** buffer response headers to prevent superfluous WriteHeader ([#35](https://github.com/RuachTech/rep/issues/35)) ([f82cd95](https://github.com/RuachTech/rep/commit/f82cd955a34c6443f39d02c6a1a7d71f154a214f))
* reduce session key burst pressure ([#37](https://github.com/RuachTech/rep/issues/37)) ([57c911f](https://github.com/RuachTech/rep/commit/57c911f3b32f6f038e1c10a9c53eb71f6773d98c))

## [0.1.4](https://github.com/RuachTech/rep/compare/gateway/v0.1.3...gateway/v0.1.4) (2026-03-15)


### Bug Fixes

* **gateway:** serve pre-rendered directories before SPA fallback ([#32](https://github.com/RuachTech/rep/issues/32)) ([b448534](https://github.com/RuachTech/rep/commit/b4485347d9b73c73e313541e1bcee5b54ab0cb55))

## [0.1.3](https://github.com/RuachTech/rep/compare/gateway/v0.1.2...gateway/v0.1.3) (2026-02-22)


### Bug Fixes

* copy LICENSE file into gateway directory and update GoReleaser config ([12ec03f](https://github.com/RuachTech/rep/commit/12ec03fd0078a68bfe28ed81906eae7c5152e653))
* update changelog source in GoReleaser configuration from github to git ([a9fef2b](https://github.com/RuachTech/rep/commit/a9fef2bf7c73a122a18867fda54deb3d4e5882ed))
* update name_template in GoReleaser configuration to include version ([4a6b3f2](https://github.com/RuachTech/rep/commit/4a6b3f2c771bf9b4cd29db7dfeda21ddcbb47c74))

## [0.1.2](https://github.com/RuachTech/rep/compare/gateway/v0.1.1...gateway/v0.1.2) (2026-02-21)


### Features

* **gateway:** add support for .env file parsing and integration with environment variable classification ([cb220c4](https://github.com/RuachTech/rep/commit/cb220c423d66de4f85332cb571a8b8948d434aaf))


### Bug Fixes

* update release configuration and versioning for consistency ([23d6a32](https://github.com/RuachTech/rep/commit/23d6a323534414aeffc09fbe413f50015cbac992))

## [0.1.1](https://github.com/RuachTech/rep/compare/gateway/v0.1.0...gateway/v0.1.1) (2026-02-21)


### Features

* add GitHub Actions workflows for Docker, Gateway CI, SDK CI, and release processes ([ca2f6f3](https://github.com/RuachTech/rep/commit/ca2f6f38798d2036da8301044110fe5361b6ed89))
* add REP gateway server implementation and example manifest ([164650a](https://github.com/RuachTech/rep/commit/164650ae732aa0a5ed5d1656ad2f68b673b63c5a))
* **crypto:** implement HKDF-based session key derivation and SRI fixes ([1b6fa5c](https://github.com/RuachTech/rep/commit/1b6fa5c305e19a58d4aa2c1d3ba170bdafd702cd))
* **gateway:** initialize gateway service with config classification and unit tests ([b51e75f](https://github.com/RuachTech/rep/commit/b51e75f903b0c0c57cfc35c7d9eea88f406d4026))
* implement release workflow and configuration for npm packages and Go gateway ([04fd197](https://github.com/RuachTech/rep/commit/04fd197d8f2d09d26a82190140a22a497229c6be))
* **inject:** enhance HTML injection to skip &lt;/head&gt; inside comments and add tests ([0860c79](https://github.com/RuachTech/rep/commit/0860c79bf515a8d675a45a3ac038e036667eca81))


### Bug Fixes

* update organization references from 'ruach-tech' to 'ruachtech' across documentation and code ([1c45afc](https://github.com/RuachTech/rep/commit/1c45afcd9d78c206cbf3de752030fb97e75020ef))
