# SNMP Trap Webhook Notification System

## Overview

Real-time ONU event monitoring via SNMP Trap listener with multi-platform
webhook notifications (Discord, Slack, Telegram). Events are batched per
severity with configurable intervals, deduplicated per ONU, and re-verified
via SNMP before sending to eliminate false alarms.

## Architecture

```
                    ZTE C320 OLT
                         |
                   SNMP Trap (UDP 1620)
                         |
                    +-----------+
                    | Listener  |  Parse trap PDU, extract ONU data
                    +-----------+
                         |
                    +-----------+
                    | Handler   |  Invalidate cache + SNMP GET verify
                    +-----------+
                    /           \
            Offline?          Online?
               |                 |
        Batcher.Add()     Batcher.Remove()
               |            (clear stale entry)
               |
    +---------------------------+
    | Batcher (per-severity)    |
    |  CRITICAL: 5 min ticker   |
    |  HIGH:     1 hr ticker    |    PowerMonitor ──→ Batcher.Add()
    |  MEDIUM:   4 hr ticker    |    (cron/interval scan RX power)
    |  LOW:      8 hr ticker    |
    +---------------------------+
               |
         Flush timer hit
               |
    Re-verify setiap ONU (fresh SNMP)
               |
        Still alert? ──→ Formatter.FormatBatch()
        Recovered?   ──→ Skip
               |
    +---------------------+
    | WebhookClient       |
    | (retry + backoff)   |
    +---------------------+
               |
    Discord / Slack / Telegram
```

## Severity System

| Severity | Color | Events | Interval | Tindakan |
|----------|-------|--------|----------|----------|
| CRITICAL | 🔴 Merah | LOS, LOSi, LOFi, Offline, AuthFailed, PowerOff | 5 menit | Wajib visit ke Customer maksimal 1x24 jam |
| HIGH | 🟠 Orange | Logging, Synchronization (stuck) | 1 jam | Wajib visit maksimal 1x24 jam jika Hard Restart tidak Solved |
| MEDIUM | 🟡 Kuning | HighRxPower (> -8 dBm), LowRxPower (< -28 dBm) | 4 jam | Wajib visit maksimal 2x24 jam setelah notifikasi |
| LOW | 🔵 Biru | DyingGasp | 8 jam | Koordinasi kepada Customer untuk memastikan tidak ada kendala kelistrikan |

## Flow Per Category

### CRITICAL (LOS, Offline, AuthFailed, PowerOff, LOSi, LOFi)

**Source:** SNMP Trap dari OLT

1. OLT kirim trap saat ONU state change
2. Listener parse trap PDU → extract Board/PON/ONU + customer data
3. Handler invalidate Redis cache → SNMP GET fresh status
4. Status offline → `Batcher.Add()` ke CRITICAL queue (dedup by Board/PON/ONU)
5. Status online → `Batcher.Remove()` hapus entry lama jika ada
6. Setiap 5 menit → flush:
   - Re-verify setiap ONU via SNMP GET (cache di-invalidate dulu)
   - Masih offline → kirim ke Discord
   - Sudah online (recovered) → skip, tidak kirim

### HIGH (Logging, Synchronization)

**Source:** SNMP Trap dari OLT

1. Same flow as CRITICAL
2. Handler verify status = "Logging" atau "Synchronization" (stuck state)
3. Masuk HIGH queue, flush setiap 1 jam
4. Re-verify: masih stuck → kirim. Sudah online → skip.

### MEDIUM (HighRxPower, LowRxPower)

**Source:** PowerMonitor (periodic cron/interval scan)

1. PowerMonitor scan semua ONU di semua Board/PON
2. Parse RX Power → bandingkan dengan threshold:
   - `> -8 dBm` → HighRxPower (overload risk)
   - `< -28 dBm` → LowRxPower (weak signal, approaching LOS)
3. `Batcher.Add()` ke MEDIUM queue (dedup by Board/PON/ONU)
4. Setiap 4 jam → flush:
   - Re-verify RX Power via fresh SNMP GET
   - Masih abnormal → kirim ke Discord
   - Sudah normal (antara -28 dan -8 dBm) → skip

### LOW (DyingGasp)

**Source:** SNMP Trap dari OLT

1. Same flow as CRITICAL
2. Handler verify status = "Dying Gasp" (power failure)
3. Masuk LOW queue, flush setiap 8 jam
4. Re-verify: masih Dying Gasp → kirim. Sudah online → skip.

## Key Features

### Deduplication

Setiap ONU hanya muncul 1x per batch, menggunakan key `Board-PON-OnuID`.
Jika trap masuk berulang untuk ONU yang sama, entry di-update (bukan ditambah).

### Cache Invalidation

Saat trap masuk DAN saat flush, Redis cache di-invalidate dulu sebelum
SNMP GET. Ini memastikan data selalu fresh dari OLT, bukan dari cache
yang mungkin stale.

### Recovery Detection

- **Saat trap masuk:** Jika handler verify ONU sudah Online, entry di-Remove
  dari batcher queue.
- **Saat flush:** Setiap ONU di re-verify lagi. Yang sudah recovered di-skip.
- **Hasilnya:** Zero false alarm untuk ONU yang recover sebelum flush.

### Multi-Platform Formatter

Auto-detect platform dari URL, override via `TRAP_WEBHOOK_TYPE`:

| URL Pattern | Platform | Format |
|-------------|----------|--------|
| `discord.com/api/webhooks` | Discord | Rich embed, quote separator |
| `hooks.slack.com` | Slack | Attachments + blocks, `===` separator |
| `api.telegram.org/bot` | Telegram | HTML, `===` separator |
| Other | Generic | Raw JSON array |

### Batch Message Format

```
🔴 [CRITICAL] 3 ONU LOS

Nama Lengkap : Pelanggan A
Alamat Lengkap : Alamat Lengkap A
Event : LOS
Board/PON/ONU : 1/5/23
RX Power : -22.50 dBm
Terakhir Offline : 20-04-2026 / 17:56:18 WIB

> (separator — quote di Discord, === di Slack/Telegram)

Nama Lengkap : Pelanggan B
Alamat Lengkap : Alamat Lengkap B
Event : LOS
Board/PON/ONU : 1/13/40
Terakhir Offline : 20-04-2026 / 18:01:30 WIB


⚠️ Tindakan
Wajib visit ke Customer maksimal 1x24 jam
```

## Environment Variables

### Webhook Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `TRAP_ENABLED` | `false` | Enable SNMP trap listener |
| `TRAP_PORT` | `1620` | UDP port for trap listener |
| `TRAP_COMMUNITY` | (from SNMP_COMMUNITY) | SNMP community for trap auth |
| `TRAP_WEBHOOK_URL` | (empty) | Webhook endpoint URL |
| `TRAP_WEBHOOK_TYPE` | (auto-detect) | Override: `discord\|slack\|telegram\|generic` |
| `TRAP_WEBHOOK_CHAT_ID` | (empty) | Telegram chat/group ID |
| `TRAP_WEBHOOK_RETRIES` | `3` | Retry attempts on failure |
| `TRAP_WEBHOOK_TIMEOUT` | `10` | HTTP timeout per request (seconds) |

### Per-Severity Batch Intervals

| Variable | Default | Description |
|----------|---------|-------------|
| `TRAP_CRITICAL_INTERVAL` | `300` | CRITICAL flush interval (seconds) |
| `TRAP_HIGH_INTERVAL` | `3600` | HIGH flush interval (seconds) |
| `TRAP_MEDIUM_INTERVAL` | `14400` | MEDIUM flush interval (seconds) |
| `TRAP_LOW_INTERVAL` | `28800` | LOW flush interval (seconds) |

### RX Power Thresholds (for MEDIUM alerts)

| Variable | Default | Description |
|----------|---------|-------------|
| `RX_POWER_HIGH_THRESHOLD` | `-8.0` | Alert when RX > this (dBm) |
| `RX_POWER_LOW_THRESHOLD` | `-25.0` | Alert when RX < this (dBm) |

### Power Monitor (source of MEDIUM events)

| Variable | Default | Description |
|----------|---------|-------------|
| `POWER_MONITOR_ENABLED` | `false` | Enable RX power scanner |
| `POWER_MONITOR_INTERVAL` | `300` | Scan interval (seconds, 0 = disable) |
| `POWER_MONITOR_CRON` | (empty) | Cron schedule (5-field) |
| `POWER_MONITOR_TIMEZONE` | (empty) | IANA timezone for cron |

## OLT Configuration (ZTE C320)

Configure OLT to send traps to the service:

```
configure terminal
snmp-server host <SERVICE_IP> version 2c <COMMUNITY> enable notifications target-addr-name trap-monitor udp-port 1620
```

Verify:
```
show snmp
```

## File Structure

```
internal/trap/
  ├── listener.go            # SNMP trap UDP listener + PDU parser
  ├── handler.go             # Event handler: SNMP verify + route to batcher
  ├── batcher.go             # Per-severity batch queue + dedup + re-verify
  ├── power_monitor.go       # Periodic RX power scanner (MEDIUM source)
  ├── webhook.go             # HTTP client with retry + backoff
  ├── formatter.go           # Interface + severity helpers + factory
  ├── formatter_discord.go   # Discord rich embed format
  ├── formatter_slack.go     # Slack attachments + blocks format
  ├── formatter_telegram.go  # Telegram HTML format
  ├── formatter_generic.go   # Raw JSON fallback
  ├── listener_test.go       # Listener + parser tests
  ├── handler_test.go        # Handler + verify + resolve tests
  ├── batcher_test.go        # Batcher + dedup + re-verify tests
  ├── power_monitor_test.go  # Power monitor tests
  ├── webhook_test.go        # Webhook client + retry tests
  └── formatter_test.go      # All formatter + batch format tests
```

## Test Coverage

```
internal/trap: 100.0% of statements
Full suite:    19/19 packages pass
```

### Test Scenarios Covered

**Handler:**
- Verified offline (LOS, DyingGasp, PowerOff, AuthFailed, Logging, Syncing) → alert
- Verified online → skip (no false alarm)
- Fetcher error → skip (safe default)
- Cache invalidation before SNMP verify
- Online ONU removes from batcher queue
- Enrichment prefers SNMP data over trap data
- Nil webhook, nil fetcher, nil batcher → no panic

**Batcher:**
- Dedup by Board/PON/ONU key
- Per-severity flush timers
- Re-verify at flush: still offline → send
- Re-verify at flush: recovered → skip
- Re-verify MEDIUM: RX power still abnormal → send
- Re-verify MEDIUM: RX power normalized → skip
- Re-verify MEDIUM: invalid RX power → skip
- Close flushes remaining events
- Format error → logged, not crash
- Remove clears ONU from all queues

**Formatter:**
- Single event format (all 4 platforms)
- Batch format with multiple ONUs (all 4 platforms)
- Severity colors, labels, actions
- WIB timestamp formatting with fallback
- Field truncation
- Empty/missing fields → dash placeholder
- Discord quote separator, Slack/Telegram === separator

**Listener:**
- Parse snmpTrapOID (offline/online/unknown)
- Parse ONU data OIDs (name, type, description, serial)
- Serial number comma-strip ("1,CDTCAF857ECE" → "CDTCAF857ECE")
- Board/PON encoding (board1Base/board2Base math)
- OID prefix collision handling (`.1.1` vs `.1.18`)
- Unrecognized traps, empty variables, nil callback

**Power Monitor:**
- High/Low RX power detection
- Normal power → no alert
- Duplicate alert suppression (30 min cooldown)
- SetBatcher routes to batcher instead of direct webhook
- Cron + interval + combined + disabled modes
- Invalid cron/timezone handling
- Panic recovery in scan

## Changelog (this session)

### Added
- Multi-platform webhook formatter (Discord, Slack, Telegram, Generic)
- 4-tier severity system (CRITICAL/HIGH/MEDIUM/LOW) with color, label, action
- Per-severity batch intervals with configurable env vars
- Event deduplication per ONU within batch window
- Double SNMP verification (on trap receive + on batch flush)
- Cache invalidation before every SNMP status check
- Recovery detection: online ONU removed from batch queue
- MEDIUM events routed through batcher (was direct webhook)
- OLT SSH alias (`olt` command in `.zshrc`)
- Webhook test CLI (`cmd/webhook-test/`)

### Changed
- Trap handler no longer trusts trap OID for event type — always verifies via SNMP
- Listener parses snmpTrapOID, ONU name/type/description/serial from trap PDU
- OID prefix matching uses `.` suffix to avoid collision (`.1.1` vs `.1.18`)
- Handler `ONUDetailFetcher` interface extended with `InvalidateONUCache`
- PowerMonitor routes alerts through batcher when available

### Fixed
- False alarm: online ONU reported as LOS (trap OID mapping removed)
- Stale cache: handler now invalidates before every SNMP verify
- Race condition: ONU recovers between trap and flush → re-verify catches it
- Duplicate entries: same ONU appearing multiple times in batch
- OID prefix collision: serial OID matched by type OID prefix
- Severity migration: ONU can only exist in one severity queue at a time — re-adding to a different severity removes from the old queue
- Batcher re-verify: flush checks that ONU still matches the severity of its queue before sending (prevents stale severity mismatch)
