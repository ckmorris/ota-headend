# Security

Report vulnerabilities privately to Chris Morris using the security
contact published in the official project repository on the ckmorris GitHub
account. If no security contact is listed, ask the maintainer for a private
channel before sending technical details. Do not put an exploit, saved lock key,
or private state directory in a public issue.

Include the application/tool versions, platform, affected command or endpoint,
impact, and a minimal reproduction without real credentials or private media.
There is no guaranteed response window or support commitment under the
PolyForm Noncommercial license.

The HLS listener has no built-in authentication or TLS and defaults to
`0.0.0.0:8090` for LAN playback. Restrict access to a trusted LAN, use a loopback
listener, or use an authenticated reverse proxy. Protect the durable state
directory: its tuner claims contain control keys. Give each installation its own
state and runtime directories and do not share those directories with untrusted
users. The process invokes locally installed tools from PATH; protect those
binaries and service configuration too.
