# First script

A script is a text file of commands with a first line that names the interpreter.

```bash
#!/usr/bin/env bash
set -euo pipefail

name="${1:-world}"
echo "Hello, ${name}"
```

Make it executable, then run it:

```bash
chmod +x hello.sh
./hello.sh Linux
```

> [!IMPORTANT]
> `set -euo pipefail` stops the script at the first error, on an unset variable, and on a failure anywhere in a pipeline.[^1]

[^1]: Without it, a script carries on after a failed command.
