# Changelog

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
