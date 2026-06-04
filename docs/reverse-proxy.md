# Reverse Proxy Examples

Use a reverse proxy only if `TRUST_PROXY=true`.

## Nginx

```nginx
server {
    listen 443 ssl http2;
    server_name hub.example.com;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto https;
        proxy_set_header X-Forwarded-Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

## Caddy

```caddyfile
hub.example.com {
    reverse_proxy 127.0.0.1:8080
}
```

## Cloudflare Tunnel

- Point the tunnel at `http://127.0.0.1:8080`.
- Keep `APP_BASE_URL` set to the public HTTPS URL.
- Enable `TRUST_PROXY=true` so forwarded headers are honored.

## HTTPS

- `APP_BASE_URL` must match the public URL.
- Secure cookies require HTTPS.
- Do not trust forwarded headers unless the proxy is trusted.
