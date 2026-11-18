# ARP

ARP finds the MAC address that belongs to an IP address on the local network.

```text
Who has 10.0.0.2? Tell 10.0.0.1      (broadcast)
10.0.0.2 is at 52:54:00:12:34:56     (reply to the asker only)
```

```bash
ip neigh show
```

> [!WARNING]
> ARP has no authentication. Any machine on the network can answer for any address.

Next layer up: [IP](../02-internet/01-ip.md).
