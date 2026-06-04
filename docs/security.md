# Security

## What Is Covered
- Password hashes, not plaintext passwords
- Encrypted panel credentials
- JWT and refresh token handling
- CSRF protection on HTML forms
- Security headers
- Login rate limiting
- Request ID propagation
- Output escaping in templates

## Operational Guidance
- Set `HUB_SECRET_KEY` to a strong secret.
- Use HTTPS in production.
- Enable `TRUST_PROXY` only behind a trusted proxy.
- Keep Redis reachable and private.

## Panel URL Safety
- Only `http` and `https` panel URLs are accepted.
- Private IP URLs are warned about and can be rejected in strict mode.
