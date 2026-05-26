# ww-bot — Project Plan

> WhatsApp-assisted AI reply bot. A personal helper that reads your incoming WhatsApp
> messages and replies *as you*, under your rules, when you're busy — never spamming,
> free to run, privacy-respecting. Last updated: 2026-05-26. Status: **planning (no code yet).**

---

## 1. Vision — what & why

A Go + Wails desktop app that lives in your menu bar, linked to your WhatsApp. When
you're busy (or unsure how/whether to reply), it answers people on your behalf according
to per-contact rules and your daily context. It also does proactive things (morning
salaam + dua to Dad, check-ins on family) and notifies you of calls and likely scammers.

Core principles the whole design bends around:
- **Free forever** — no paid API as a default; ships with zero keys.
- **Privacy-first** — derived summaries persist; raw messages never hit disk.
- **Small single binary** — bundleable, nothing approaching 100 MB.
- **Protect the number** — anti-ban behavior is the top engineering priority.
- **No spam** — strictly a personal reply helper.

---

## 2. Decisions made

| Topic | Decision |
|---|---|
| WhatsApp number | **Primary number** (user accepted the ban risk; rails must earn it) |
| Desktop framework | **Wails v3 (alpha, pinned)** — for native system tray + headless background |
| Reply autonomy | **Autonomous + escalate only when unsure** (low confidence / missing info → draft for approval); per-contact trust can be raised later |
| Control channel | **Both** WhatsApp "Message Yourself" self-chat **and** the desktop UI |
| LLM backends | **Free-forever registry**; auto-detect fills a list the **user orders in settings** |
| LLM decision format | **Strict JSON contract** (portable across weak local models + CLI agents), not native tool-calling |
| Voice transcription | **Free-tier online (Groq default → Cloudflare fallback)**; local whisper.cpp deferred |
| Memory | **Summary-based per person**; **no vector DB for v1**; SQLite stores profiles + summaries only |

---

## 3. Tech stack (all pure-Go, single small binary, target < 100 MB)

| Layer | Pick | Notes |
|---|---|---|
| Desktop shell | **Wails v3** (alpha, pinned) | Native tray + headless/background; ~10 MB binary; MIT |
| Frontend | React + Vite + **Tailwind v4 (PostCSS)** + **shadcn/ui** | "ChatCN" = shadcn/ui; start from a template |
| WhatsApp | **`go.mau.fi/whatsmeow`** | Only maintained Go lib that reads personal chats + detects calls; session in SQLite; MPL-2.0 |
| Structured DB | **`modernc.org/sqlite`** (pure Go, no CGo) | Contact profiles, summaries, settings, audit log, draft queue |
| RAG/vectors | **None in v1** (chromem-go later if needed) | Memory is summary-based; a text column suffices |
| LLM client | **`openai/openai-go`** behind an `LLMProvider` interface | One client, base-URL swap reaches many providers |
| Voice STT | Free-tier APIs behind a `Transcriber` interface | Groq default; OGG uploads directly, no ffmpeg |
| Scheduler | `robfig/cron` | Morning salaam/dua, check-ins |
| Secrets | OS keychain (`zalando/go-keyring`) | API keys + DB encryption key |

**Call detection works:** whatsmeow dispatches `*events.CallOffer` with the caller JID —
can't *answer* calls, but can notify ("Mom is calling").

---

## 4. Architecture — module map

```
ww-bot/
├─ wails v3 app (system tray, runs headless in background)
├─ frontend/        React + Vite + Tailwind v4 + shadcn/ui
└─ internal/
   ├─ wa/        whatsmeow: live events in, SINGLE send path out; HistorySync backfill
   ├─ inbox/     per-chat debounce window (~30s) + ephemeral last-10 RAM buffer (no disk)
   ├─ brain/     builds context (profile + summary + window) → provider → JSON decision
   ├─ llm/       provider registry: detect & rank CLI-agent / Ollama / free-tier / user-key
   ├─ stt/       free-tier transcription (Groq → Cloudflare …) behind one interface
   ├─ memory/    SQLite: contact profiles + rolling summaries only (encrypted)
   ├─ schedule/  cron: morning salaam/dua, check-ins; daily-plan ingestion
   ├─ approval/  draft queue → desktop/phone notify → approve / edit / reject
   ├─ commands/  slash commands (self-chat + UI)
   └─ safety/    the outbound gate: pacing, caps, jitter, quiet hours, kill-switch, guards
```

---

## 5. The life of a message

1. Message arrives via `whatsmeow`. **We don't react instantly** — held in a per-person
   buffer; wait ~30s of silence (debounce). A burst of 5 messages is treated as one thought.
   (Max-wait cap ~2–3 min so a non-stop talker still gets handled.)
2. If it's a **voice note**, transcribe it (free tier). If audio isn't clear enough →
   stay silent rather than guess (confidence gate via `no_speech_prob`/low logprob).
3. **Brain** builds context: contact profile ("this is Dad, speaks X, reply this way,
   trust tier") + rolling **summary** + last ~10 messages held only in RAM.
4. **LLM** (whatever free backend is configured) returns a strict JSON decision:
   `{action: reply|escalate|silent, text, confidence, memory_update, flags}`.
5. **Decision routing:**
   - Confident **and** known contact → reply goes out through the **outbound gate**.
   - Unsure / stranger / missing info / commitment category → **don't send**; show draft
     to approve/edit/reject (desktop or phone).
6. **Calls** → notify only ("Mom is calling"). **Scammers** → flagged, never auto-blocked.

**Proactive side:** scheduled good-morning salaam + dua to Dad, family check-ins — all
through the same throttled gate; afterward it tells you what it sent.

**Your control:** slash commands in the self-chat *and* desktop UI — pause everything,
set a contact's rules, or speak "today I'm doing X" so replies reflect your day.

---

## 6. Anti-ban machinery (top priority)

**Inbound debounce (`inbox/`):** per-conversation buffer + quiet timer (~30s, reset on
each new msg) with a 2–3 min max-wait cap. Flush the whole burst to the brain as ONE
context — never reply per-message. No instant read receipts / no instant "online" (bot
tells). Voice notes transcribed then folded into the batch.

**Outbound gate (`safety/`):** a single goroutine owns the send queue; the ONLY code path
that calls whatsmeow send. Enforces:
- Reaction delay (human beat before sending, never zero)
- Typing-presence simulation (duration ~ message length; optional bubble splitting)
- Rate governor: per-minute / per-hour / per-day caps + per-contact cooldown, with **jitter**
- Quiet hours (e.g. 11pm–7am; scheduled duas exempt)
- Backlog reconciliation (answer latest state once after being offline, don't blast old)
- Kill-switch / `/pause` checked here

ALL outbound (autonomous, proactive, approved drafts) funnels through this gate so caps
apply uniformly — this is what protects the number.

Starting defaults (tunable): 30s quiet window / 3 min max-wait; ~1 msg per 8–20s with
jitter; soft daily cap; quiet hours 11pm–7am.

---

## 7. Memory & privacy model

- **Raw messages live only in RAM.** Per active chat we keep an ephemeral rolling window
  (~last 10 messages), feed it to the model, then discard. Nothing raw is written to disk.
- **whatsmeow reality:** it's a live client, NOT an on-demand message store. It gives live
  events + a one-time HistorySync at link. The in-RAM window is how we get "last 10
  messages" context *without* persisting chats — this honors the privacy rule.
- **What persists (encrypted SQLite):** (a) contact profile — who they are, language, reply
  style, trust tier; (b) a rolling per-person **summary** the model updates over time; plus
  settings and an audit log of what the bot itself sent (our own data).
- **No vector DB for v1** — summary-based memory needs no embeddings. Add chromem-go later
  only if summaries outgrow simple storage.

---

## 8. Free LLM backends (registry, user-ordered in settings)

Ships with **no keys**. App auto-detects what's available; user orders the priority list.
All behind one `LLMProvider` interface; the brain doesn't care which answers.

1. **Installed agent CLIs** (zero key, uses existing subscription) — `claude -p` (Claude
   Code), `codex exec` (Codex), Gemini CLI, driven in headless/print mode.
2. **Local models** — Ollama (fully offline, free).
3. **Free-tier online** — OpenRouter `:free` models, Google Gemini free tier, Groq free tier.
4. **User-supplied paid key** — opt-in only; never our default.

Because free/local/CLI backends have weak or no native function-calling, the baseline is
the **strict JSON decision contract** (above), parsed by us. Native tool-calling is only an
optional enhancement where supported.

---

## 9. Free voice-transcription providers (BYO key)

Default order: **Groq → Cloudflare**. Numbers change often — keep them configurable.

| Provider | Model | Free quota | Resets | Card? | OpenAI `/audio/transcriptions`? | OGG direct? | Catch |
|---|---|---|---|---|---|---|---|
| **Groq** ⭐ | Whisper v3 / turbo | ~2,000 req & ~8 hrs/day | Daily | No | **Yes** | Yes | 25 MB file cap (fine) |
| **Cloudflare Workers AI** | Whisper / turbo | 10k neurons/day (~215–240 min) | Daily | No | No (custom REST) | Yes | needs adapter |
| **Speechmatics** | Batch/RT | ~8 hrs/month | Monthly | No | No | Yes | own API, 50+ langs |
| **IBM Watson** | Lite | 500 min/month | Monthly | No | No | Yes | instance deleted after 30d idle |
| **Azure Speech F0** | MS STT | 5 hrs/month | Monthly | Account | No | needs transcode | OGG fiddly |
| **Google Cloud STT** | Chirp/STT v2 | 60 min/month | Monthly | **Yes** | No | Yes (OGG_OPUS) | billing card required |
| **Gemini API** | 2.5-flash | ~250 req/day | Daily | No | Partial (chat-shape) | Yes | ⚠️ trains on free-tier data |

**One-time credits only (not the free path):** Deepgram ($200/1yr), AssemblyAI ($50),
Fireworks ($1, OpenAI-compatible), OpenAI ($5 trial — otherwise paid), Sarvam (₹1,000, Indian
languages), ElevenLabs Scribe (small monthly Free plan, minutes inconsistent across sources).

**Recommendation:** default **Groq** (recurring daily quota + true OpenAI-compatible +
OGG-native + multilingual + no card), auto-fallback **Cloudflare**. **Skip Gemini for voice**
(free tier may train on your audio — clashes with privacy).

STT interface gets built-in provider profiles (base-URL + key fields in settings). Most are a
base-URL swap; Cloudflare/Speechmatics/IBM/Azure each need a small adapter.

---

## 10. Safeguards & features (approved)

**Essential (v1):**
- **Human-takeover detection** — if you reply manually (whatsmeow sees own outgoing msgs),
  instantly back off that chat and cancel any pending draft. No double-replies.
- **Sensitive-content guard** — OTPs / 2FA codes / passwords / bank details NEVER sent to an
  LLM and NEVER auto-replied. Detect pattern, hard-skip, just notify.
- **Commitment guardrail** — bot never makes real-world promises on your behalf (meetings,
  money, yes/no invites, deadlines); those always escalate regardless of confidence/trust.
- **Group/status/broadcast policy** — NEVER auto-reply in groups/broadcasts/status by
  default; notify-only unless a group is explicitly opted in.

**High value:**
- **Edit-before-send + approve from phone** — approve/edit/reject drafts from the self-chat
  too, not just desktop. Draft timeout: expires silently after N min instead of sending stale.
- **Tone/style mirroring + reply in contact's language** — match your style (length, emoji,
  language mix) and the contact's language/script.
- **Status dashboard + quota tracking + audit log** — connection state, active backend,
  remaining free quota; auto-rotate providers when one is exhausted; log what was sent & why.
- **New-number warm-up ramp** — gradually increase activity on a freshly linked device.

**Open decisions (flagged, not built):**
- Local-only / paranoid mode (Ollama + local whisper, zero network) toggle.
- Disclosure-that-it's-a-bot ethics (default off; jurisdiction-dependent).

---

## 11. Risks & honest caveats

- **whatsmeow violates WhatsApp ToS.** Real, currently-elevated ban risk even for low-volume
  personal use (enforcement wave documented in whatsmeow issue #810). User is on their primary
  number, so a ban is costly — the anti-ban machinery is mandatory, not optional. Official
  Business Cloud API is ToS-safe but *cannot* read personal chats or see calls, so it can't do
  this job.
- **Free-tier numbers change frequently** (Gemini cut 50–80% in Dec 2025) — keep configurable.
- **Wails v3 is alpha** — pin a version; budget for breaking changes between alphas.
- **Local whisper.cpp** (if added later) needs CGo — kept out of the default build to stay small.

---

## 12. Next step

Build the riskiest unknown first: **whatsmeow QR pairing + a connection that survives
restart** — prove the WhatsApp link works before layering the brain on top. Then `inbox/`
(debounce + RAM window) and the `safety/` outbound gate, then the `brain/` + `llm/` registry.

---

## Key references
- whatsmeow: https://github.com/tulir/whatsmeow · events: https://pkg.go.dev/go.mau.fi/whatsmeow/types/events · ban-warning issue: https://github.com/tulir/whatsmeow/issues/810
- Wails v3 (alpha): https://v3.wails.io · system tray: https://v3alpha.wails.io/features/menus/systray/
- shadcn/ui: https://ui.shadcn.com/docs/installation/vite · Tailwind v4: https://ui.shadcn.com/docs/tailwind-v4
- modernc.org/sqlite: https://pkg.go.dev/modernc.org/sqlite
- openai-go: https://github.com/openai/openai-go · OpenRouter: https://openrouter.ai/docs/quickstart
- Groq STT: https://console.groq.com/docs/speech-to-text · rate limits: https://console.groq.com/docs/rate-limits
- Cloudflare Workers AI pricing: https://developers.cloudflare.com/workers-ai/platform/pricing/
