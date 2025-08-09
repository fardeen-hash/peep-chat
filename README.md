# 🗨️ p2p-chat — Proof of Concept (POC)

A simple command-line **peer-to-peer chat** using [go-libp2p](https://github.com/libp2p/go-libp2p).

This is a single-file POC to demonstrate decentralized messaging with libp2p, including DHT-based offline message storage.

---

## ✨ Features

- ✅ CLI app that creates a libp2p host (identity is persisted to disk)
- 🔑 Shows your own *"invite"* multiaddrs to share with peers
- 🔌 Connect to other peers using their multiaddr
- 📩 Send encrypted 1:1 messages via libp2p secure streams
- 🗃️ Store offline messages in the DHT under a per-peer key (append-only)

---

## ⚠️ Limitations (Proof-of-Concept)

> This POC is a minimal working example. Important limitations:

- ❌ **No end-to-end encryption** for DHT-stored messages  
  _(Recommended: X25519 + XSalsa20-Poly1305 via libsodium)_
- ⚠️ **DHT is not a reliable long-term storage** — entries can be dropped or overwritten
- 🌐 **No relays included** — NAT traversal depends on your network; relays must be added manually

---

## 🚀 Getting Started

### 🧱 Prerequisites
- Go 1.20 or newer  
- Internet access to fetch modules

---

### 🔧 Build & Run

```bash
# 1. Set up Go modules
go mod init p2p-chat
go get

# 2. Build the binary
go build -o p2p-chat main.go

# 3. Run it
# On Windows:
p2p-chat.exe

# On Linux/macOS:
./p2p-chat
```

###  Commands (interactive)
```bash
  peers                  - list connected peers
  invite                 - print a copy-paste invite multiaddr
  connect <multiaddr>    - connect to a peer using their invite string
  msg <peerID> <message> - send an immediate message to peer (if online)
  store <peerID> <text>  - append a message to recipient's DHT inbox (offline delivery)
  fetch <peerID>         - fetch stored messages for peerID from DHT (you should run for your own peerID)
  id                     - prints your peer ID
  help                   - this help
  quit                   - exit
```

NOTES:

p2p-chat (single-file POC)
==========================
This is a proof-of-concept CLI peer-to-peer chat using go-libp2p.
Features included in this POC:
- CLI app that creates a libp2p host (identity saved to disk)
- Prints your "invite" multiaddrs (copy & paste to other peers)
- Connect to another peer via multiaddr
- Send direct 1:1 messages over secured libp2p streams (encrypted by transport)
- Store "offline" messages in the DHT under a per-recipient key (append-only list)

Important limitations (POC):
- Offline/DHT stored messages in this POC are NOT end-to-end encrypted. You must add payload
  encryption (e.g., X25519 ECDH + xsalsa20-poly1305 / libsodium) for real privacy.
- DHT is not a reliable long-term store. This POC appends to a DHT value (size limits apply).
- NAT traversal and relay behavior depends on network; this POC does not run its own relay nodes.



NOTE FOR ROOKIE USERS:

- To chat, run the binary on two machines (or two terminals on the same machine with different ports).
- On machine A type `invite` and copy the printed multiaddr string into machine B using `connect <addr>`.
- On machine B run `msg <peerID-of-A> Hello` to send a message to A (if A is online).
- Use `store` to place an encrypted message into the DHT for offline retrieval (recipient runs `fetch`).

TODO (next steps I can implement on request):
- End-to-end payload encryption for DHT-stored messages (recommended)
- Better indexing for multiple messages instead of rewriting one large DHT value
- Integrated relay discovery and auto-relay selection for NATed peers


