# end-to-end-chat

Very basic end-to-end chat implementation :)

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

Without `-addr`, write mode listens for inbound connections and broadcasts stdin to all currently connected peers. Use `-addr` to also make an outbound connection:

```sh
./bin_client/client -w -addr localhost:8080
```

## API

| Method | Endpoint | Description |
|---|---|---|
| `GET` | `/api/messages` | List all messages (id + timestamp) |
| `GET` | `/api/messages/search?q=` | Search messages by content, from, to, or attachment name (case-insensitive) |
| `GET` | `/api/messages/{id}` | Get a single message |
| `POST` | `/api/messages/{pubKey}` | Send a message to a connected peer |
| `PUT` | `/api/messages/{id}` | Update your own message (`{"content":"..."}`) |
| `DELETE` | `/api/messages/{id}` | Delete your own message |
| `GET` | `/api/files/{id}` | Download a file attachment (raw bytes) |
| `POST` | `/api/peers/connect` | Connect to a peer (`{"addr":"host:port"}`) |
| `GET` | `/admin/peers` | List peers |
| `PUT` | `/admin/peers/{pubKey}/accept` | Accept peer |
| `PUT` | `/admin/peers/{pubKey}/reject` | Reject peer |
| `GET` | `/admin/sessions` | List active sessions |

All API endpoints are localhost-only.

### Protocol

All peer-to-peer communication uses a typed envelope. Attachment data is encoded as base64 within the `message` envelope by Go's json.Marshal (`[]byte` marshals to base64):

| Type | Purpose |
|---|---|
| `message` | Chat message payload (from, to, content, time, id, attachments[]) |
| `delete` | Delete a message by id (only owner) |
| `update` | Update a message's content by id (only owner) |

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
| `-max-msg-size` | `50MB` | Maximum message size in bytes |
| `-ping-window` | `5s` | Ping window duration |
| `-cert` | `""` | TLS certificate file path |
| `-key` | `""` | TLS private key file path |

## Network limitations

This project does not use a relay server, STUN/TURN, or any type of middle connection. Peers connect directly to each other using raw TCP over the internet.

Because of this, peer-to-peer communication will not work across many real-world network configurations:

- **NAT** – Most home/office routers hide devices behind a single public IP. Direct inbound connections are blocked unless port forwarding is configured.
- **CGNAT** – Mobile and some residential ISPs place customers behind shared IPs, making port forwarding impossible.
- **Firewalls** – Many networks block unsolicited inbound traffic on non-standard ports.

### Workaround

You can bridge peers across the internet by using a virtual private network that gives each device a routable LAN address. One popular option is [ZeroTier](https://github.com/zerotier/zerotierone): it creates a secure software-defined network so peers appear to be on the same local subnet even when they are behind NAT or CGNAT.

## Clean

```sh
make clean
```
