# Windows packaging

WorkBridge Windows service wrapper template is source-only.
It does not install a service, grant network access, change credentials,
or establish remote tool authority by itself.

The Go MCP server runs over stdio by default. Optional HTTP is literal
loopback-only and requires a bearer environment secret at least 32 bytes.
WinSW is not bundled; service installation is a separately authorized
operator action. WorkBridgeMCP.xml.template is not an installed service.
