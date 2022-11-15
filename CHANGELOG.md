<!--
Guiding Principles:

Changelogs are for humans, not machines.
There should be an entry for every single version.
The same types of changes should be grouped.
Versions and sections should be linkable.
The latest version comes first.
The release date of each version is displayed.
Mention whether you follow Semantic Versioning.

Usage:

Change log entries are to be added to the Unreleased section under the
appropriate stanza (see below). Each entry should ideally include a tag and
the Github issue reference in the following format:

* (<tag>) \#<issue-number> message

The issue numbers will later be link-ified during the release process so you do
not have to worry about including a link manually, but you can if you wish.

Types of changes (Stanzas):

"Features" for new features.
"Improvements" for changes in existing functionality.
"Deprecated" for soon-to-be removed features.
"Bug Fixes" for any bug fixes.
"Client Breaking" for breaking CLI commands and REST routes used by end-users.
"API Breaking" for breaking exported APIs used by developers building on SDK.
"State Machine Breaking" for any changes that result in a different AppState
given same genesisState and txList.
Ref: https://keepachangelog.com/en/1.0.0/
-->

# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Unreleased

### Features

### Bug fixes

- [#94](https://github.com/envadiv/Passage3D/pull/94) Fix localnet commands in `Makefile`

### Misc Improvements

## [v1.1.0](https://github.com/envadiv/Passage3D/releases/tag/v1.1.0) - 2022-10-17

- Update cosmos-sdk to v0.45.9 and tendermint v0.34.21 which includes the patch for IBC issue
- fix lint by @anilCSE in https://github.com/envadiv/Passage3D/pull/79

**Full Changelog**: https://github.com/envadiv/Passage3D/compare/v1.0.0...v1.1.0

## [v1.0.0](https://github.com/envadiv/Passage3D/releases/tag/v1.0.0) - 2022-08-08

Initial Release!
