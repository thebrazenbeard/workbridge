# Troubleshooting

## Client cannot start WorkBridge

Check:
- the configured binary path is absolute and exists;
- the client account can execute the binary;
- the WorkBridge config file exists and is readable;
- stdout is reserved for MCP protocol traffic in stdio mode;
- diagnostic logging goes to stderr or a file rather than stdout.

## Initialize times out

Run the black-box smoke test directly:

```powershell
./scripts/Test-WorkBridgeBinary.ps1 -Binary path/to/workbridge-mcp.exe
```

If the process exits, inspect stderr and its exit code.

## Filesystem access is denied

A denial is not automatically a bug. Confirm:
- the path is inside an explicitly configured root;
- the requested read/write capability is enabled;
- the path does not resolve through a symlink/junction outside the root;
- the process identity running WorkBridge can access the path.

Do not fix a denial by widening the root to an entire drive unless that is actually
the authority you intend to grant.

## Process execution is denied

Confirm the runtime policy explicitly enables process execution and that the
requested command satisfies its allow/deny rules.

Do not add a shell wrapper merely to bypass a command policy.

## HTTP client cannot connect

For a local workstation deployment, the server should normally be loopback-only.
Confirm that the client and server are on the same workstation and that the
configured URL uses the intended loopback address/port.

Do not open a firewall port as a troubleshooting shortcut.
