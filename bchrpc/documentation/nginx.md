# NGINX

## Serving bchrpc API through an NGINX reverse proxy (optional)
For various reasons, like load balancing, ssl handling, etc. It might be handy to serve the bchrpc API through a reverse proxy, although not necessary. Here we provide a sample config for NGINX.

### Upstream
With load balancing
```
upstream bchrpc {
    ip_hash; # Session persistence: make same client always connect to same server
    server bchd1.example.com:8335;
    server bchd2.example.com:8335;
}
```

Without load balancing
```
upstream bchrpc {
    server bchd1.example.com:8335;
}
```

### Server
gRPC needs HTTP/2 from the client, so the server block must enable it with TLS (on nginx before 1.25.1 use
`listen 443 ssl http2;` instead of `http2 on;`).
```
server {
    listen                  443 ssl;
    http2                   on;
    server_name             bchd.example.com;

    ssl_certificate         /path/to/your/cert;
    ssl_certificate_key     /path/to/your/key;

    location / {
        # https://nginx.org/en/docs/http/ngx_http_grpc_module.html

        # bchd sends nothing on a subscription until there is something to send, and an address
        # subscription can be quiet for days, so raise the 60 second default. Clients should still
        # subscribe again when a stream ends.
        grpc_read_timeout       1d;
        grpc_send_timeout       1d;

        # bchd only serves gRPC over TLS. nginx does not verify its self signed cert by default.
        grpc_pass               grpcs://bchrpc;
    }
}
```
