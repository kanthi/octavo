# The configuration file

The system lives in `/etc/nixos/configuration.nix`.

```nix
{ pkgs, ... }:
{
  networking.hostName = "books";
  environment.systemPackages = with pkgs; [ git go rustc ];
  services.openssh.enable = true;
}
```

Apply it:

```bash
sudo nixos-rebuild switch
```

> [!NOTE]
> Something broke? `sudo nixos-rebuild switch --rollback` returns to the previous generation.
