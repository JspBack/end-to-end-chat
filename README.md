# end-to-end-chat

Very basic end-to-end chat implementation. :)
On first run, the client generates a key pair (public + private). The private key is used to derive the database name and access token, so your messages persist across restarts.

## Build

```sh
make build
```

## Quick start

**Terminal 1 — start the peer-1:**

```sh
./bin_client/client -client peer-1
```

**Terminal 2 — start a peer that connects to the peer-2:**

```sh
./bin_peer/peer -p 8081 -client peer-2
```

**Terminal 3 — start the web UI pointing at the peer-1:**

```sh
./bin_ui/ui -p 8082
```

**Terminal 4 — start the web UI pointing at the peer-2:**

```sh
./bin_ui/ui -p 8083 -t localhost:8081
```

**Or accept via curl:**

```sh
curl -X PUT localhost:8080/admin/peers/<pub_key>/accept
```

Replace `<pub_key>` with the hex key shown in the client log.

Message content is logged at `debug` level. To see it in the terminal, add `-l debug`:

```sh
./bin_client/client -l debug
./bin_peer/peer -l debug -addr localhost:8080 -p 8081
```

## Write mode

Start the client in write mode to type messages from stdin:

```sh
./bin_client/client -w
```

## API

| Endpoint | Description |
|---|---|
| `GET /api/messages` | List all messages (id + timestamp) |
| `GET /api/messages/{id}` | Get a single message |
| `POST /api/messages/{pubKey}` | Send a message to a connected peer |
| `POST /api/peers/connect` | Connect to a peer (`{"addr":"host:port"}`) |
| `GET /admin/peers` | List peers |
| `PUT /admin/peers/{pubKey}/accept` | Accept peer |
| `PUT /admin/peers/{pubKey}/reject` | Reject peer |
| `GET /admin/sessions` | List active sessions |

All API endpoints are localhost-only.

## Flags

| Flag | Default | Description |
|---|---|---|
| `-client` | `default` | Client name |
| `-p` | `8080` | Port to listen on |
| `-addr` | `""` | Peer address to connect to (`host:port`) |
| `-w` | `false` | Read stdin and broadcast to connected peers |
| `-l` | `info` | Log level (`debug`, `info`, `warn`, `error`) |
| `-t` | `15s` | Timeout for operations |
| `-rate-limit` | `100` | HTTP requests per window per IP |
| `-rate-window` | `1m` | Rate limiter window duration |
| `-max-msg-size` | `1MB` | Maximum message size in bytes |
| `-ping-window` | `5s` | Ping window duration |
| `-cert` | `""` | TLS certificate file path |
| `-key` | `""` | TLS private key file path |

## Clean

```sh
make clean
```
