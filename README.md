# end-to-end-chat
Basic implementation of end to end encryption


# main logic
- first launch generates 2 keys, secret and public, public on ram secret on host. +
- secret key stored on the host and public is shared to peer. +
- first connection request happens if allowed, then keys exchanged. +
- p_key saved into known_keys table and status should be accepted. (like a firewall, on application level there should be p_key cache clean option.) +
- send messages are encrypted via public key that shared from peers then decrypted on peer. +
- Peers are storing messages and they use secret key.+
- Once the client shut down p_key rotates.+
- rate limit impl for network layer +

# test-case
- peer-1 to peer-2 +
- peer-1 to peer-2 while peer-1 to peer-3 +
- peer-1 to peer-2 while peer-1 to peer-3 while peer-2 to peer-3 +

# optimizations
