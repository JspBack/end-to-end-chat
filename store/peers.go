package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/google/uuid"

	"github.com/JspBack/end-to-end-chat/keys"
)

type PeerStatus string

const (
	Pending  PeerStatus = "pending"
	Accepted PeerStatus = "accepted"
	Rejected PeerStatus = "rejected"
)

var NilStatus PeerStatus

func (s PeerStatus) String() string {
	return string(s)
}

type KnownPeer struct {
	PeerIP     net.IP     `json:"peer_ip"`
	PubKey     keys.Key   `json:"pub_key"`
	Name       string     `json:"name,omitempty"`
	Nickname   string     `json:"nickname,omitempty"`
	Status     PeerStatus `json:"status"`
	Online     bool       `json:"online"`
	LastSeen   time.Time  `json:"last_seen,omitzero"`
	ProfilePic uuid.UUID  `json:"profile_pic,omitempty"`
}

type KnownPeerStore struct {
	db *sql.DB
}

func (k *KnownPeerStore) Add(peer *KnownPeer) error {
	q := "INSERT OR REPLACE INTO known_peers (pub_key, peer_ip, name, nickname, status, last_seen, profile_pic) VALUES (?, ?, ?, ?, ?, ?, ?)"
	if _, err := k.db.ExecContext(context.Background(), q, peer.PubKey.String(), peer.PeerIP.String(), peer.Name, peer.Nickname,
		string(peer.Status), peer.LastSeen.String(), peer.ProfilePic.String()); err != nil {
		return fmt.Errorf("store: add known peer: %w", err)
	}
	return nil
}

func (k *KnownPeerStore) Get(pubKey keys.Key) (*KnownPeer, error) {
	var peer KnownPeer
	var peerIPStr, lastSeenStr string
	q := "SELECT pub_key, peer_ip, COALESCE(name, ''), COALESCE(nickname, ''), " +
		"status, COALESCE(last_seen, ''), COALESCE(profile_pic, '') FROM known_peers WHERE pub_key = ?"
	err := k.db.QueryRowContext(context.Background(), q, pubKey.String()).Scan(
		&peer.PubKey, &peerIPStr, &peer.Name, &peer.Nickname, &peer.Status, &lastSeenStr, &peer.ProfilePic)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, os.ErrNotExist
		}
		return nil, fmt.Errorf("store: get known peer: %w", err)
	}
	peer.PeerIP = net.ParseIP(peerIPStr)
	peer.LastSeen, _ = time.Parse(time.RFC3339, lastSeenStr)
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
		var peerIPStr, lastSeenStr string
		if err = rows.Scan(&peer.PubKey, &peerIPStr, &peer.Name, &peer.Nickname, &peer.Status, &lastSeenStr, &peer.ProfilePic); err != nil {
			return nil, fmt.Errorf("store: scan peer: %w", err)
		}
		peer.PeerIP = net.ParseIP(peerIPStr)
		peer.LastSeen, _ = time.Parse(time.RFC3339, lastSeenStr)
		peers = append(peers, peer)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("store: rows iteration: %w", err)
	}
	return peers, nil
}

func (k *KnownPeerStore) Remove(pubKey keys.Key) error {
	q := "DELETE FROM known_peers WHERE pub_key = ?"
	if _, err := k.db.ExecContext(context.Background(), q, pubKey.String()); err != nil {
		return fmt.Errorf("store: remove known peer: %w", err)
	}
	return nil
}
