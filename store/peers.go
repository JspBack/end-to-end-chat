package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
)

func (k *KnownPeerStore) Add(peer *KnownPeer) error {
	q := "INSERT OR REPLACE INTO known_peers (peer_ip, pub_key, status) VALUES (?, ?, ?)"
	if _, err := k.db.ExecContext(context.Background(), q, peer.PeerIP, peer.PubKey, peer.Status); err != nil {
		return fmt.Errorf("store: add known peer: %w", err)
	}
	return nil
}

func (k *KnownPeerStore) Get(peerIP string) (*KnownPeer, error) {
	var peer KnownPeer
	q := "SELECT peer_ip, pub_key, status FROM known_peers WHERE peer_ip = ?"
	if err := k.db.QueryRowContext(context.Background(), q, peerIP).Scan(&peer.PeerIP, &peer.PubKey, &peer.Status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, os.ErrNotExist
		}
		return nil, fmt.Errorf("store: get known peer: %w", err)
	}
	return &peer, nil
}

func (k *KnownPeerStore) Remove(peerIP string) error {
	q := "DELETE FROM known_peers WHERE peer_ip = ?"
	if _, err := k.db.ExecContext(context.Background(), q, peerIP); err != nil {
		return fmt.Errorf("store: remove known peer: %w", err)
	}
	return nil
}
