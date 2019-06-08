# NGINX

## Serving bchrpc API through an NGINX reverse proxy (optional)
For various reasons, like load balancing, ssl handling, etc. It might be handy to serve the bchrpc API thourgh a reverse proxy, although not necessary. Here we provide a sample config for NGINX.

### Upstream
With load balancing
```
upstream bchrpc {
    ip_hash; # Session persistence: make same client always connect to same server
    server bchd01.bitcoin.cash:8335;
    server bchd02.bitcoin.cash:8335;
}
```

Without load balancing
```
upstream bchrpc {
    server bchd01.bitcoin.cash:8335;
}
```

### Location
```
location / {
        # Raise default timeout because blocks can take longer than 10 minutes (600 seconds) in between, this causes a timeout on SubscribeBlocks stream
        proxy_connect_timeout       3600;
        proxy_send_timeout          3600;
        proxy_read_timeout          3600;
        send_timeout                3600;
        # grpc requires http/2
        http2_push_preload          on;
        proxy_hide_header           X-Frame-Options;
        proxy_http_version          1.1;
        proxy_set_header            Upgrade $http_upgrade;
        proxy_set_header            Connection "upgrade";
        proxy_set_header            X-Real-IP $remote_addr;
        proxy_set_header            Host $http_host;
        proxy_set_header            X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header            X-Forwarded-Proto $scheme;
        proxy_redirect              off;

        proxy_pass https://bchrpc;
    }
```
