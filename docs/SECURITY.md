# WorkBridgeMCP security model

## Default posture

The intended default profile is:

- stdio transport;
- read roots explicitly enumerated;
- no write roots;
- process execution disabled;
- no network listener.

HTTP mode is optional, but when selected it is literal-loopback only and bearer
authentication is mandatory. `http.bearer_token_env` names the environment variable;
the token itself is not stored in config. HTTP startup fails closed if the variable is
missing, empty, shorter than 32 bytes, or contains whitespace.

## Filesystem boundaries

WorkBridge uses Go 1.25.12 `os.Root` for read/write roots. The source and CI intentionally pin the tested security floor to Go 1.25.12 rather than a floating 1.25.x toolchain.

A configured root is opened once. Operations use root-relative `Open`, `OpenFile`,
`Stat`, `Lstat`, and `Mkdir` calls rather than accepting a path after a separate
global path-validation step.

Per Go's `os.Root` contract, methods can access only files/directories beneath the root;
symbolic links may be followed only when they remain beneath the root, and absolute
symlink targets are rejected. This directly hardens traversal and symlink/reparse escape
compared with resolve-then-open logic.

Read operations:

- reject paths outside read roots;
- bound file size;
- accept UTF-8 text only for `workspace_read_text`;
- bound directory entry count.

Write operations:

- are not registered unless write roots exist;
- bound write size;
- create new files with exclusive creation;
- statically refuse symbolic-link leaf overwrite;
- require `overwrite=true` for existing files;
- create only one directory level at a time.

### Current claim ceiling

`os.Root` is the filesystem containment primitive, but WorkBridge does not claim that
this makes a workstation safe against an arbitrary malicious local administrator.

In particular, this source does not claim proof against every local object-identity race,
filesystem-boundary/mount behavior, device/special-file behavior on non-Windows systems,
or replacement of a permitted in-root object by another local actor. Windows is the
primary workstation target; Linux CI is portability evidence, not identical platform
semantics.

## Process boundaries

Process execution is absent unless explicitly enabled.

Each executable grant contains:

- an operator-chosen stable name;
- an absolute executable path;
- an exact lowercase SHA-256.

At startup WorkBridge resolves the path, verifies it is a regular file, and hashes it.
Immediately before execution it resolves and hashes the file again. Immediately after
execution it verifies the hash again.

The MCP caller supplies only the grant name and literal argument array. WorkBridge does
not route process requests through a shell. Child processes receive a reduced environment,
not the complete server environment.

Working directories are separately bounded by an `os.Root` policy.
Runtime, output bytes, argument count, and aggregate argument bytes are bounded.

### Current claim ceiling

Hash checks substantially reduce accidental executable substitution but are not a
cryptographic binding between the final hash read and the operating system's executable
open. High-assurance deployment should place admitted executables in operator-controlled,
non-writable locations and may require a future OS-specific executable-handle identity
mechanism.

The same principle applies to the working-directory path passed to the OS process API:
rooted validation constrains admission, but a mutually hostile local administrator is
outside the current qualification ceiling.

## Secrets

Do not put credentials, bearer tokens, API keys, or NAS credentials in:

- repository files;
- service-wrapper XML;
- committed JSON configuration;
- tool arguments unless the called local executable explicitly owns that secret flow.

HTTP bearer values are loaded through an environment-variable indirection.

## Transport

Stdio is preferred for desktop MCP clients.

Loopback HTTP uses stateless Streamable MCP sessions, exact endpoint matching, bearer
authentication, bounded HTTP timeouts/header size, and graceful shutdown.

WorkBridge refuses non-loopback listen addresses. Public/LAN exposure requires a new
reviewed transport/authentication design; changing a bind string is not sufficient.

## Effect semantics

Source presence is not installation.
A passing unit test is not a workstation effect.
A successful tool call is evidence of that call, not standing authority for another one.
