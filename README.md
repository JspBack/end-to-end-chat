# end-to-end-chat
Basic implementation of end to end encryption


# main logic
- first launch generates 2 keys, secret and public, public on ram secret on host. +
- secret key stored on the host and public is shared to peer. +
- first connection request happens, then keys exchanged.
- send messages are encrypted via public key that shared from peers then decrypted on peer.
- Peers are storing messages, they use secret key.
- Once the client shut down
- other instance can connect via this ram and using it's ip.
- they can message and that message is encrypted via secret key.
