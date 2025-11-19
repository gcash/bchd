# NGINX

## Serving bchrpc API through an NGINX reverse proxy (optional)
For various reasons, like load balancing, ssl handling, etc. It might be handy to serve the bchrpc API through a reverse proxy, although not necessary. Here we provide a sample config for NGINX.

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
        # https://nginx.org/en/docs/http/ngx_http_grpc_module.html
        
        # Raise default timeout because blocks can take longer than 10 minutes (600 seconds) in between, 
        # this causes a timeout on SubscribeBlocks stream
        grpc_read_timeout       3600;
        grpc_send_timeout       3600;
            
        grpc_pass               grpcs://bchrpc;
    }
```
