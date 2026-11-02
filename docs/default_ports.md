While bchd is highly configurable when it comes to the network configuration,
the following is intended to be a quick reference for the default ports used so
port forwarding can be configured as required.

bchd provides a `--upnp` flag which can be used to automatically map the Bitcoin Cash
peer-to-peer listening port if your router supports UPnP. If your router does
not support UPnP, or you don't wish to use it, please note that only the
peer-to-peer port should be forwarded unless you specifically want to allow RPC
access to your bchd from external sources such as in more advanced network
configurations.

| Network  | Peer-to-peer | RPC       | gRPC      |
| -------- | ------------ | --------- | --------- |
| mainnet  | TCP 8333     | TCP 8334  | TCP 8335  |
| testnet3 | TCP 18333    | TCP 18334 | TCP 18335 |
| testnet4 | TCP 28333    | TCP 18334 | TCP 18335 |
| chipnet  | TCP 48333    | TCP 18334 | TCP 18335 |
| regtest  | TCP 18444    | TCP 18334 | TCP 18335 |
| simnet   | TCP 18555    | TCP 18556 | TCP 18557 |
