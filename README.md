# TCC-Matter Bridge

A Raspberry Pi service that bridges Honeywell Total Connect Comfort (TCC) thermostats to Apple HomeKit via the Matter protocol.

## Features

- Control your TCC thermostat from Apple Home app
- Web UI for configuration and monitoring
- Real-time temperature and status updates
- Automatic reconnection and error handling

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Raspberry Pi                             │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │                    Go Backend (:8080)                     │  │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────────┐   │  │
│  │  │  Web UI     │  │  TCC Client │  │  Matter Bridge  │   │  │
│  │  │  (Vue/Bulma)│  │  (REST API) │  │  (WS Client)    │   │  │
│  │  └─────────────┘  └─────────────┘  └─────────────────┘   │  │
│  └──────────────────────────────────────────────────────────┘  │
│                             │ HTTP/WebSocket                    │
│  ┌──────────────────────────┴───────────────────────────────┐  │
│  │              Node.js Matter.js Service (:5540)            │  │
│  │  ┌─────────────────────────────────────────────────────┐ │  │
│  │  │  Thermostat Device (Matter Thermostat Cluster)      │ │  │
│  │  └─────────────────────────────────────────────────────┘ │  │
│  └──────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

## Requirements

- Raspberry Pi 4 or newer (ARM64)
- Node.js 18+
- Go 1.21+
- Network access to TCC cloud service

## Quick Start

### 1. Install Dependencies

```bash
# Install Node.js and Go (on Raspberry Pi)
./scripts/install.sh

# Or manually install dependencies
make install
```

### 2. Build

```bash
make build
```

### 3. Run

```bash
# Development mode
make dev

# Or run directly
./bin/tcc-bridge
```

### 4. Configure

1. Open `http://localhost:8080` in your browser
2. Go to Configuration and enter your TCC credentials
3. Go to Pairing and scan the QR code with your iPhone

## Configuration

The service stores data in `~/.tcc-bridge/`:

- `tcc-bridge.db` - SQLite database
- `encryption.key` - Encryption key for stored credentials

### Environment Variables

- `SERVER_PORT` - HTTP server port (default: 8080)
- `MATTER_PORT` - Matter protocol port (default: 5540)
- `TCC_POLL_INTERVAL` - Polling interval in seconds (default: 600)

## Project Structure

```
mitsubishi/
├── cmd/server/          # Go entry point
├── internal/
│   ├── config/          # Configuration
│   ├── tcc/             # TCC API client
│   ├── matter/          # Matter bridge client
│   ├── storage/         # SQLite storage
│   ├── web/             # HTTP/WebSocket server
│   └── log/             # Logging
├── matter-bridge/       # Node.js Matter service
├── web/                 # Vue 3 frontend
├── configs/             # Systemd service files
└── scripts/             # Installation scripts
```

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/status` | GET | System status |
| `/api/thermostat` | GET | Thermostat state |
| `/api/thermostat/setpoint` | POST | Set temperature |
| `/api/thermostat/mode` | POST | Set mode |
| `/api/config` | GET | Configuration status |
| `/api/config/credentials` | POST | Save TCC credentials |
| `/api/pairing` | GET | Matter pairing info |
| `/api/logs` | GET | Event logs |
| `/api/ws` | WS | WebSocket for live updates |

## Deployment

### Docker (Recommended)

Build and push to your registry:

```bash
# Build Docker image
make docker-build

# Push to registry
make docker-push
```

Run with Docker Compose:

```bash
# Start services
make docker-up

# View logs
make docker-logs

# Stop services
make docker-down
```

Or run directly:

```bash
docker run -d \
  --name tcc-bridge \
  -p 8080:8080 \
  -p 5540:5540 \
  -v tcc-data:/app/data \
  --restart unless-stopped \
  registry.gstephens.org/tcc-bridge:latest
```

### Systemd Service

```bash
# Install service
make install-service

# Start service
sudo systemctl start tcc-bridge

# View logs
sudo journalctl -u tcc-bridge -f
```

## Troubleshooting

### TCC Connection Issues

- Verify credentials at mytotalconnectcomfort.com
- Check network connectivity
- TCC rate limits requests; wait 10 minutes between polls

### HomeKit Pairing Issues

- Ensure iPhone and Pi are on the same network
- Reset pairing: delete `matter-bridge/data/` and restart

### Service Won't Start

- Check logs: `sudo journalctl -u tcc-bridge -e`
- Verify Node.js installed: `node --version`
- Verify permissions on data directory

## License

MIT

## Acknowledgments

- [matter.js](https://github.com/project-chip/matter.js) - Matter protocol implementation
- [pyhtcc](https://github.com/csm10495/pyhtcc) - TCC API reference

## ToDo

- Log should show connection requests and responses from TCC including the "Test Connection" button request
- Pairing page shows "Loading QR code..." but then never loads
- The Status page is blank. It should at least show if we haven't configured the connection yet. If configured it should show the basic thermostat information

### Latest

- When I clicked Off it took a while to work. There are no messages in the UI Logs page and here's the -debug

2026-01-26 17:07:17 [DEBUG] TCC endpoint /portal/Device/GetZoneListData redirected to error/login
2026-01-26 17:07:17 [DEBUG] Trying to fetch known device ID 2246437
2026-01-26 17:07:17 [DEBUG] Fetching device data for device 2246437 from /portal/Device/CheckDataSession/2246437
2026-01-26 17:07:17 [DEBUG] WebSocket client connected (1 total)
2026-01-26 17:07:17 [DEBUG] Device data response (status 200, url https://mytotalconnectcomfort.com/portal/Device/CheckDataSession/2246437): {"success":true,"deviceLive":true,"communicationLost":false,"latestData":{"uiData":{"DispTemperature":67.0000,"HeatSetpoint":70.0,"CoolSetpoint":73.0,"DisplayUnits":"F","StatusHeat":0,"StatusCool":0,"HoldUntilCapable":true,"ScheduleCapable":true,"VacationHold":0,"DualSetpointStatus":false,"HeatNextPeriod":40,"CoolNextPeriod":40,"HeatLowerSetptLimit":50.0000,"HeatUpperSetptLimit":83.0000,"CoolLowerSetptLimit":67.0000,"CoolUpperSetptLimit":87.0000,"ScheduleHeatSp":70.0000,"ScheduleCoolSp":73.0000,...
2026-01-26 17:07:17 [DEBUG] Invalid humidity value 128 from TCC, capping at 100%
2026-01-26 17:07:17 [DEBUG] Successfully fetched device data: temp=67.0°F, heat=70.0, cool=73.0, mode=heat
2026-01-26 17:07:17 [DEBUG] Successfully fetched device 2246437
2026-01-26 17:07:17 [DEBUG] [matter-bridge] 2026-01-26 17:07:17.526 DEBUG  Transaction          Transaction set<tcc-matter-bridge.thermostat>#18 locked tcc-matter-bridge.thermostat.thermostat.state
2026-01-26 17:07:17 [WARN] [matter-bridge] 2026-01-26 17:07:17.526 ERROR  Transaction          State has not settled after 5 pre-commit cycles which likely indicates an infinite loop
2026-01-26 17:07:17 [DEBUG] [matter-bridge] 2026-01-26 17:07:17.526 DEBUG  Transaction          Transaction set<tcc-matter-bridge.thermostat>#18 rolled back and unlocked 1 resource
2026-01-26 17:07:17 [WARN] [matter-bridge] Failed to update thermostat state: Rolled back due to pre-commit error
2026-01-26 17:07:17 [WARN] [matter-bridge]       at errorRollback (file:///Users/greg/Dev/go/mitsubishi/matter-bridge/node_modules/@matter/node/dist/esm/behavior/state/transaction/Tx.js:325:13)
2026-01-26 17:07:17 [WARN] [matter-bridge]       at nextCycle (file:///Users/greg/Dev/go/mitsubishi/matter-bridge/node_modules/@matter/node/dist/esm/behavior/state/transaction/Tx.js:333:16)
2026-01-26 17:07:17 [WARN] [matter-bridge]       at nextPreCommit (file:///Users/greg/Dev/go/mitsubishi/matter-bridge/node_modules/@matter/node/dist/esm/behavior/state/transaction/Tx.js:349:28)
2026-01-26 17:07:17 [WARN] [matter-bridge]       at #executePreCommit (file:///Users/greg/Dev/go/mitsubishi/matter-bridge/node_modules/@matter/node/dist/esm/behavior/state/transaction/Tx.js:376:12)
2026-01-26 17:07:17 [WARN] [matter-bridge]       at Tx.commit (file:///Users/greg/Dev/go/mitsubishi/matter-bridge/node_modules/@matter/node/dist/esm/behavior/state/transaction/Tx.js:240:42)
2026-01-26 17:07:17 [WARN] [matter-bridge]       at commitTransaction (file:///Users/greg/Dev/go/mitsubishi/matter-bridge/node_modules/@matter/node/dist/esm/behavior/state/transaction/Tx.js:33:23)
2026-01-26 17:07:17 [WARN] [matter-bridge]       at process.processTicksAndRejections (node:internal/process/task_queues:95:5)
2026-01-26 17:07:17 [WARN] [matter-bridge]       at async Endpoint.set (file:///Users/greg/Dev/go/mitsubishi/matter-bridge/node_modules/@matter/node/dist/esm/endpoint/Endpoint.js:146:5)
2026-01-26 17:07:17 [WARN] [matter-bridge]       at async ThermostatEndpoint.updateState (file:///Users/greg/Dev/go/mitsubishi/matter-bridge/dist/thermostat.js:106:13)
2026-01-26 17:07:17 [WARN] [matter-bridge]       at async BridgeServer.stateHandler (file:///Users/greg/Dev/go/mitsubishi/matter-bridge/dist/index.js:21:13)
2026-01-26 17:07:17 [DEBUG] Polled 1 devices from TCC
2026-01-26 17:07:31 [DEBUG] [matter-bridge] 2026-01-26 17:07:31.024 DEBUG  MdnsBroadcaster      Announcement Generator: Commission mode mode: 1 qname: E7064C421FF73CA7._matterc._udp.local port: 5540 interface: lo0
2026-01-26 17:07:31 [DEBUG] [matter-bridge] 2026-01-26 17:07:31.153 DEBUG  MdnsScanner          Adding operational device DEDC14BEB86EC5E5-000000005D15A967._matter._tcp.local in cache (interface en26, ttl=4500) with TXT data: SII: 320 SAI: 300 SAT: 4000 T: 0 DT: undefined PH: undefined ICD: 0 VP: undefined DN: undefined RI: undefined PI: undefined
2026-01-26 17:07:31 [DEBUG] [matter-bridge] 2026-01-26 17:07:31.185 DEBUG  MdnsScanner          Adding operational device DEDC14BEB86EC5E5-0000000004A565F9._matter._tcp.local in cache (interface en26, ttl=4500) with TXT data: SII: 320 SAI: 300 SAT: 4000 T: 0 DT: undefined PH: undefined ICD: 0 VP: undefined DN: undefined RI: undefined PI: undefined
2026-01-26 17:07:31 [DEBUG] [matter-bridge] 2026-01-26 17:07:31.186 DEBUG  MdnsScanner          Adding operational device DEDC14BEB86EC5E5-0000000052B854A1._matter._tcp.local in cache (interface en26, ttl=4500) with TXT data: SII: 320 SAI: 300 SAT: 4000 T: 0 DT: undefined PH: undefined ICD: 0 VP: undefined DN: undefined RI: undefined PI: undefined
2026-01-26 17:07:31 [DEBUG] [matter-bridge] 2026-01-26 17:07:31.212 DEBUG  MdnsScanner          Adding operational device DEDC14BEB86EC5E5-0000000096642E4C._matter._tcp.local in cache (interface en26, ttl=4500) with TXT data: SII: 320 SAI: 300 SAT: 4000 T: 0 DT: undefined PH: undefined ICD: 0 VP: undefined DN: undefined RI: undefined PI: undefined
2026-01-26 17:07:31 [DEBUG] [matter-bridge] 2026-01-26 17:07:31.212 DEBUG  MdnsScanner          Adding operational device DEDC14BEB86EC5E5-0000000056E5840E._matter._tcp.local in cache (interface en26, ttl=4500) with TXT data: SII: 320 SAI: 300 SAT: 4000 T: 0 DT: undefined PH: undefined ICD: 0 VP: undefined DN: undefined RI: undefined PI: undefined
2026-01-26 17:07:31 [DEBUG] [matter-bridge] 2026-01-26 17:07:31.213 DEBUG  MdnsScanner          Adding operational device DEDC14BEB86EC5E5-000000002B67AC96._matter._tcp.local in cache (interface en26, ttl=4500) with TXT data: SII: 320 SAI: 300 SAT: 4000 T: 0 DT: undefined PH: undefined ICD: 0 VP: undefined DN: undefined RI: undefined PI: undefined
2026-01-26 17:07:36 [DEBUG] WebSocket client disconnected (0 total)
2026-01-26 17:07:37 [DEBUG] WebSocket client connected (1 total)
2026-01-26 17:08:15 [DEBUG] [matter-bridge] 2026-01-26 17:08:15.550 INFO   UdpMulticastServer   utun3: send EMSGSIZE ff02::fb:5353
2026-01-26 17:08:17 [DEBUG] [matter-bridge] 2026-01-26 17:08:17.650 DEBUG  MdnsScanner          Adding operational device DEDC14BEB86EC5E5-00000000B415B65E._matter._tcp.local in cache (interface en26, ttl=4500) with TXT data: SII: 320 SAI: 300 SAT: 4000 T: 0 DT: undefined PH: undefined ICD: 0 VP: undefined DN: undefined RI: undefined PI: undefined

- The heat setpoint seems to be out of sync. It got set to 70 somehow from 67. Apple Home shows it's 68 when it is 70.

- When the bridge loses network access, Apple Home shows "No Response" for the Thermostat. When network access is restored, will Apple Home eventually see that the bridge service is back?
- Why does the status page show HomeKit and TCC Connection as "connected" when there is a loss of network connectivity for the system running the service

- When the service has been off line for a while, Apple Home shows "No Response" for the Thermostat. When the service is restarted, Apple Home shows it available again but the Status screen shows HomeKit "Disconnected".  It also shows "Not paired". Why might shutting it down and restarting it cause the service to think HomeKit is now not paired

- The current temperature shows 68 in HomeKit but 64 on the web page and 64 on the thermostat. When the service is started or it gets information from TCC about the thermostat state, does it send the current temperature and other thermostat values via matter to homekit so it has the latest data?

