# Two hosts, one switch

- [ ] Give `h1` the address `10.0.0.1/24`
- [ ] Give `h2` the address `10.0.0.2/24`
- [ ] From `h1`, run [ping](../01-tools/01-inspect/02-ping.md) against `h2`
- [ ] Read the ARP table on `h1`

```bash
ip addr add 10.0.0.1/24 dev eth0
ip link set eth0 up
```
