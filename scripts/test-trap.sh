#!/bin/bash
#
# SNMP Trap Testing Script
# Tests the trap listener locally without a real OLT device
#
# Usage:
#   ./scripts/test-trap.sh          # Run all tests
#   ./scripts/test-trap.sh send     # Only send traps (webhook server already running)
#   ./scripts/test-trap.sh webhook  # Only start webhook receiver
#

set -e

TRAP_PORT="${TRAP_PORT:-1620}"
TRAP_COMMUNITY="${TRAP_COMMUNITY:-public}"
WEBHOOK_PORT=9999

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

# ZTE C320 OID constants (matching config/oid_generator.go)
OID_ONU_STATUS=".1.3.6.1.4.1.3902.1082.500.10.2.3.8.1.4"
OID_ONU_OFF_REASON=".1.3.6.1.4.1.3902.1082.500.10.2.3.8.1.7"
OID_ONU_INDEX=".1.3.6.1.4.1.3902.1082.500.10.2.3.3.1.2"

# Board/PON encoded values (from config/oid_generator.go)
# Board 1 PON 1 = 285278465 (285278464 + 1)
# Board 1 PON 5 = 285278469 (285278464 + 5)
# Board 2 PON 3 = 285278723 (285278720 + 3)
BOARD1_PON1=285278465
BOARD1_PON5=285278469
BOARD2_PON3=285278723

echo -e "${CYAN}========================================${NC}"
echo -e "${CYAN}  SNMP Trap Testing Tool${NC}"
echo -e "${CYAN}  Target: localhost:${TRAP_PORT}${NC}"
echo -e "${CYAN}  Community: ${TRAP_COMMUNITY}${NC}"
echo -e "${CYAN}========================================${NC}"
echo ""

# ─── Functions ────────────────────────────────────────────

start_webhook_receiver() {
    echo -e "${YELLOW}Starting webhook receiver on port ${WEBHOOK_PORT}...${NC}"
    python3 -c "
from http.server import HTTPServer, BaseHTTPRequestHandler
import json, sys

class WebhookHandler(BaseHTTPRequestHandler):
    count = 0

    def do_POST(self):
        WebhookHandler.count += 1
        length = int(self.headers.get('Content-Length', 0))
        body = self.rfile.read(length)
        try:
            data = json.loads(body)
            print(f'\n--- Webhook #{WebhookHandler.count} ---')
            print(json.dumps(data, indent=2, ensure_ascii=False))
            print('---')
        except json.JSONDecodeError:
            print(f'Invalid JSON: {body}')
        self.send_response(200)
        self.end_headers()
        self.wfile.write(b'OK')

    def log_message(self, format, *args):
        pass  # Suppress default HTTP logging

print(f'Webhook receiver listening on http://0.0.0.0:${WEBHOOK_PORT}')
print('Waiting for trap events...')
print('Press Ctrl+C to stop')
print()
HTTPServer(('0.0.0.0', ${WEBHOOK_PORT}), WebhookHandler).serve_forever()
" &
    WEBHOOK_PID=$!
    sleep 1
    echo -e "${GREEN}Webhook receiver started (PID: ${WEBHOOK_PID})${NC}"
}

check_prerequisites() {
    if ! command -v snmptrap &> /dev/null; then
        echo -e "${RED}Error: snmptrap not found${NC}"
        echo "Install with: brew install net-snmp"
        exit 1
    fi

    # Check if app is running
    if ! curl -s http://localhost:8081/health > /dev/null 2>&1; then
        echo -e "${RED}Error: App not running on localhost:8081${NC}"
        echo "Start with: task dev"
        exit 1
    fi
    echo -e "${GREEN}App is running${NC}"

    # Check if trap listener is active (try sending a test UDP packet)
    echo -e "${GREEN}Prerequisites OK${NC}"
    echo ""
}

send_trap() {
    local description="$1"
    local oid_suffix="$2"
    local value_type="$3"
    local value="$4"
    local extra_oid="$5"
    local extra_type="$6"
    local extra_value="$7"

    echo -e "${YELLOW}Sending: ${description}${NC}"

    if [ -n "$extra_oid" ]; then
        snmptrap -v 2c -c "$TRAP_COMMUNITY" "localhost:${TRAP_PORT}" '' \
            "${oid_suffix}" "${value_type}" "${value}" \
            "${extra_oid}" "${extra_type}" "${extra_value}" 2>&1
    else
        snmptrap -v 2c -c "$TRAP_COMMUNITY" "localhost:${TRAP_PORT}" '' \
            "${oid_suffix}" "${value_type}" "${value}" 2>&1
    fi

    if [ $? -eq 0 ]; then
        echo -e "${GREEN}  -> Sent OK${NC}"
    else
        echo -e "${RED}  -> Failed${NC}"
    fi
    sleep 1
}

# ─── Test Cases ───────────────────────────────────────────

run_trap_tests() {
    echo -e "${CYAN}--- Test 1: ONU LOS (Board 1, PON 1, ONU 23) ---${NC}"
    send_trap "ONU LOS - Board 1/PON 1/ONU 23" \
        "${OID_ONU_STATUS}.${BOARD1_PON1}.23" "i" "2"

    echo -e "${CYAN}--- Test 2: ONU DyingGasp (Board 1, PON 5, ONU 7) ---${NC}"
    send_trap "ONU DyingGasp - Board 1/PON 5/ONU 7" \
        "${OID_ONU_STATUS}.${BOARD1_PON5}.7" "i" "5"

    echo -e "${CYAN}--- Test 3: ONU Offline (Board 2, PON 3, ONU 15) ---${NC}"
    send_trap "ONU Offline - Board 2/PON 3/ONU 15" \
        "${OID_ONU_STATUS}.${BOARD2_PON3}.15" "i" "7"

    echo -e "${CYAN}--- Test 4: ONU Online/Recovery (Board 1, PON 1, ONU 23) ---${NC}"
    echo -e "${YELLOW}  (This should NOT trigger webhook - online event filtered)${NC}"
    send_trap "ONU Online - Board 1/PON 1/ONU 23" \
        "${OID_ONU_STATUS}.${BOARD1_PON1}.23" "i" "4"

    echo -e "${CYAN}--- Test 5: ONU PowerOff with reason (Board 1, PON 1, ONU 10) ---${NC}"
    send_trap "ONU PowerOff with reason - Board 1/PON 1/ONU 10" \
        "${OID_ONU_STATUS}.${BOARD1_PON1}.10" "i" "7" \
        "${OID_ONU_OFF_REASON}.${BOARD1_PON1}.10" "i" "9"

    echo -e "${CYAN}--- Test 6: ONU AuthFailed (Board 2, PON 3, ONU 1) ---${NC}"
    send_trap "ONU AuthFailed - Board 2/PON 3/ONU 1" \
        "${OID_ONU_STATUS}.${BOARD2_PON3}.1" "i" "6"

    echo ""
    echo -e "${GREEN}========================================${NC}"
    echo -e "${GREEN}  All trap tests sent!${NC}"
    echo -e "${GREEN}  Check webhook receiver output above${NC}"
    echo -e "${GREEN}  Expected: 5 webhooks (Test 4 filtered)${NC}"
    echo -e "${GREEN}========================================${NC}"
}

# ─── Main ─────────────────────────────────────────────────

cleanup() {
    if [ -n "$WEBHOOK_PID" ]; then
        kill "$WEBHOOK_PID" 2>/dev/null
        echo -e "\n${YELLOW}Webhook receiver stopped${NC}"
    fi
}
trap cleanup EXIT

case "${1:-all}" in
    webhook)
        start_webhook_receiver
        wait "$WEBHOOK_PID"
        ;;
    send)
        check_prerequisites
        run_trap_tests
        ;;
    all)
        check_prerequisites
        start_webhook_receiver
        echo ""
        sleep 2
        run_trap_tests
        echo ""
        echo -e "${YELLOW}Waiting 5s for async webhooks to arrive...${NC}"
        sleep 5
        echo -e "${YELLOW}Press Ctrl+C to stop${NC}"
        wait "$WEBHOOK_PID"
        ;;
    *)
        echo "Usage: $0 [all|send|webhook]"
        exit 1
        ;;
esac
