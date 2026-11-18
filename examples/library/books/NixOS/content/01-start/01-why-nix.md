# Why Nix

| Usual distribution | NixOS |
|---|---|
| Install packages one command at a time | List packages in a file |
| Upgrades change the system in place | Each rebuild is a new generation |
| Undo by hand | Boot the previous generation |

> [!TIP]
> Try a package without installing it: `nix shell nixpkgs#ripgrep`.
