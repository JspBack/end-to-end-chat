package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"

	"github.com/google/uuid"
)

const (
	PeerStatusPending  = "pending"
	PeerStatusAccepted = "accepted"
	PeerStatusRejected = "rejected"
)

type KnownPeer struct {
	PeerIP     string `json:"peer_ip"`
	PubKey     string `json:"pub_key"`
	Name       string `json:"name,omitempty"`
	Nickname   string `json:"nickname,omitempty"`
	Status     string `json:"status"`
	Online     bool   `json:"online"`
	LastSeen   string `json:"last_seen,omitempty"`
	ProfilePic string `json:"profile_pic,omitempty"`
}

type KnownPeerStore struct {
	db *sql.DB
}

func (k *KnownPeerStore) Add(peer *KnownPeer) error {
	q := "INSERT OR REPLACE INTO known_peers (pub_key, peer_ip, name, nickname, status, last_seen, profile_pic) VALUES (?, ?, ?, ?, ?, ?, ?)"
	if _, err := k.db.ExecContext(context.Background(), q, peer.PubKey, peer.PeerIP, peer.Name, peer.Nickname,
		peer.Status, peer.LastSeen, peer.ProfilePic); err != nil {
		return fmt.Errorf("store: add known peer: %w", err)
	}
	return nil
}

func (k *KnownPeerStore) Get(pubKey string) (*KnownPeer, error) {
	var peer KnownPeer
	q := "SELECT pub_key, peer_ip, COALESCE(name, ''), COALESCE(nickname, ''), " +
		"status, COALESCE(last_seen, ''), COALESCE(profile_pic, '') FROM known_peers WHERE pub_key = ?"
	err := k.db.QueryRowContext(context.Background(), q, pubKey).Scan(
		&peer.PubKey, &peer.PeerIP, &peer.Name, &peer.Nickname, &peer.Status, &peer.LastSeen, &peer.ProfilePic)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, os.ErrNotExist
		}
		return nil, fmt.Errorf("store: get known peer: %w", err)
	}
	return &peer, nil
}

func (k *KnownPeerStore) List() ([]KnownPeer, error) {
	q := "SELECT pub_key, peer_ip, COALESCE(name, ''), COALESCE(nickname, ''), " +
		"status, COALESCE(last_seen, ''), COALESCE(profile_pic, '') FROM known_peers"
	rows, err := k.db.QueryContext(context.Background(), q)
	if err != nil {
		return nil, fmt.Errorf("store: list known peers: %w", err)
	}
	defer rows.Close()

	var peers []KnownPeer
	for rows.Next() {
		var peer KnownPeer
		if err = rows.Scan(&peer.PubKey, &peer.PeerIP, &peer.Name, &peer.Nickname, &peer.Status, &peer.LastSeen, &peer.ProfilePic); err != nil {
			return nil, fmt.Errorf("store: scan peer: %w", err)
		}
		peers = append(peers, peer)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("store: rows iteration: %w", err)
	}
	return peers, nil
}

func (k *KnownPeerStore) SetName(pubKey, name string) error {
	q := "UPDATE known_peers SET name = ? WHERE pub_key = ?"
	if _, err := k.db.ExecContext(context.Background(), q, name, pubKey); err != nil {
		return fmt.Errorf("store: set peer name: %w", err)
	}
	return nil
}

func (k *KnownPeerStore) SetProfilePic(pubKey string, fileID uuid.UUID) error {
	idStr := ""
	if fileID != uuid.Nil {
		idStr = fileID.String()
	}
	q := "UPDATE known_peers SET profile_pic = ? WHERE pub_key = ?"
	if _, err := k.db.ExecContext(context.Background(), q, idStr, pubKey); err != nil {
		return fmt.Errorf("store: set profile pic: %w", err)
	}
	return nil
}

func (k *KnownPeerStore) SetNickname(pubKey, nickname string) error {
	q := "UPDATE known_peers SET nickname = ? WHERE pub_key = ?"
	if _, err := k.db.ExecContext(context.Background(), q, nickname, pubKey); err != nil {
		return fmt.Errorf("store: set peer nickname: %w", err)
	}
	return nil
}

func (k *KnownPeerStore) SetLastSeen(pubKey, lastSeen string) error {
	q := "UPDATE known_peers SET last_seen = ? WHERE pub_key = ?"
	if _, err := k.db.ExecContext(context.Background(), q, lastSeen, pubKey); err != nil {
		return fmt.Errorf("store: set last_seen: %w", err)
	}
	return nil
}

func (k *KnownPeerStore) Remove(pubKey string) error {
	q := "DELETE FROM known_peers WHERE pub_key = ?"
	if _, err := k.db.ExecContext(context.Background(), q, pubKey); err != nil {
		return fmt.Errorf("store: remove known peer: %w", err)
	}
	return nil
}
