# Changelog

## [1.8.1](https://github.com/viicslen/gitura/compare/v1.8.0...v1.8.1) (2026-11-18)


### Bug Fixes

* **ui:** keep long code in a comment from widening the comment box ([0cafc4a](https://github.com/viicslen/gitura/commit/0cafc4a659ac44a906a24da850943b05c310521a))
* **ui:** move comment edit and delete to the author line so the run button can't cover them ([#82](https://github.com/viicslen/gitura/issues/82)) ([8764a87](https://github.com/viicslen/gitura/commit/8764a8726159b117f0797c61b0ef57734ceae62b))
* **ui:** put edit and delete on the comment's author line in threads ([22f2072](https://github.com/viicslen/gitura/commit/22f207231802cd605c575e62f79a88fac5a70d39))
* **ui:** stop long code in a comment from pushing its text off-screen ([#80](https://github.com/viicslen/gitura/issues/80)) ([29df5d0](https://github.com/viicslen/gitura/commit/29df5d0d2d56912dbc325f60c10dbffc064b82b3))
* **ui:** stop the run button covering edit and delete on short comments ([eee7e4d](https://github.com/viicslen/gitura/commit/eee7e4d351b9c9cd7cdace074cd2b98a171319e8))

## [1.8.0](https://github.com/viicslen/gitura/compare/v1.7.0...v1.8.0) (2026-11-18)


### Features

* **api:** look up the base and head branch of each open PR ([27defb3](https://github.com/viicslen/gitura/commit/27defb364e4437df2a982053cba9cb16ba438d91))
* **ui:** copy a PR's base and head branch from the Overview ([492b7f7](https://github.com/viicslen/gitura/commit/492b7f7a3fd70754f4161e0fc718e232bef88964))
* **ui:** copy a PR's base and head branch straight from the Overview ([#77](https://github.com/viicslen/gitura/issues/77)) ([1aff414](https://github.com/viicslen/gitura/commit/1aff414f16f6b909efeb09b40ec1d7bb8a290855))
* **ui:** match the List / Tree switch to the toolbar button height ([9625e17](https://github.com/viicslen/gitura/commit/9625e17892bff23fc75a5ec95292b60578006330))
* **ui:** put a PR's labels beside its branches in the Overview ([3e3acba](https://github.com/viicslen/gitura/commit/3e3acbaf912b643654334b05ad65c47dfd5b4599))
* **ui:** put the List / Tree switch at the end of the PR list toolbar ([073e840](https://github.com/viicslen/gitura/commit/073e840a30322106fc822bd154080e3f1df99c77))
* **ui:** see stacked PRs nested under their base PR in the PR list ([#79](https://github.com/viicslen/gitura/issues/79)) ([6df1d4a](https://github.com/viicslen/gitura/commit/6df1d4aa500642ae1fc40832d172c2b2a83a2dd9))
* **ui:** show stacked PRs as a tree in the PR list ([d06e10a](https://github.com/viicslen/gitura/commit/d06e10a286490f55a3d9973402d3cfe8f53bb8e3))


### Bug Fixes

* **ui:** save toggle changes in order ([7afef98](https://github.com/viicslen/gitura/commit/7afef9809162fb7b5b9cb99d8008019b20344bf5))

## [1.7.0](https://github.com/viicslen/gitura/compare/v1.6.0...v1.7.0) (2026-11-18)


### Features

* **ui:** add a Timeline view that interleaves commits, reviews and comments ([933dae0](https://github.com/viicslen/gitura/commit/933dae0c7d0afc89361d9e5bec4573140036a836))
* **ui:** call the Overview's sectioned layout Details ([8a8a20b](https://github.com/viicslen/gitura/commit/8a8a20b9f9f9fece89cc57ce1c97c56f59b0dc5e))
* **ui:** run commands and watch run history from every PR view ([a64964a](https://github.com/viicslen/gitura/commit/a64964a4b7ddade86d02e3abed771ea24a79503e))
* **ui:** see what was reviewed, replied or resolved after each commit in an Overview timeline ([#75](https://github.com/viicslen/gitura/issues/75)) ([7d6698a](https://github.com/viicslen/gitura/commit/7d6698ada0287aaf3bccbf470788b47642eb04fb))
* **ui:** set the run checkout and watch run history from Overview and Files too ([#74](https://github.com/viicslen/gitura/issues/74)) ([4f3783f](https://github.com/viicslen/gitura/commit/4f3783fbdce5de2e2fab690392904572b62ae8ff))
* **ui:** show the timeline as an Overview layout instead of its own view ([1ea0354](https://github.com/viicslen/gitura/commit/1ea035426e40f0ffc6e5af1d9f72b1825e68b80e))

## [1.6.0](https://github.com/viicslen/gitura/compare/v1.5.1...v1.6.0) (2026-11-17)


### Features

* **search:** search any pull request on GitHub, with its open, closed or merged state ([ee6ae96](https://github.com/viicslen/gitura/commit/ee6ae96faf3e50d8fe9f452101d0b05dc0fa2093))
* **ui:** open any pull request from Ctrl+K by searching or typing its number ([1bea8a6](https://github.com/viicslen/gitura/commit/1bea8a6d95710d5be3c6e041a92552a35bfba418))
* **ui:** open any pull request from Ctrl+K by searching or typing its number ([#72](https://github.com/viicslen/gitura/issues/72)) ([918a0ab](https://github.com/viicslen/gitura/commit/918a0ab6747676b4dfc729d45df4fd457129362d))

## [1.5.1](https://github.com/viicslen/gitura/compare/v1.5.0...v1.5.1) (2026-11-16)


### Bug Fixes

* **ui:** stop glows staying lit when Effects is switched off ([#69](https://github.com/viicslen/gitura/issues/69)) ([f87cfd2](https://github.com/viicslen/gitura/commit/f87cfd2f99f3c3b3d3cde706b0477b2576331f33))
* **ui:** stop the rail and viewed-bar glows staying lit with Effects off ([6c69d9e](https://github.com/viicslen/gitura/commit/6c69d9ea1c9b67c497b437521c535e90711a35fd))


### Performance Improvements

* **diff:** cut instead of glide when many files change viewed state at once ([ace9077](https://github.com/viicslen/gitura/commit/ace9077ff220c885e2703551eb9a80da06e19621))
* **ui:** keep dialogs, the run panel and bulk file marking smooth by moving their motion off layout and full-window blur ([#71](https://github.com/viicslen/gitura/issues/71)) ([3ceb3c5](https://github.com/viicslen/gitura/commit/3ceb3c536eb7d296ccc06527d31a752e4438f785))
* **ui:** open the run panel without re-laying out the diff every frame ([9a5142a](https://github.com/viicslen/gitura/commit/9a5142a01e6d809d1da557292517cbcccb4aa5b5))
* **ui:** repaint the busy glow 20 times a second and cascade only 15 cards ([17ddc1a](https://github.com/viicslen/gitura/commit/17ddc1a23e7f7b9e209b45a0aee2dc39a961a33b))
* **ui:** stop blurring the whole window behind every dialog ([dbdb7cf](https://github.com/viicslen/gitura/commit/dbdb7cfac93bdb4deef8ba53db6b7bdd0077ff83))
* **ui:** stop the pointer tilt forcing a layout on every mouse move ([b6e91cc](https://github.com/viicslen/gitura/commit/b6e91cc1a070e2263c10babdc400f21b78778e93))

## [1.5.0](https://github.com/viicslen/gitura/compare/v1.4.0...v1.5.0) (2026-11-16)


### Features

* **ui:** add an Effects switch, off by default, and drop the cursor glow and aurora ([32fed75](https://github.com/viicslen/gitura/commit/32fed75d26c0f1d3d50fd0648998934c2753c732))
* **ui:** add spring, light and celebration effects in the logo's emerald ([4cf7699](https://github.com/viicslen/gitura/commit/4cf7699eff0854400822eaaf68161538baefcdba))
* **ui:** cascade, light up and roll the counts of the PR list ([20265ae](https://github.com/viicslen/gitura/commit/20265aeee1d6de5cdbf6c8ecef3ff04f77ebfa62))
* **ui:** celebrate resolved threads and merges, and deal threads like cards ([325f6b7](https://github.com/viicslen/gitura/commit/325f6b7a6d2c1b21a34e4247f410a7ba4fa2f3f9))
* **ui:** fly the PR title into the title strip and reveal the app from the sign-in card ([3c71e05](https://github.com/viicslen/gitura/commit/3c71e05c2c3504159201dc08d99dda77f46f8d5c))
* **ui:** make every screen land with spring, light and celebration in the logo's emerald ([#68](https://github.com/viicslen/gitura/issues/68)) ([0f9b6f1](https://github.com/viicslen/gitura/commit/0f9b6f126e7872d20da8726ed72396ff680d9e3f))
* **ui:** slide files in tree order, light up viewed files and the run panel's result ([4662919](https://github.com/viicslen/gitura/commit/4662919d6c402175d5d5b58540fae8975a64b2df))
* **ui:** spring every dialog, menu and toast open and reveal a new theme from where it was picked ([e0d8a82](https://github.com/viicslen/gitura/commit/e0d8a820b32f77db086aefd377d2d637ddabc7ee))


### Bug Fixes

* **ui:** keep wide tables inside their comment card instead of stretching it off-screen ([09b93cd](https://github.com/viicslen/gitura/commit/09b93cddb106243673fb9d82681f185da26044fd))
* **ui:** keep wide tables inside their comment card instead of stretching it off-screen ([#65](https://github.com/viicslen/gitura/issues/65)) ([7c08d90](https://github.com/viicslen/gitura/commit/7c08d904bfea28e25a43ebf7df3075a19384fb0e))
* **ui:** stop the app freezing on view transitions on NVIDIA under Linux ([1ab8ff0](https://github.com/viicslen/gitura/commit/1ab8ff013f16f38f6decae9381dec4a1dbf4f99f))
* **ui:** stop the PR filter sidebar showing a scrollbar when nothing overflows ([c08b8e4](https://github.com/viicslen/gitura/commit/c08b8e4949a224dc508e989d6b19b2c40b7e7d1d))
* **ui:** stop the PR filter sidebar showing a scrollbar when nothing overflows ([#67](https://github.com/viicslen/gitura/issues/67)) ([483c4e4](https://github.com/viicslen/gitura/commit/483c4e4e076505ca41dd03586dc217f984696fc1))

## [1.4.0](https://github.com/viicslen/gitura/compare/v1.3.0...v1.4.0) (2026-11-13)


### Features

* **ui:** finish the motion pass so thread, file, list and page changes glide instead of popping ([#64](https://github.com/viicslen/gitura/issues/64)) ([94d3676](https://github.com/viicslen/gitura/commit/94d3676a3926a8cd42638f8d8d76664d84301a56))
* **ui:** flow between states instead of cutting, and bring back the dead dialog and menu animations ([#61](https://github.com/viicslen/gitura/issues/61)) ([eecaf34](https://github.com/viicslen/gitura/commit/eecaf34c71dd049f2e33a4206d8d6f7f8da1fe70))
* **ui:** flow page, list, panel and inline changes into their next state ([259b3e0](https://github.com/viicslen/gitura/commit/259b3e0df5f759d9fb449158bcb9cf92843cdb62))
* **ui:** glide, crossfade or settle every remaining state change ([1b31cb2](https://github.com/viicslen/gitura/commit/1b31cb2fa0e48e9dc9583bee89d6843a90ba1107))


### Bug Fixes

* **dev:** bind the Vite dev server to IPv4 so the app window is not blank ([d8bcdce](https://github.com/viicslen/gitura/commit/d8bcdceff1593afa74762d1c4634d83c6d13eaa3))
* **dev:** stop `just dev` opening a blank window ([#63](https://github.com/viicslen/gitura/issues/63)) ([37c7980](https://github.com/viicslen/gitura/commit/37c7980e881681004547293ed0524c635ddb1575))
* **ui:** bring back dialog and menu animations and add motion tokens ([ec059e9](https://github.com/viicslen/gitura/commit/ec059e93e87dced6cfff81b98e822f5ec9b07ee2))

## [1.3.0](https://github.com/viicslen/gitura/compare/v1.2.0...v1.3.0) (2026-11-13)


### Features

* **linux:** render the webview on the GPU ([6e3900d](https://github.com/viicslen/gitura/commit/6e3900d030e4a2e0275e2c1f7514a1870e32c717))


### Performance Improvements

* **linux:** render the webview on the GPU now that Wails 3 handles NVIDIA ([#59](https://github.com/viicslen/gitura/issues/59)) ([4579202](https://github.com/viicslen/gitura/commit/4579202aac59b9b8c26bf7df7649dc333fbac162))

## [1.2.0](https://github.com/viicslen/gitura/compare/v1.1.0...v1.2.0) (2026-11-13)


### Features

* **appearance:** save the theme in settings.toml and show it as icons in the account menu ([4aef992](https://github.com/viicslen/gitura/commit/4aef992e63af49162a0f052a99b5d25c26faa253))
* **launch:** add --focus to open a review in its own restricted window ([e2ca296](https://github.com/viicslen/gitura/commit/e2ca296f5d75c008926dc419eaa2b128fee53e65))
* **launch:** review a single PR or local changes from the CLI without the rest of the app ([#56](https://github.com/viicslen/gitura/issues/56)) ([e0b07e5](https://github.com/viicslen/gitura/commit/e0b07e54bab342d6aed548742a6d9a039a8c17c1))
* **settings:** keep diff, review and PR list preferences in settings ([dbc15a9](https://github.com/viicslen/gitura/commit/dbc15a992de5ca54b95aa085d229c5dd07c7c9fe))
* **settings:** keep theme, diff and PR list preferences in Settings, and optionally leave a review after merging ([#58](https://github.com/viicslen/gitura/issues/58)) ([0b401a6](https://github.com/viicslen/gitura/commit/0b401a68c0cde69d5af573a1ec48d37d3b6b8c50))

## [1.1.0](https://github.com/viicslen/gitura/compare/v1.0.1...v1.1.0) (2026-11-13)


### Features

* **launch:** open the branch's PR from --local once everything is pushed ([8799bfe](https://github.com/viicslen/gitura/commit/8799bfe7610a45ee14e348c45b1ece644b2a147c))
* **launch:** open the branch's PR from --local when nothing is left unpushed ([#55](https://github.com/viicslen/gitura/issues/55)) ([8799bfe](https://github.com/viicslen/gitura/commit/8799bfe7610a45ee14e348c45b1ece644b2a147c))


### Bug Fixes

* **ui:** put the Refresh button next to the PR title ([ff0d8e1](https://github.com/viicslen/gitura/commit/ff0d8e1c91d5980702c19c7b8510de82f6f66b45))
* **ui:** put the Refresh button next to the PR title it reloads ([#53](https://github.com/viicslen/gitura/issues/53)) ([ff0d8e1](https://github.com/viicslen/gitura/commit/ff0d8e1c91d5980702c19c7b8510de82f6f66b45))

## [1.0.1](https://github.com/viicslen/gitura/compare/v1.0.0...v1.0.1) (2026-11-12)


### Bug Fixes

* **nix:** build with Go 1.27 and GTK4/WebKitGTK 6.0 ([ffd5e48](https://github.com/viicslen/gitura/commit/ffd5e48fd3e8ba5575f42ae52aef6f0eb40c5855))

## [1.0.0](https://github.com/viicslen/gitura/compare/v0.15.0...v1.0.0) (2026-11-12)


### ⚠ BREAKING CHANGES

* move to Wails v3 and put every dependency on its latest stable release ([#50](https://github.com/viicslen/gitura/issues/50))

### Features

* move to Wails v3 and put every dependency on its latest stable release ([#50](https://github.com/viicslen/gitura/issues/50)) ([b9c2cc9](https://github.com/viicslen/gitura/commit/b9c2cc98763e6e2537814928b77fae3101f90743))

## [0.15.0](https://github.com/viicslen/gitura/compare/v0.14.0...v0.15.0) (2026-11-12)


### Features

* **diff:** mark a folder's files viewed from its context menu, and auto-mark hidden tests ([5093834](https://github.com/viicslen/gitura/commit/509383429db69527d0bc2f6b58637f766c6fa346))
* **diff:** mark a whole folder viewed from its right-click menu, and auto-mark hidden tests ([#48](https://github.com/viicslen/gitura/issues/48)) ([3cf3cb5](https://github.com/viicslen/gitura/commit/3cf3cb59421b01412041924082dd11e9e44748ce))
* **title-strip:** make Merge stand out and keep the GitHub link by the PR state ([aacbf99](https://github.com/viicslen/gitura/commit/aacbf997fdc5fa2f997a5a5438ae1591857b7a46))
* **title-strip:** make Merge stand out and keep the GitHub link by the PR state ([#46](https://github.com/viicslen/gitura/issues/46)) ([f527fdf](https://github.com/viicslen/gitura/commit/f527fdfce13e6bb9b0b765a5f4506404c2e160d0))


### Bug Fixes

* **comments:** keep diff line colours across the full width when scrolling sideways ([975422e](https://github.com/viicslen/gitura/commit/975422e9aead6857fd700132defe311d4d55313b))
* **comments:** keep diff line colours across the full width when scrolling sideways ([#45](https://github.com/viicslen/gitura/issues/45)) ([d902711](https://github.com/viicslen/gitura/commit/d90271112df0bf360c3bf795a18fc180484cda82))
* **diff:** stop comment threads from stalling while you scroll them ([248c3df](https://github.com/viicslen/gitura/commit/248c3dfd251975175152a585623da244b6c81cc3))
* **diff:** stop comment threads from stalling while you scroll them ([#49](https://github.com/viicslen/gitura/issues/49)) ([e1c498f](https://github.com/viicslen/gitura/commit/e1c498fb6ddac0d24920ba6dd5e0d5568c89dc7c))


### Performance Improvements

* **diff:** stop collapsing a thread from re-parsing its comments ([76ffae8](https://github.com/viicslen/gitura/commit/76ffae875f46763a7f917b9bd9e81aa847e97b09))
* **diff:** stop typing a comment from re-rendering every diff line ([0ced3fd](https://github.com/viicslen/gitura/commit/0ced3fdfae3f4c62c2d5aca122fd4b8f4507e716))

## [0.14.0](https://github.com/viicslen/gitura/compare/v0.13.5...v0.14.0) (2026-11-11)


### Features

* **cli:** open a PR from the command line, in the window you already have ([#43](https://github.com/viicslen/gitura/issues/43)) ([b710880](https://github.com/viicslen/gitura/commit/b7108805e8570cef6f9440dea0f296bb38761203))
* **cli:** open a PR straight from the command line ([505dae5](https://github.com/viicslen/gitura/commit/505dae5ffe6e9863daee15f467d5e7ce48f48520))
* **cli:** open local changes from the command line ([869b6b9](https://github.com/viicslen/gitura/commit/869b6b9eaa8a6609c86f9ad82ea7fbbbfc629652))
* **cli:** open local changes from the command line, in the window you already have ([#44](https://github.com/viicslen/gitura/issues/44)) ([6af61ae](https://github.com/viicslen/gitura/commit/6af61ae0514b682763513830a5acdc1f9f33e3c6))


### Bug Fixes

* **pr-list:** drop merged PRs from the list when leaving the PR view ([#41](https://github.com/viicslen/gitura/issues/41)) ([bca408b](https://github.com/viicslen/gitura/commit/bca408b0dce1c0aea47b64271ef74d82ac0e3295))
* **pr-list:** reflect merges and edits from the PR view in the list ([59286e6](https://github.com/viicslen/gitura/commit/59286e60ca4460327d753b5b5e39b2dd56d7219a))

## [0.13.5](https://github.com/viicslen/gitura/compare/v0.13.4...v0.13.5) (2026-11-10)


### Bug Fixes

* **nix:** stop gitura opening to a blank window from app launchers ([#39](https://github.com/viicslen/gitura/issues/39)) ([b77f9d0](https://github.com/viicslen/gitura/commit/b77f9d064715961ca913ae516d1960a67d5103c2))
* **nix:** wrap gitura so WebKitGTK finds GSettings schemas ([0b1580f](https://github.com/viicslen/gitura/commit/0b1580f6bceb89806f634c887903632fd353fcb0))

## [0.13.4](https://github.com/viicslen/gitura/compare/v0.13.3...v0.13.4) (2026-11-10)


### Bug Fixes

* **nix:** show gitura in app launchers ([#37](https://github.com/viicslen/gitura/issues/37)) ([1d9f70f](https://github.com/viicslen/gitura/commit/1d9f70fbc7643bede8bb7c968e2b9c1b26c0c4f9))

## [0.13.3](https://github.com/viicslen/gitura/compare/v0.13.2...v0.13.3) (2026-11-10)


### Bug Fixes

* **deps:** bump echo to v4.16.3 for Dependabot advisories ([9a8372e](https://github.com/viicslen/gitura/commit/9a8372e0adfaa71ecd3ffdf271263b254c78e147))
* **diff:** strip comment-preview tags until stable ([21e296f](https://github.com/viicslen/gitura/commit/21e296f08472aba187de6698ff35605718d016b1))
* **diff:** strip stray angle brackets from inline thread previews ([0e7a477](https://github.com/viicslen/gitura/commit/0e7a4777f029e39f03c85029e8189070330e0bb2))

### License

* relicense under AGPL-3.0; releases up to v0.13.2 remain MIT ([148e6ba](https://github.com/viicslen/gitura/commit/148e6ba68a7ba89f62a3b3a38befbd4fe8ab67e5))

## [0.13.2](https://github.com/viicslen/gitura/compare/v0.13.1...v0.13.2) (2026-11-10)


### Bug Fixes

* **app:** re-check asset proxy redirects against the host allowlist ([57f71ea](https://github.com/viicslen/gitura/commit/57f71ea2cb1b68ac66c17ab0a53b07f0bfd119a7))

## [0.13.1](https://github.com/viicslen/gitura/compare/v0.13.0...v0.13.1) (2026-11-10)


### Bug Fixes

* **nix:** use stdenv.hostPlatform.isLinux ([af1af95](https://github.com/viicslen/gitura/commit/af1af9555d3b14b134ee76471c69265e6eefaabc))

## [0.13.0](https://github.com/viicslen/gitura/compare/v0.12.1...v0.13.0) (2026-11-09)


### Features

* **frontend:** merge, squash, rebase or close a PR from the title strip ([07e2548](https://github.com/viicslen/gitura/commit/07e25482e8ae345faa7077c23f8e73224901c591))
* **frontend:** zoom markdown images in a lightbox on click ([24176c2](https://github.com/viicslen/gitura/commit/24176c204017703f2398ffd68c4e2c7f6ab0c00a))
* **github:** merge and close the loaded pull request ([73a90aa](https://github.com/viicslen/gitura/commit/73a90aae473831fa97db9f35338ff744b5a5ee35))
* **review:** merge, squash, rebase or close a PR without leaving gitura ([#31](https://github.com/viicslen/gitura/issues/31)) ([f3e4cda](https://github.com/viicslen/gitura/commit/f3e4cda25da66e7f9e22a7a8be147b503af714d6))

## [0.12.1](https://github.com/viicslen/gitura/compare/v0.12.0...v0.12.1) (2026-11-09)


### Bug Fixes

* **nix:** add git to nativeCheckInputs ([d98befb](https://github.com/viicslen/gitura/commit/d98befbb000d0efeed7bd23b6ccfde75b32f8d6e))

## [0.12.0](https://github.com/viicslen/gitura/compare/v0.11.0...v0.12.0) (2026-11-09)


### Features

* **frontend:** review local changes and send comments to a command ([e040de5](https://github.com/viicslen/gitura/commit/e040de55207be276599229bc829db19d0d614e6d))
* **local-review:** review local changes before they're a PR and send comments to your agent ([#28](https://github.com/viicslen/gitura/issues/28)) ([fd83267](https://github.com/viicslen/gitura/commit/fd83267a4f9ca220a74e33e677c9bacc0b1abd98))
* **local-review:** serve local git changes through the PR file and draft APIs ([85bad9a](https://github.com/viicslen/gitura/commit/85bad9ab864f544788c7faeebaf0246e8ab99d9b))

## [0.11.0](https://github.com/viicslen/gitura/compare/v0.10.0...v0.11.0) (2026-11-09)


### Features

* **frontend:** improve PR list page and exact org/repo filtering ([1211816](https://github.com/viicslen/gitura/commit/12118160ffd8d26376e781a28a33373c8ba71dd1))
* **pr-list:** exact org/repo filtering and a denser, scannable PR list ([#26](https://github.com/viicslen/gitura/issues/26)) ([7e451e8](https://github.com/viicslen/gitura/commit/7e451e8f445c2b30352b7a857f2dc22dd3ec2cee))

## [0.10.0](https://github.com/viicslen/gitura/compare/v0.9.1...v0.10.0) (2026-11-09)


### Features

* **settings:** allow choosing installed system fonts ([b4dd028](https://github.com/viicslen/gitura/commit/b4dd0282666ef45f10c686f9f315589c00389c64))

## [0.9.1](https://github.com/viicslen/gitura/compare/v0.9.0...v0.9.1) (2026-11-09)


### Bug Fixes

* **markdown:** proxy private repo images through the GitHub API ([2833bc5](https://github.com/viicslen/gitura/commit/2833bc56dc9fa7120bb1d8c2988126b0ca8da5cd))

## [0.9.0](https://github.com/viicslen/gitura/compare/v0.8.0...v0.9.0) (2026-11-09)


### Features

* **settings:** sidebar layout with card rows in a smaller dialog ([f42f3fb](https://github.com/viicslen/gitura/commit/f42f3fb8131266a59245b2ff79ae1951b7e6fded))

## [0.8.0](https://github.com/viicslen/gitura/compare/v0.7.0...v0.8.0) (2026-11-09)


### Features

* **files:** leave left/right arrows to the page in the file tree ([76df86a](https://github.com/viicslen/gitura/commit/76df86a42feff287623503644251a24c5295f09b))
* **settings:** rebind file and thread navigation keys ([c91db38](https://github.com/viicslen/gitura/commit/c91db386fa05808b6e8766cede77f6e7854f8dda))
* **settings:** rebind the keys that move between files and threads ([#22](https://github.com/viicslen/gitura/issues/22)) ([bf1b133](https://github.com/viicslen/gitura/commit/bf1b133dbd71d81cec2c3bf09056690b6eabbc1f))
* **ui:** back arrow in the title strip while reviewing a PR ([eb21d07](https://github.com/viicslen/gitura/commit/eb21d072a0cb713ed09de69d26267b4ca11643eb))
* **ui:** count badges on the review view buttons in the side rail ([a7089bc](https://github.com/viicslen/gitura/commit/a7089bcd2b44f4a3f7e9d10af95e257d11748d14))
* **ui:** edit and delete pending draft comments in place ([ba09af3](https://github.com/viicslen/gitura/commit/ba09af38726fcd3af57a9e5a60d449abc25f46ef))
* **ui:** readable default fonts and an Appearance setting ([56eb6e6](https://github.com/viicslen/gitura/commit/56eb6e6cd13333f41535cb86689051fac9db2bc2))

## [0.7.0](https://github.com/viicslen/gitura/compare/v0.6.1...v0.7.0) (2026-11-09)


### Features

* **api:** PR labels, viewed files, guarded body edits and stable review numbers ([efe17de](https://github.com/viicslen/gitura/commit/efe17dec589ba8e398d1f70054111a9249c6c712))
* **comments:** GitHub-style markdown, clickable task lists, labels and tables ([4f24c2c](https://github.com/viicslen/gitura/commit/4f24c2c4f33200acffb2a2f83137ffe4b543e9df))
* **files:** viewed files, filters, line wrap, Vue highlighting and safer drafts ([5ffcce7](https://github.com/viicslen/gitura/commit/5ffcce7ba44b9bd88e9d6c823334b96bac35270e))
* give the app its own icon and show it in the side rail ([408672a](https://github.com/viicslen/gitura/commit/408672a4119b8c9f239e54cf2a7ef1f849ab920b))
* **ui:** edit and delete your own comments in place ([1724908](https://github.com/viicslen/gitura/commit/1724908f189e2b9c31adb63311070717574f4d61))
* **ui:** frameless window with a side rail and an in-app title strip ([d2926f7](https://github.com/viicslen/gitura/commit/d2926f72a0bf3eda3aaa3efe78f4cb339ee0d17b))
* **ui:** review a PR end to end from a side rail, with viewed files and GitHub-style comments ([#19](https://github.com/viicslen/gitura/issues/19)) ([d1134e6](https://github.com/viicslen/gitura/commit/d1134e6414723eb33e32bee8c51b2419e9e5e30a))


### Bug Fixes

* **ui:** let the window grow past the monitor it opened on ([1b1e629](https://github.com/viicslen/gitura/commit/1b1e6291ee9061d0755d05d82916a9581f050057))

## [0.6.1](https://github.com/viicslen/gitura/compare/v0.6.0...v0.6.1) (2026-11-09)


### Bug Fixes

* **nix:** pass desktop and production build tags required by Wails ([e8dfbe4](https://github.com/viicslen/gitura/commit/e8dfbe41c76a8bd6d7959db107ccda36e6022498))

## [0.6.0](https://github.com/viicslen/gitura/compare/v0.5.1...v0.6.0) (2026-11-06)


### Features

* command output streaming ([dddbf6d](https://github.com/viicslen/gitura/commit/dddbf6d688ce0c74fde03b87d53d06adc30947f6))
* **review:** add a PR overview tab with reviews and general comments ([f9de0eb](https://github.com/viicslen/gitura/commit/f9de0eba09401999dea6bdec097c6f64cbb0b83b))
* **review:** JetBrains-style side-by-side diff with inline threads and reviewing ([cfdc876](https://github.com/viicslen/gitura/commit/cfdc8764cd5676f212b77a206e2eef5051ff08cb))
* **review:** review PRs in a JetBrains-style split diff with threads inline ([#16](https://github.com/viicslen/gitura/issues/16)) ([801039e](https://github.com/viicslen/gitura/commit/801039e5f46664aa8fc7b6c4fd003829bc77d331))
* **review:** scroll split diff panes in sync through change blocks ([35b5f43](https://github.com/viicslen/gitura/commit/35b5f435a4097ce0213560e281101b807d875f49))
* **review:** show changed files as a folder tree in a resizable sidebar ([70feea2](https://github.com/viicslen/gitura/commit/70feea2996467f958cba05b5857ab9a463c6cbce))


### Bug Fixes

* handle avatars correctly ([4603bfd](https://github.com/viicslen/gitura/commit/4603bfd445711c6469179b77a3a8e94c54fa3751))
* **nix:** build the dev shell with the current Go toolchain ([f9a1d06](https://github.com/viicslen/gitura/commit/f9a1d06a66bfcb0762775841e3c0b646a2a75a9d))
* **nix:** update stale flake hashes and compute them in dependency order ([6756ecd](https://github.com/viicslen/gitura/commit/6756ecd9904562423467d78920a03fc3f161eddf))
* **pr-list:** show a retryable error when GitHub search is unavailable ([8ffd31e](https://github.com/viicslen/gitura/commit/8ffd31e88c9ed520c699843e7f239a6c6c64e4c0))
* **review:** stop the shorter diff pane bouncing while scrolling ([20a71cf](https://github.com/viicslen/gitura/commit/20a71cf89c1936e875a04e54d01567faa1666a49))

## [0.5.1](https://github.com/viicslen/gitura/compare/v0.5.0...v0.5.1) (2026-09-29)


### Bug Fixes

* **nix:** make the frontend build work in the sandbox ([501ca0e](https://github.com/viicslen/gitura/commit/501ca0e1aeaa69f6b25ecc6a7ba1d62cfef2feff))
* **runner:** put the gosec suppression in a form gosec reads ([4d547a4](https://github.com/viicslen/gitura/commit/4d547a48d4d93c61a37650b8f1d6b0468354f967))
* **runner:** substitute command placeholders per argument ([6dd4a04](https://github.com/viicslen/gitura/commit/6dd4a041ecf1f7f4904d5ffe4fb640ef5db2bbdd))

## [0.5.0](https://github.com/viicslen/gitura/compare/v0.4.0...v0.5.0) (2026-04-14)


### Features

* add refresh button to pr page ([90fd59f](https://github.com/viicslen/gitura/commit/90fd59f6a5ec8a2f0fcf0330a6c3d7063b38085a))

## [0.4.0](https://github.com/viicslen/gitura/compare/v0.3.0...v0.4.0) (2026-04-07)


### Features

* add option to run commands based on the PR comments and code blocks ([6bd93d6](https://github.com/viicslen/gitura/commit/6bd93d6eb71999962f7e0ccbbf42bcdf8f83b87c))
* add per pr local path ([f70d25b](https://github.com/viicslen/gitura/commit/f70d25b6c280bda080511e661472ef31e6388481))
* improve command running ([eea62ba](https://github.com/viicslen/gitura/commit/eea62baf256503c0cdc5df85fa2aee73d9a8d611))

## [0.3.0](https://github.com/viicslen/gitura/compare/v0.2.1...v0.3.0) (2026-04-03)


### Features

* add syntax highlighting and single-side view for added/removed files ([115674b](https://github.com/viicslen/gitura/commit/115674bf5c46bbb0a7f62c95731b5e1448308c28))

## [0.2.1](https://github.com/viicslen/gitura/compare/v0.2.0...v0.2.1) (2026-04-03)


### Bug Fixes

* **nix:** move CGO_ENABLED into env to fix nixpkgs conflict ([a8eb452](https://github.com/viicslen/gitura/commit/a8eb452751571317dc3c5958fdff410ef214bb71))

## [0.2.0](https://github.com/viicslen/gitura/compare/v0.1.0...v0.2.0) (2026-04-03)


### Features

* add Nix flake with versioned package definition and release-please integration ([5e90729](https://github.com/viicslen/gitura/commit/5e907293927d9d78c48115dd5e30cbc0e9f5d160))
* add Nix flake with versioned package definition and release-please integration ([#8](https://github.com/viicslen/gitura/issues/8)) ([7d3e739](https://github.com/viicslen/gitura/commit/7d3e73967b09ac57133883f41ab3e5e01c27871a))


### Bug Fixes

* general issues with file review comments ([999e201](https://github.com/viicslen/gitura/commit/999e20172ad2e6c3576dec19a29b6ec06eda78a1))
* remove vendor/ from git and restore placeholder vendorHash ([c0d08dd](https://github.com/viicslen/gitura/commit/c0d08dd940dff3cbf1027a4c4a623f9bbbd22087))
* replace placeholder hashes - vendor Go modules and use importNpmLock for frontend ([f9990e7](https://github.com/viicslen/gitura/commit/f9990e74a0decb02ca0ba46cd36900164faa726a))
* restore GitHub sync for pending review on PR load ([77e7221](https://github.com/viicslen/gitura/commit/77e72211a472e27d983c2b99ab36487f3ab4291f))

## [0.1.0](https://github.com/viicslen/gitura/compare/v0.0.1...v0.1.0) (2026-04-01)


### Features

* changed files view ([05cccdd](https://github.com/viicslen/gitura/commit/05cccddd97b10ee8892d16cb10cf2d1cee22f781))


### Bug Fixes

* resolve 404 and button label bugs in PR review flow ([c58d5cf](https://github.com/viicslen/gitura/commit/c58d5cfd39c618003309635cbf21178d0bdd81dc))
