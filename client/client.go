package client

import (
	"encoding/json"
	"fmt"

	"github.com/JspBack/end-to-end-chat/keys"
	"github.com/JspBack/end-to-end-chat/store"
)

type Client struct {
	Name  string
	Keys  *keys.Keys
	Store *store.Store
}

func New(client, db, keyfile string) *Client {
	return &Client{
		Name:  client,
		Keys:  keys.Load(keyfile),
		Store: store.New(db),
	}
}

func (c *Client) Put(msg *Message) (string, error) {
	plain, err := json.Marshal(msg)
	if err != nil {
		return "", fmt.Errorf("client: marshal: %w", err)
	}
	id, err := c.Store.Chats.Put(string(plain), c.Keys.Private)
	if err != nil {
		return "", fmt.Errorf("client: store put: %w", err)
	}
	return id, nil
}

func (c *Client) Get(id string) (*Message, error) {
	plain, err := c.Store.Chats.Get(id, c.Keys.Private)
	if err != nil {
		return nil, fmt.Errorf("client: store get: %w", err)
	}
	var msg Message
	if err = json.Unmarshal([]byte(plain), &msg); err != nil {
		return nil, fmt.Errorf("client: unmarshal: %w", err)
	}
	return &msg, nil
}

func (c *Client) Update(id string, msg *Message) error {
	plain, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("client: marshal: %w", err)
	}
	if err = c.Store.Chats.Update(id, string(plain), c.Keys.Private); err != nil {
		return fmt.Errorf("client: store update: %w", err)
	}
	return nil
}

func (c *Client) Delete(id string) error {
	if err := c.Store.Chats.Delete(id); err != nil {
		return fmt.Errorf("client: store delete: %w", err)
	}
	return nil
}

func (c *Client) AddKnownPeer(peer *store.KnownPeer) error {
	if err := c.Store.KnownPeers.Add(peer); err != nil {
		return fmt.Errorf("client: add known peer: %w", err)
	}
	return nil
}

func (c *Client) GetKnownPeer(peerIP string) (*store.KnownPeer, error) {
	peer, err := c.Store.KnownPeers.Get(peerIP)
	if err != nil {
		return nil, fmt.Errorf("client: get known peer: %w", err)
	}
	return peer, nil
}

func (c *Client) RemoveKnownPeer(peerIP string) error {
	if err := c.Store.KnownPeers.Remove(peerIP); err != nil {
		return fmt.Errorf("client: remove known peer: %w", err)
	}
	return nil
}
