# Ethernet

Ethernet carries frames between machines on one network.

## Frame layout

| Field | Size | Meaning |
|---|---|---|
| Destination MAC | 6 bytes | Who should receive it |
| Source MAC | 6 bytes | Who sent it |
| EtherType | 2 bytes | What is inside, such as `0x0800` for IPv4 |
| Payload | 46 to 1500 bytes | The packet |

### MAC addresses

A MAC address is 48 bits, written as six pairs: `52:54:00:12:34:56`.

### Broadcast

The address `ff:ff:ff:ff:ff:ff` reaches every machine on the network. [ARP](02-arp.md) depends on it.

## Seeing it

```bash
ip link show eth0
```

The picture of the whole stack is in the [overview](../../index.md).
