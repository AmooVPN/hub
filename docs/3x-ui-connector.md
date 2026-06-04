# 3x-ui Connector

## Scope
- The app talks to 3x-ui panels only through HTTP APIs.
- It does not touch the panel database directly.

## Behaviors
- Login with panel username/password
- List inbounds
- Add, update, delete clients
- Refresh traffic and session data
- Normalize panel errors
- Retry once after a stale session returns `401` or `403`

## URL Rules
- Only `http` and `https` URLs are accepted.
- Base URLs are normalized before storage.
- Private IP panel URLs are warned about in the UI.

## Timeout
- Remote panel calls use request timeouts.
