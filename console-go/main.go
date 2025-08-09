package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	logging "github.com/ipfs/go-log"
	libp2p "github.com/libp2p/go-libp2p"
	kaddht "github.com/libp2p/go-libp2p-kad-dht"
	crypto "github.com/libp2p/go-libp2p/core/crypto"
	host "github.com/libp2p/go-libp2p/core/host"
	network "github.com/libp2p/go-libp2p/core/network"
	peer "github.com/libp2p/go-libp2p/core/peer"
	peerstore "github.com/libp2p/go-libp2p/core/peerstore"
	ma "github.com/multiformats/go-multiaddr"
)

const (
	protocolID      = "/p2pchat/1.0.0"
	dhtMsgKeyPrefix = "/p2pchat/messages/"
	identityFile    = "p2pchat_id.key"
)

var logger = logging.Logger("p2pchat")

type Message struct {
	From string `json:"from"`
	When int64  `json:"when"`
	Body string `json:"body"`
}

func main() {
	logging.SetLogLevel("p2pchat", "info")

	ctx := context.Background()

	priv, err := loadOrCreateIdentity(identityFile)
	if err != nil {
		fmt.Println("failed to load/create identity:", err)
		return
	}

	// Create a libp2p host
	h, err := libp2p.New(
		libp2p.Identity(priv),
	)
	if err != nil {
		fmt.Println("failed to create libp2p host:", err)
		return
	}

	defer h.Close()

	fmt.Println("Started host:")
	fmt.Println("  Peer ID:", h.ID().String())
	addrs := h.Addrs()
	for _, a := range addrs {
		fmt.Println("  -", a)
	}

	// Setup DHT
	dht, err := kaddht.New(ctx, h)
	if err != nil {
		fmt.Println("failed to create DHT:", err)
		return
	}
	// Bootstrap the DHT (no external bootstrap nodes used for strict invite-only P2P)
	if err := dht.Bootstrap(ctx); err != nil {
		fmt.Println("warning: dht bootstrap error:", err)
	}

	// Handle incoming streams
	h.SetStreamHandler(protocolID, func(s network.Stream) {
		defer s.Close()
		peerAddr := s.Conn().RemotePeer().String()
		r := bufio.NewReader(s)
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					fmt.Println("stream read err:", err)
				}
				return
			}
			line = strings.TrimSpace(line)
			var m Message
			if err := json.Unmarshal([]byte(line), &m); err != nil {
				fmt.Println("invalid message from", peerAddr, "raw:", line)
				continue
			}
			fmt.Printf("\n<msg from=%s when=%s> %s\n> ", m.From, time.UnixMilli(m.When).Format(time.RFC3339), m.Body)
		}
	})

	// CLI loop
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Type 'help' for commands.")
	for {
		fmt.Printf("> ")
		text, _ := reader.ReadString('\n')
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		parts := strings.SplitN(text, " ", 3)
		switch parts[0] {
		case "help":
			printHelp()
		case "peers":
			listPeers(h)
		case "invite":
			printInvite(h)
		case "connect":
			if len(parts) < 2 {
				fmt.Println("usage: connect <multiaddr>")
				continue
			}
			if err := connectPeer(ctx, h, parts[1]); err != nil {
				fmt.Println("connect error:", err)
			}
		case "msg":
			if len(parts) < 3 {
				fmt.Println("usage: msg <peerID> <message>")
				continue
			}
			target := parts[1]
			body := parts[2]
			if err := sendMessage(ctx, h, target, body); err != nil {
				fmt.Println("send error:", err)
			}
		case "store":
			if len(parts) < 3 {
				fmt.Println("usage: store <peerID> <text>")
				continue
			}
			target := parts[1]
			body := parts[2]
			if err := storeOfflineMessage(ctx, dht, target, h.ID().String(), body); err != nil {
				fmt.Println("store error:", err)
			}
		case "fetch":
			if len(parts) < 2 {
				fmt.Println("usage: fetch <peerID>")
				continue
			}
			if err := fetchOfflineMessages(ctx, dht, parts[1]); err != nil {
				fmt.Println("fetch error:", err)
			}
		case "id":
			fmt.Println(h.ID().String())
		case "quit", "exit":
			fmt.Println("bye")
			return
		default:
			fmt.Println("unknown command. type 'help'")
		}
	}
}

func printHelp() {
	fmt.Println("commands:")
	fmt.Println("  peers                  - list connected peers")
	fmt.Println("  invite                 - print invite multiaddr")
	fmt.Println("  connect <multiaddr>    - connect to a peer using their invite string")
	fmt.Println("  msg <peerID> <message> - send immediate message to peer (if online)")
	fmt.Println("  store <peerID> <text>  - append message to recipient's DHT inbox (offline delivery)")
	fmt.Println("  fetch <peerID>         - fetch stored messages for peerID from DHT")
	fmt.Println("  id                     - print your peer id")
	fmt.Println("  help                   - help")
	fmt.Println("  quit                   - exit")
}

func loadOrCreateIdentity(path string) (crypto.PrivKey, error) {
	// If key exists, load it. Otherwise create and save.
	if _, err := os.Stat(path); err == nil {
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		priv, err := crypto.UnmarshalPrivateKey(b)
		if err != nil {
			return nil, err
		}
		return priv, nil
	}

	// generate ed25519 keypair
	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.Ed25519, -1, rand.Reader)
	if err != nil {
		return nil, err
	}
	b, err := crypto.MarshalPrivateKey(priv)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, b, 0600); err != nil {
		return nil, err
	}
	return priv, nil
}

func printInvite(h host.Host) {
	id := h.ID().String()
	addrs := h.Addrs()
	// choose the first address + /p2p/<peerid>
	if len(addrs) == 0 {
		fmt.Println("no listen addresses available. try running with an explicit listen addr or open firewall/port")
		return
	}
	for _, a := range addrs {
		fmt.Printf("%s/p2p/%s\n", a.String(), id)
	}
	fmt.Println("Share one of the lines above with peers as an invite. They can 'connect <that-line>'.")
}

func listPeers(h host.Host) {
	peers := h.Network().Peers()
	if len(peers) == 0 {
		fmt.Println("no connected peers")
		return
	}
	fmt.Println("connected peers:")
	for _, p := range peers {
		fmt.Println(" -", p.String())
	}
}

func connectPeer(ctx context.Context, h host.Host, addrStr string) error {
	maddr, err := ma.NewMultiaddr(addrStr)
	if err != nil {
		// support the case where user passes full multiaddr that includes /p2p/<peerid>
		// try to parse and connect
		if strings.Contains(addrStr, "/p2p/") {
			maddr, err = ma.NewMultiaddr(addrStr)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}
	pi, err := peer.AddrInfoFromP2pAddr(maddr)
	if err != nil {
		return err
	}
	h.Peerstore().AddAddrs(pi.ID, pi.Addrs, peerstore.PermanentAddrTTL)
	if err := h.Connect(ctx, *pi); err != nil {
		return err
	}
	fmt.Println("connected to", pi.ID.String())
	return nil
}

func sendMessage(ctx context.Context, h host.Host, peerIDStr string, body string) error {
	pid, err := peer.Decode(peerIDStr)
	if err != nil {
		return err
	}
	// open stream
	s, err := h.NewStream(ctx, pid, protocolID)
	if err != nil {
		return err
	}
	defer s.Close()
	m := Message{From: h.ID().String(), When: time.Now().UnixMilli(), Body: body}
	b, _ := json.Marshal(m)
	b = append(b, '\n')
	_, err = s.Write(b)
	if err != nil {
		return err
	}
	fmt.Println("sent")
	return nil
}

func storeOfflineMessage(ctx context.Context, dht *kaddht.IpfsDHT, recipientPeerID string, from string, body string) error {
	// Append message to DHT key: /p2pchat/messages/<recipientPeerID>
	key := dhtMsgKeyPrefix + recipientPeerID
	var msgs []Message
	val, err := dht.GetValue(ctx, key)
	if err == nil {
		// existing value
		_ = json.Unmarshal(val, &msgs)
	}
	msgs = append(msgs, Message{From: from, When: time.Now().UnixMilli(), Body: body})
	n, _ := json.Marshal(msgs)
	// Note: PutValue may be limited in size by network; large values won't replicate well.
	if err := dht.PutValue(ctx, key, n); err != nil {
		return err
	}
	fmt.Println("stored for offline delivery (in DHT key)")
	return nil
}

func fetchOfflineMessages(ctx context.Context, dht *kaddht.IpfsDHT, peerID string) error {
	key := dhtMsgKeyPrefix + peerID
	val, err := dht.GetValue(ctx, key)
	if err != nil {
		return fmt.Errorf("no messages or error: %w", err)
	}
	var msgs []Message
	if err := json.Unmarshal(val, &msgs); err != nil {
		return err
	}
	fmt.Printf("fetched %d messages:\n", len(msgs))
	for i, m := range msgs {
		fmt.Printf("%d) from=%s at=%s\n   %s\n", i+1, m.From, time.UnixMilli(m.When).Format(time.RFC3339), m.Body)
	}
	return nil
}
