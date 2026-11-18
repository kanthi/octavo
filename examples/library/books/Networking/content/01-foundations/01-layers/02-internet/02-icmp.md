# ICMP

ICMP carries error and status messages for IP.

| Type | Meaning |
|---|---|
| 8 | Echo request |
| 0 | Echo reply |
| 11 | Time exceeded (TTL reached zero) |

The [ping](../../../02-netops/01-tools/01-inspect/02-ping.md) tool sends type 8 and waits for type 0.
