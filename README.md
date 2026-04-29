# Caddy Nomad Service Discovery

Caddy module that provides dynamic upstream discovery from HashiCorp Nomad services. Most of the code is adapted from [Caddy's SRV upstream source](https://github.com/caddyserver/caddy/blob/master/modules/caddyhttp/reverseproxy/upstreams.go).

## Installation

```
xcaddy build --with github.com/notarun/caddy-nomad-sd
```

## Automatic domain with SSL

```
{
    acme_dns <provider> <credentials>
}

*.example.com {
    reverse_proxy {
        dynamic nomadsd {
            namespace {labels.1}
            name      {labels.2}
            tag       {labels.3}
        }
    }
}
```

A request to `http-tag.my-service.my-ns.example.com` resolves to Nomad service `my-service` in namespace `my-ns` with tag `http-tag`.

## Usage

```
reverse_proxy {
    dynamic nomadsd <service_name> {
        name      <service_name>   # required: Nomad service name
        namespace <namespace>      # optional
        tag       <tag>            # optional
        refresh   <duration>       # optional: cache refresh interval (default: 1m)
    }
}
```

## Configuration

Set via environment variables:

- `NOMAD_ADDR` - Nomad server address (default: `http://127.0.0.1:4646`, or `unix:///path/to/socket`)
- `NOMAD_TOKEN` - ACL token
- `NOMAD_CACERT` / `NOMAD_CAPATH` - CA cert for TLS
- `NOMAD_CLIENT_CERT` / `NOMAD_CLIENT_KEY` - mTLS client cert/key
- `NOMAD_SKIP_VERIFY` - Skip TLS verification

## Module ID

`http.reverse_proxy.upstreams.nomadsd`
