# WW Bot

WhatsApp AI reply assistant with safety rails. Runs as a native desktop app (macOS/Windows) built with [Wails v3](https://v3.wails.io/).

## Features

- AI-powered auto-reply via OpenAI, Anthropic, or local CLI agents
- Per-contact approval queue — every draft is reviewed before sending
- Safety rails: rate limits, quiet hours, daily caps, kill-switch
- Credential redaction strips secrets before they reach the LLM
- Encrypted message history (AES-256-GCM, keys in OS keychain)
- Proactive messaging on schedules
- Live dashboard, activity log, contact management

## Getting Started

### Prerequisites

- [Go 1.25+](https://go.dev/dl/)
- [Bun](https://bun.sh/)
- [Wails v3 CLI](https://v3.wails.io/getting-started/installation/)

### Development

```bash
wails3 dev
```

Hot-reloads both frontend and backend changes.

### Production Build

```bash
wails3 task darwin:package      # macOS .app
wails3 task windows:package     # Windows .exe / NSIS installer
```

## Tech Stack

| Layer | Tech |
|---|---|
| Backend | Go, Wails v3 |
| Frontend | React 19, TypeScript, Tailwind CSS v4, shadcn/ui |
| Storage | SQLite (encrypted at rest) |
| Secrets | OS keychain via go-keyring |

## Disclaimer

**This project is not affiliated with, endorsed by, or connected to WhatsApp, Meta Platforms, Inc., or any of their subsidiaries.**

WW Bot uses an unofficial, community-maintained library ([whatsmeow](https://github.com/tulir/whatsmeow)) to interact with WhatsApp's protocol. This may violate [WhatsApp's Terms of Service](https://www.whatsapp.com/legal/terms-of-service). Consequences may include:

- Temporary or permanent ban of your WhatsApp account
- Loss of access to messages, groups, and contacts tied to that account

**By using this software, you accept full responsibility for any consequences arising from its use.** The authors and contributors of WW Bot:

- Make no guarantees about the availability, reliability, or safety of this software
- Are not liable for any account bans, data loss, or damages resulting from its use
- Do not encourage or endorse any violation of third-party terms of service
- Provide this software as-is, for educational and personal use only

If you depend on your WhatsApp account for business or critical communication, **do not use this software on that account**.

## License

[MIT](LICENSE)
