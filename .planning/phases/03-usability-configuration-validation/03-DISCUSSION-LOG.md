# Phase 3: Usability — Configuration & Validation - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-07-09
**Phase:** 3-Usability — Configuration & Validation
**Areas discussed:** Config file structure, Precedence hierarchy, Output directory, Widevine device flags, Validation strategy, QOL fixes

---

## Config File Structure

| Option | Description | Selected |
|--------|-------------|----------|
| JSON only | Simplest to implement, matches USAB-02 | ✓ |
| JSON + YAML | Needs gopkg.in/yaml.v3 dep, YAML supports comments | |
| All CLI flags | audio-lang, subs-lang, video-quality, audio-quality, workers, output-dir, etp-rt, widevine-device | ✓ |
| Only quality/langs | Only persistent settings in config, session things as flags | |
| Let the agent decide | Agent picks based on typical CLI conventions | |
| XDG spec first | Check $XDG_CONFIG_HOME, fallback to ~/.config/animeheaven/ | ✓ |
| Fixed path only | Always ~/.config/animeheaven/ | |
| Optional — warn if invalid | Invalid JSON warns and continues with defaults | |
| Optional — silent | No warning on invalid JSON | |
| Other (freeform) | Generate default config if file missing — minimal skeleton | ✓ |

**User's choice:** JSON only. All CLI flags persistable. XDG spec first. Optional file — if missing, auto-generate minimal skeleton. If invalid JSON, warn and continue with defaults. Explicit-only overrides (unset fields fall through).

---

## Precedence Hierarchy

| Option | Description | Selected |
|--------|-------------|----------|
| Explicit only | Only explicitly-set config values override defaults | ✓ |
| Full replace | Whole config object replaces all defaults | |
| Env var replaces | CRUNCHYROLL_CLIENT_AUTH replaces hardcoded cred when set | ✓ |
| Document only | Hardcoded credential always used, env var just for docs | |
| Full hierarchy | Flag > env var > config > .env > default | |
| Deprecate .env | CLI flag + config file replace .env entirely | ✓ |
| CLI layer (main.go) | main.go reads env var, passes to client | ✓ |
| API client layer | Pass both flag value and env var to api.New | |

**User's choice:** Explicit-only overrides. CRUNCHYROLL_CLIENT_AUTH replaces hardcoded credential. Deprecate .env file approach. CRUNCHYROLL_ETP_RT handled in CLI layer (main.go).

---

## Output Directory

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, series subfolder | Series subfolder inside output dir | ✓ |
| No subfolder | Files go directly into --output-dir | |
| Auto-create | os.MkdirAll creates missing output dir | |
| Error if missing | Error if output dir doesn't exist | ✓ |

**User's choice:** Series subfolder inside output dir. Error if output dir doesn't exist (don't auto-create). Default behavior (no flag) stays as-is.

---

## Widevine Device Flags

| Option | Description | Selected |
|--------|-------------|----------|
| Single --widevine-device flag | Accepts .wvd path or directory with client_id.bin + private_key.pem | ✓ |
| Three separate flags | --widevine-wvd, --widevine-client-id, --widevine-private-key | |
| Keep env var, remove .env | Legacy env var names remain as env fallback, .env file removed | ✓ |
| Remove env vars too | CLI flag or config file only | |

**User's choice:** Single `--widevine-device` flag. Keep env var fallbacks for legacy names, remove `.env` file entirely.

---

## Validation Strategy

| Option | Description | Selected |
|--------|-------------|----------|
| Structure + ID length | Validate /watch/ or /series/ pattern + 9-14 char ID | ✓ |
| Structure + HTTP check | Also HEAD request to verify URL is reachable | |
| LookPath only | Quick check, 99% coverage | |
| LookPath + --version | Confirms binary actually runs | ✓ |
| Hard error — abort | Clear message and exit 1 if FFmpeg missing | ✓ |
| Warning — proceed | Let user decide, fail later at mux time | |

**User's choice:** Structure + ID length validation. LookPath + --version for FFmpeg. Hard error with clear message if FFmpeg missing.

---

## QOL Fixes

| Option | Description | Selected |
|--------|-------------|----------|
| Bundled in one plan | QOL-04 + QOL-05 together (both processURL) | ✓ |
| Separate plans | QOL-04 and QOL-05 as individual plans | |
| Yes — after config plan | QOL-08 depends on config resolution for final effective langs | ✓ |
| Independent — refine later | Parse at flag.Parse time, refine later | |
| Separate plans | QOL-07 and QOL-08 independent | ✓ |
| Bundle together | Both small utility improvements in one plan | |

**User's choice:** QOL-04 + QOL-05 bundled (both in processURL). QOL-08 after config plan. QOL-07 standalone. QOL-07 and QOL-08 are separate plans from each other.

---

## the agent's Discretion

None — all decisions made explicitly by the user.

## Deferred Ideas

None — discussion stayed within Phase 3 scope.
