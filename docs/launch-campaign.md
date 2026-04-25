# SFree Launch Campaign — Hacker News & Reddit

## 1. Show HN Post

### Title

```
Show HN: SFree – Combine Google Drive, Telegram, and S3 into one object store
```

### Body

```
Hi HN,

I built SFree, an open-source tool that turns multiple free-tier and personal
storage services into a single object store.

You register your Google Drive accounts, Telegram bots, or S3-compatible buckets
(MinIO, Backblaze B2, Wasabi, etc.) as storage sources. Then you create a bucket,
pick which sources back it, and SFree handles the rest: uploads get split into
chunks and distributed round-robin across your sources. Downloads reassemble
from the chunk manifest. Every bucket also gets auto-generated S3 credentials,
so you can use any S3 SDK or tool (rclone, mc, boto3) to read and write objects.

What actually works today (v0.1.0):

- Go backend with REST API and API docs
- Three storage backends: Google Drive, Telegram, S3-compatible
- S3-compatible endpoint with AWS SigV4 auth
- React web UI for signup, bucket management, file upload/download/preview
- Public share links — generate time-limited URLs for any file
- GitHub OAuth login (plus HTTP Basic Auth)
- CLI tool (sfree) for power users
- Configurable chunk distribution strategies (round-robin, weighted)
- Docker Compose one-command setup with MongoDB + MinIO

What it does NOT do (being upfront):

- No replication — chunks are distributed, not copied. If you lose an upstream
  source, those files are gone.
- No erasure coding.
- Auth is functional but not production-hardened.
- Rate limiting is in-memory and basic, not a full production abuse protection
  layer.

This is an early-stage project aimed at self-hosters and homelab tinkerers who
want to unify their scattered free storage behind one interface. It is not a
replacement for production object storage.

Tech stack: Go 1.25, MongoDB, React 19 + Vite, Woodpecker CI.
License: MIT.

GitHub: https://github.com/siberianbearofficial/sfree

Happy to answer questions about the architecture or where this is headed.
```

---

## 2. Reddit Post Drafts

### r/selfhosted

**Title:** `SFree — self-hosted tool to combine Google Drive, Telegram, and S3 storage into one API`

**Body:**

```
I want to share a project I've been working on: SFree, an open-source
(MIT-licensed) tool that turns your existing storage accounts into a unified
object store.

**The problem:** You've got 15 GB free on Google Drive, a Telegram bot that
can store files, and maybe a MinIO instance on a spare VPS. Using them together
is a manual juggling act.

**What SFree does:** You register those services as "sources," create a bucket
backed by whichever sources you choose, and SFree splits uploads into chunks
distributed round-robin across them. Every bucket also gets S3-compatible
credentials, so tools like rclone or mc just work.

**What works right now (v0.1.0):**
- Go backend with full REST API + API docs
- Google Drive, Telegram, and S3-compatible backends
- S3-compatible endpoint (AWS SigV4 auth)
- React web UI for buckets, files, preview, and public share links
- GitHub OAuth + CLI tool for power users
- Configurable distribution strategies (round-robin, weighted)
- Docker Compose one-command setup

**Honest caveats:**
- Early stage — no replication or erasure coding
- Auth is functional, not production-hardened
- Losing an upstream source = data loss for affected chunks
- Rate limiting exists, but it is in-memory and still basic

This is aimed at homelab/self-hosting experimentation, not production
workloads. If you're interested in the approach or want to contribute, the
repo is here:

https://github.com/siberianbearofficial/sfree

Feedback and ideas welcome — especially around which backends people would
want next.
```

### r/opensource

**Title:** `SFree: Open-source unified storage — pool Google Drive, Telegram, and S3 behind one API (MIT, Go)`

**Body:**

```
SFree is an MIT-licensed tool that combines multiple storage services into a
single object store with an S3-compatible interface.

**How it works:**
1. Register storage sources (Google Drive, Telegram bots, S3-compatible services)
2. Create a bucket backed by your chosen sources
3. Upload files through REST, the web UI, or the S3-compatible endpoint
4. SFree chunks, distributes, and reassembles objects from the saved manifest
5. Use any S3 SDK/tool via auto-generated per-bucket credentials

**Tech stack:** Go 1.25, MongoDB, React 19, Docker Compose, Woodpecker CI.

This is early-stage software — no replication, no erasure coding, experimental.
But the core loop works: chunk, distribute, reassemble, serve over S3, and apply
basic request limits. Plus public share links, GitHub OAuth, a CLI, and
configurable distribution strategies shipped in v0.1.0.

Contributions welcome — the codebase is clean Go with API docs and CI.

https://github.com/siberianbearofficial/sfree
```

### r/homelab

**Title:** `Built a tool to pool my free Google Drive + Telegram + MinIO into one S3-compatible store`

**Body:**

````
I had a bunch of scattered free storage — Google Drive accounts, a Telegram
bot, a MinIO bucket on a Pi — and got tired of managing them separately.

SFree is an open-source (MIT) Go tool that unifies them behind one REST API
and one S3-compatible endpoint. You register your storage services, create
buckets, and it handles chunking and round-robin distribution across backends.
Every bucket gets its own S3 credentials, so rclone, mc, and boto3 work
out of the box.

**Setup is Docker Compose + Go binary:**
```bash
cd api-go && docker compose up -d   # MongoDB
ENV=local go run ./cmd/server        # API on :8080
```

**What I'm running it with:**
- Google Drive (best metadata/quota reporting)
- Telegram bot (good for small-chunk storage, API-only)
- MinIO on a VPS (S3-compatible, API-only)

**Also ships with:** public share links, GitHub OAuth, a CLI tool, and
configurable distribution strategies (round-robin or weighted).

**Fair warnings:**
- No chunk replication — if a source goes down, affected files are gone
- Auth is functional but not production-hardened — run it behind a reverse proxy
- Rate limiting is basic and in-memory, so keep normal proxy protections in
  front of public deployments
- This is experimental, not for irreplaceable data

If you're into storage hacking or want to help add backends, check it out:

https://github.com/siberianbearofficial/sfree

Would love to hear what backends would be most useful to add — Dropbox?
OneDrive? FTP?
````

---

## 3. Launch Timing Recommendation

### Optimal window

**Tuesday–Thursday, 8:00–10:00 AM ET (US East Coast)**

Rationale:
- HN traffic peaks during US work hours on weekdays. Tuesday–Thursday
  historically gets the best engagement for Show HN posts.
- Reddit self-hosting communities (r/selfhosted, r/homelab) are most active
  on weekday evenings US time, but posting in the morning lets the post
  build momentum throughout the day.
- Avoid Mondays (buried in weekend catchup) and Fridays (low follow-through).
- Avoid weekends — lower technical audience engagement.

### Launch sequence

1. **T-0**: Verify README, Quick Start, and repo links are all clean.
   v0.1.0 released, landing page live, demo visuals in README.
2. **T+0 (morning)**: Post Show HN. Do NOT post Reddit simultaneously.
3. **T+2 hours**: If HN post gains traction (10+ points), post to r/selfhosted.
4. **T+4 hours**: Post to r/homelab.
5. **T+6 hours**: Post to r/opensource.
6. **T+1 day**: Cross-post to secondary channels (see section 4).

Staggering prevents looking like a spam blitz and lets you tailor messaging
based on early reactions.

### Pre-launch checklist

- [x] v0.1.0 released and tagged
- [x] Landing page merged and live
- [x] Demo visuals (annotated screenshots) in README
- [ ] README Quick Start verified end-to-end (fresh clone to running)
- [ ] API docs accessible and accurate
- [ ] Docker Compose brings up a working instance
- [x] GitHub repo description is live and matches the current prototype caveats
- [x] GitHub topics are set intentionally for launch discovery (`storage`, `s3`,
      `self-hosted`, `google-drive`, `telegram`, `homelab`,
      `object-storage`, `golang`, plus closely related supporting topics)
- [x] GitHub repo license badge is visible in README
- [ ] GitHub social preview image is uploaded in repo Settings and matches the
      README/launch visuals

---

## 4. Communities and Channels to Cross-Post

### Primary (launch day / day-after)

| Channel | Why | Post style |
|---|---|---|
| **Hacker News** (Show HN) | Largest technical audience, star driver | Technical deep-dive, honest about limitations |
| **r/selfhosted** (~500k members) | Core target audience | Problem/solution framing, setup instructions |
| **r/homelab** (~1.5M members) | Adjacent audience, hardware tinkerers | Personal story angle, "here's my setup" |
| **r/opensource** (~200k members) | OSS discovery community | Tech stack + contribution angle |

### Secondary (day 2–3)

| Channel | Why | Post style |
|---|---|---|
| **r/golang** | Go developer community, potential contributors | Architecture/implementation angle |
| **r/DataHoarder** | Storage maximizers | "Pool your free tiers" angle |
| **Lobsters** (lobste.rs) | HN alternative, technical audience | Similar to HN post, shorter |
| **dev.to** | Developer blog platform | Tutorial-style "How I built..." |

### Tertiary (week 1–2)

| Channel | Why | Post style |
|---|---|---|
| **Awesome Self-Hosted** (GitHub list) | Long-tail discovery, backlink | Submit PR to add SFree |
| **AlternativeTo** | People searching for storage solutions | List as alternative to multi-cloud tools |
| **Product Hunt** | General tech audience | Only if HN/Reddit response validates interest |
| **Telegram groups** (self-hosting, Go) | Niche but relevant | Short announcement + link |
| **Discord** (selfhosted, homelab servers) | Community engagement | Casual share, not promotional |

### Do NOT post to

- General programming subreddits (r/programming) — too broad, will get buried
- Twitter/X as primary channel — no existing audience to amplify
- Paid promotion — premature for current stage
