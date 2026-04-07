# zone.com

Experimental Go server for the Windows XP / Millennium Zone.com checkers client.

The project implements enough of the original Zone/Millennium protocol stack for two legacy clients to:

- connect through the encrypted Zone transport handshake
- complete proxy negotiation
- enter the lobby/game room flow
- match against each other automatically
- start a checkers game
- exchange checkers game messages and chat
- handle opponent disconnects through the XP client's expected proxy disconnect flow

## Project layout

- [`cmd/zoneserver/main.go`](/home/gabriels/projetos/zone.com/cmd/zoneserver/main.go): server entry point
- [`internal/wire`](/home/gabriels/projetos/zone.com/internal/wire): low-level Zone transport framing, encryption, checksums, keepalives
- [`internal/conn`](/home/gabriels/projetos/zone.com/internal/conn): post-handshake connection wrapper
- [`internal/proto`](/home/gabriels/projetos/zone.com/internal/proto): protocol constants and message marshaling
- [`internal/proxy`](/home/gabriels/projetos/zone.com/internal/proxy): Millennium proxy negotiation
- [`internal/room`](/home/gabriels/projetos/zone.com/internal/room): lobby/game routing, matchmaking, table management
- [`internal/checkers`](/home/gabriels/projetos/zone.com/internal/checkers): server-side checkers rules and board state
- [`internal/server`](/home/gabriels/projetos/zone.com/internal/server): end-to-end connection flow

## Connection flow

Each client connection goes through these phases:

1. Connection handshake
   - reads the Zone `Hi`
   - negotiates options
   - sends `Hello`

2. Proxy negotiation
   - parses packed XP proxy startup messages
   - sends packed proxy responses expected by the Millennium client
   - confirms the requested service/channel

3. Room bootstrap
   - reads `ClientConfig`
   - allocates a user
   - sends `ZUserIDResponse`
   - sends `ServerStatus`

4. Matchmaking
   - seats players automatically
   - starts a table when two players are present
   - sends `StartGameM`

5. Game loop
   - routes `RoomMsgGameMessage`
   - handles checkers startup, moves, turn completion, chat, draw/endgame traffic

## Current behavior

Implemented:

- XP/Millennium proxy startup compatibility
- packed proxy service info flow
- connection keepalive and ping handling
- automatic two-player matchmaking
- room/game startup for checkers
- server-side validation of checkers moves
- checkers chat relay
- proxy-based opponent disconnect handling

Current assumptions:

- only one game service is supported: the checkers Millennium service
- tables are auto-managed; there is no full Zone UI/lobby feature set
- this is focused on the 2-player XP client path, not the broader Zone ecosystem

## Running

Build:

```bash
go build -o zoneserver ./cmd/zoneserver
```

Run:

```bash
./zoneserver
```

By default the server listens on TCP port `28805`, which matches the Millennium proxy flow used by the XP client.

## Testing

Run all tests with:

```bash
env GOCACHE=/tmp/go-build-cache go test ./...
```

## Status

This is a compatibility-oriented reimplementation, not a full archival reproduction of the original Microsoft backend. The goal is practical interoperability with the legacy client, with protocol behavior driven by source inspection and live client testing.
