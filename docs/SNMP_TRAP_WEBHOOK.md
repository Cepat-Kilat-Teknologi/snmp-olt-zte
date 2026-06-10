# SNMP Trap Webhook Notification System

## Overview

Real-time ONU event monitoring via SNMP Trap listener with multi-platform
webhook notifications (Discord, Slack, Telegram). Events are batched per
severity with configurable intervals, deduplicated per ONU, and re-verified
via SNMP before sending to eliminate false alarms.

## Architecture

```
                    ZTE C320/C300 OLT
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
    Re-verify each ONU (fresh SNMP)
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

| Severity | Color | Events | Interval | Action (default) |
|----------|-------|--------|----------|------------------|
| CRITICAL | 🔴 Red | LOS, LOSi, LOFi, Offline, AuthFailed, PowerOff | 5 min | Mandatory customer visit within 1x24 hours |
| HIGH | 🟠 Orange | Logging, Synchronization (stuck) | 1 hr | Mandatory visit within 1x24 hours if Hard Restart does not resolve |
| MEDIUM | 🟡 Yellow | HighRxPower (> -8 dBm), LowRxPower (< -28 dBm) | 4 hr | Mandatory visit within 2x24 hours after notification |
| LOW | 🔵 Blue | DyingGasp | 8 hr | Coordinate with customer to ensure no electrical issues |

> **i18n:** Action messages are configurable via `TRAP_ACTION_CRITICAL`, `TRAP_ACTION_HIGH`,
> `TRAP_ACTION_MEDIUM`, `TRAP_ACTION_LOW` environment variables. Defaults are English.
> Set your own language in `.env` (e.g., Indonesian, Thai, etc.).

## Flow Per Category

### CRITICAL (LOS, Offline, AuthFailed, PowerOff, LOSi, LOFi)

**Source:** SNMP Trap from OLT

1. OLT sends trap on ONU state change
2. Listener parses trap PDU → extracts Board/PON/ONU + customer data
3. Handler invalidates Redis cache → SNMP GET fresh status
4. Status offline → `Batcher.Add()` to CRITICAL queue (dedup by Board/PON/ONU)
5. Status online → `Batcher.Remove()` clears stale entry if exists
6. Every 5 minutes → flush:
   - Re-verify each ONU via SNMP GET (cache invalidated first)
   - Still offline → send to webhook
   - Recovered (online) → skip, no notification

### HIGH (Logging, Synchronization)

**Source:** SNMP Trap from OLT

1. Same flow as CRITICAL
2. Handler verifies status = "Logging" or "Synchronization" (stuck state)
3. Enters HIGH queue, flush every 1 hour
4. Re-verify: still stuck → send. Online → skip.

### MEDIUM (HighRxPower, LowRxPower)

**Source:** PowerMonitor (periodic cron/interval scan)

1. PowerMonitor scans all ONUs across all Board/PON
2. Parses RX Power → compares against thresholds:
   - `> -8 dBm` → HighRxPower (overload risk)
   - `< -28 dBm` → LowRxPower (weak signal, approaching LOS)
3. `Batcher.Add()` to MEDIUM queue (dedup by Board/PON/ONU)
4. Every 4 hours → flush:
   - Re-verify RX Power via fresh SNMP GET
   - Still abnormal → send to webhook
   - Normalized (between -28 and -8 dBm) → skip

### LOW (DyingGasp)

**Source:** SNMP Trap from OLT

1. Same flow as CRITICAL
2. Handler verifies status = "Dying Gasp" (power failure)
3. Enters LOW queue, flush every 8 hours
4. Re-verify: still Dying Gasp → send. Online → skip.

## Key Features

### Deduplication

Each ONU appears only once per batch, keyed by `Board-PON-OnuID`.
If multiple traps arrive for the same ONU, the entry is updated (not duplicated).

### Cache Invalidation

On both trap receive AND flush, Redis cache is invalidated before
SNMP GET. This ensures data is always fresh from the OLT, not from
potentially stale cache.

### Recovery Detection

- **On trap receive:** If handler verifies ONU is Online, entry is removed
  from batcher queue.
- **On flush:** Each ONU is re-verified again. Recovered ONUs are skipped.
- **Result:** Zero false alarms for ONUs that recover before flush.

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
🔴 CRITICAL - 3 ONU LOS

Full Name : Customer A
Address : Address A
Event : LOS
Board/PON/ONU : 1/5/23
RX Power : -22.50 dBm
Last Online : 20-04-2026/17:56:18

> (separator — quote block on Discord, === on Slack/Telegram)

Full Name : Customer B
Address : Address B
Event : LOS
Board/PON/ONU : 1/13/40
Last Online : 20-04-2026/18:01:30


⚠️ Action
Mandatory customer visit within 1x24 hours
```

> Action text is configurable via `TRAP_ACTION_*` env vars for i18n.

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

### Per-Severity Action Messages (i18n)

| Variable | Default | Description |
|----------|---------|-------------|
| `TRAP_ACTION_CRITICAL` | Mandatory customer visit within 1x24 hours | Action text for CRITICAL |
| `TRAP_ACTION_HIGH` | Mandatory visit within 1x24 hours if Hard Restart does not resolve | Action text for HIGH |
| `TRAP_ACTION_MEDIUM` | Mandatory visit within 2x24 hours after notification | Action text for MEDIUM |
| `TRAP_ACTION_LOW` | Coordinate with customer to ensure no electrical issues | Action text for LOW |

### Per-Severity Batch Intervals

| Variable | Default | Description |
|----------|---------|-------------|
| `TRAP_CRITICAL_INTERVAL` | `300` | CRITICAL flush interval (seconds) |
| `TRAP_HIGH_INTERVAL` | `3600` | HIGH flush interval (seconds) |
| `TRAP_MEDIUM_INTERVAL` | `14400` | MEDIUM flush interval (seconds) |
| `TRAP_LOW_INTERVAL` | `28800` | LOW flush interval (seconds) |

### Repeat Notification Intervals

| Variable | Default | Description |
|----------|---------|-------------|
| `TRAP_CRITICAL_REPEAT` | `60` | Re-notify interval for CRITICAL (minutes, 0 = once only) |
| `TRAP_HIGH_REPEAT` | `60` | Re-notify interval for HIGH (minutes) |
| `TRAP_MEDIUM_REPEAT` | `0` | Re-notify interval for MEDIUM (minutes) |
| `TRAP_LOW_REPEAT` | `0` | Re-notify interval for LOW (minutes) |

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
Full suite:    20/20 packages pass
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
- Severity colors, labels, configurable action messages
- SetActionMessages custom + empty-skip behavior
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

## Changelog

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
