# zone.com

Experimental Go server for the Windows XP / Millennium Zone.com game clients.

The project implements enough of the original Zone/Millennium protocol stack for legacy clients to:

- connect through the encrypted Zone transport handshake
- complete proxy negotiation
- enter the lobby/game room flow
- match against each other automatically
- start a supported game session
- exchange game messages and chat
- handle opponent disconnects through the XP client's expected proxy disconnect flow

Currently supported game services:

- Checkers: `mchkr_zm_***`
- Reversi: `mrvse_zm_***`
- Backgammon: `mbckg_zm_***`

## Project layout

- [`cmd/zoneserver/main.go`](/home/gabriels/projetos/zone.com/cmd/zoneserver/main.go): server entry point
- [`internal/wire`](/home/gabriels/projetos/zone.com/internal/wire): low-level Zone transport framing, encryption, checksums, keepalives
- [`internal/conn`](/home/gabriels/projetos/zone.com/internal/conn): post-handshake connection wrapper
- [`internal/proto`](/home/gabriels/projetos/zone.com/internal/proto): protocol constants and message marshaling
- [`internal/proxy`](/home/gabriels/projetos/zone.com/internal/proxy): Millennium proxy negotiation
- [`internal/room`](/home/gabriels/projetos/zone.com/internal/room): lobby/game routing, matchmaking, table management
- [`internal/checkers`](/home/gabriels/projetos/zone.com/internal/checkers): server-side checkers rules and board state
- [`internal/reversi`](/home/gabriels/projetos/zone.com/internal/reversi): server-side Reversi rules and state serialization
- [`internal/backgammon`](/home/gabriels/projetos/zone.com/internal/backgammon): Backgammon shared state, dice encoding, piece positions
- [`internal/integration`](/home/gabriels/projetos/zone.com/internal/integration): black-box socket-level integration tests
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
   - resolves the requested service to a game definition

3. Room bootstrap
   - reads `ClientConfig`
   - allocates a user
   - sends `ZUserIDResponse`
   - sends room `Enter` notifications so clients can resolve opponent player info
   - sends `ServerStatus`

4. Matchmaking
   - seats players automatically
   - groups players by requested game service
   - starts a table when two players for the same game are present
   - sends `StartGameM`

5. Game loop
   - routes `RoomMsgGameMessage`
   - dispatches to the selected game session
   - handles startup, moves, chat, endgame, rematch, and state-sync traffic

## Current behavior

Implemented:

- XP/Millennium proxy startup compatibility
- packed proxy service info flow
- connection keepalive and ping handling
- automatic two-player matchmaking
- room/game startup for supported game services
- server-side validation of checkers moves
- server-side validation and state sync for Reversi
- Backgammon session with MSVC-aligned wire protocol (DICEINFO padding), server-side dice rolling, shared state management, and transaction relay
- chat relay
- proxy-based opponent disconnect handling
- rematch handling for Reversi, including ready-state notifications
- suppression of protocol paths that the XP clients treat as corruption (`EndLog` relay, legacy Reversi `FinishMove`, Backgammon `NewMatch`/`TieRoll`/etc.)

Current assumptions:

- tables are auto-managed; there is no full Zone UI/lobby feature set
- this is focused on the 2-player XP client path, not the broader Zone ecosystem
- Checkers, Reversi, and Backgammon are implemented; other Millennium services are not

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
