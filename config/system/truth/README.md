# system truth baseline

This directory is the repository-managed baseline seed for Athena system truth.

Current role:

- serves as the default `SYSTEM_TRUTH_DIR` when no environment override is provided
- stores the only human-maintained truth sources under `sources/`
- does not store generated active state or compiled publish packages
- accepts Git-reviewed baseline recovery from validated source truth changes

Generated outputs:

- `output/system-state/`
  - default `SYSTEM_ACTIVE_STATE_DIR`
  - stores control-plane generated active state for each asset
  - contains `meta.json`, `parse_result.json`, `compile_result.json`, `pipeline.json`, `.versions/`, and `.truth-version`
- `output/system-assets/`
  - default `SYSTEM_COMPILED_ASSETS_DIR`
  - stores compiled publish packages from `go run ./scripts/build-system-assets`
  - is rebuilt destructively and must stay separate from active state

Layout:

- `sources/`
  - the only human-edited source layer
  - split into `core/` and `scenes/`

Source layout:

- `sources/core/SOUL.md`
  - global persona baseline
- `sources/core/AGENTS.md`
  - global operational profile
- `sources/core/policy_rule/*.md`
  - cross-scene policy rules
- `sources/core/user_profile/*.md`
  - default user profile views
- `sources/core/memory_view/*.md`
  - default memory views
- `sources/scenes/<scene_id>/SCENE.md`
  - scene definition
- `sources/scenes/<scene_id>/workflow.yaml`
  - workflow definition
- `sources/scenes/<scene_id>/contract/*.yaml`
  - contracts for scene outputs / evidence / artifacts
- `sources/scenes/<scene_id>/policy_rule/*.md`
  - scene-local rules
- `sources/scenes/<scene_id>/skills/<skill_id>/`
  - standard skill package:
    - `SKILL.md`
    - `scripts/`
    - `references/`
    - `assets/`

Naming rules:

- `sources/core/SOUL.md` -> `persona.default`
- `sources/core/AGENTS.md` -> `agent_profile.default`
- `sources/core/policy_rule/<id>.md` -> `policy_rule.core.<id>`
- `sources/core/user_profile/<id>.md` -> `user_profile.<id>`
- `sources/core/memory_view/<id>.md` -> `memory_view.<id>`
- `sources/scenes/<scene_id>/SCENE.md` -> `scene.<scene_id>`
- `sources/scenes/<scene_id>/workflow.yaml` -> `workflow.<scene_id>.main`
- `sources/scenes/<scene_id>/contract/<id>.yaml` -> `contract.<scene_id>.<id>`
- `sources/scenes/<scene_id>/policy_rule/<id>.md` -> `policy_rule.<scene_id>.<id>`
- `sources/scenes/<scene_id>/skills/<skill_id>/` -> `skill.<scene_id>.<skill_id>`

Formal asset types:

- `persona`
- `agent_profile`
- `policy_rule`
- `user_profile`
- `memory_view`
- `scene`
- `workflow`
- `contract`
- `skill`

Guardrails:

- `sources/` is the only human-edited truth layer
- the formal source model is `sources/core/` plus `sources/scenes/<scene_id>/`
- legacy top-level asset directories such as `sources/rule_spec/`, `sources/skill_bundle/`, `sources/skill_summary/`, `sources/user_profile/`, and `sources/memory_view/` are not consumed by sync
- formal source files must not be empty
- generated active state lives under `SYSTEM_ACTIVE_STATE_DIR` and must not be edited manually
- compiled publish packages live under `SYSTEM_COMPILED_ASSETS_DIR` and must not overlap active state
- running-instance edits happen through control-plane against the generated active state
- control-plane export is not an automatic Git overwrite
- baseline recovery back into this directory must follow repository workflow and review gates

Compile and sync:

- run `go run ./scripts/sync-system-truth` to traverse the `sources/` tree
- the sync step creates or updates `output/system-state/` by default:
  - `<asset-id>/meta.json`
  - `<asset-id>/parse_result.json`
  - `<asset-id>/compile_result.json`
  - `<asset-id>/pipeline.json`
  - `<asset-id>/.versions/*.json`
- run `go run ./scripts/build-system-assets` to generate:
  - `output/system-assets/manifest.json`
  - `output/system-assets/assets/*.json`
