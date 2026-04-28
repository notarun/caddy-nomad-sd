# Caddy Nomad Service Discovery

Caddy module that provides dynamic upstream discovery from HashiCorp Nomad services.

## Installation

```
xcaddy build --with github.com/notarun/caddy-nomad-sd
```

## Usage

```
reverse_proxy {
    # Use Nomad service discovery for dynamic upstreams
    # <service_name> is the Nomad service to discover
    dynamic nomadsd <service_name> {
        name    <service_name>   # required: Nomad service name
        refresh <duration>       # optional: cache refresh interval (default: 1m)
    }
}
```

## Configuration

Set via environment variables:

- `NOMAD_ADDR` - Nomad server address (default: `http://127.0.0.1:4646`)
- `NOMAD_TOKEN` - ACL token
- `NOMAD_CACERT` / `NOMAD_CAPATH` - CA cert for TLS
- `NOMAD_CLIENT_CERT` / `NOMAD_CLIENT_KEY` - mTLS client cert/key
- `NOMAD_SKIP_VERIFY` - Skip TLS verification

## Module ID

`http.reverse_proxy.upstreams.nomadsd`
