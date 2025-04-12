# avatarad
Gravatar-like LDAP avatar proxy service

[![Build Status](https://cloud.drone.io/api/badges/tkushnir/avatarad/status.svg)](https://cloud.drone.io/tkushnir/avatarad)
[![Go Report Card](https://goreportcard.com/badge/github.com/tkushnir/avatarad)](https://goreportcard.com/report/github.com/tkushnir/avatarad)

Sources can be found at https://github.com/tkushnir/avatarad

Docker hub project is here https://hub.docker.com/r/timophey73/avatarad

Currently only OpenLDAP servers are supported, but you may try it with MS AD.

Gravatar URL format is fully compatible with the service, but only `size` parameter is taken into account.

## usage (docker)

Simple way to use the `avatarad` service is to run the docker command:

```shell
docker run --rm \
    -e LDAP_SERVER_FQDN=ldap.example.org \
    -e LDAP_BIND_USER=cn=admin,dc=example,dc=org \
    -e LDAP_BIND_PASSWORD=S3cr3t \
    -e LDAP_USER_BASE=ou=users,dc=example,dc=org \
    -p 8080:8080 \
    timophey73/avatarad
```

Also one can use `docker-compose.yml` for this purpose:

```yaml
services:
  server:
    image: timophey73/avatarad
    restart: always
    ports:
    - 8080:8080
    environment:
      LDAP_SERVER_FQDN: ldap.example.org
      LDAP_BIND_USER: cn=admin,dc=example,dc=org
      LDAP_BIND_PASSWORD: S3cr3t
      LDAP_USER_BASE: ou=users,dc=example,dc=org
```

After that you could access the URL in your browser (change the IP-address):

```
http://192.168.1.1:8080/avatar/00000000000000000000000000000000
```

If you need to access the `avatarad` service with SSL (HTTPS), you should use proxy service (nginx, ha-proxy, traefik or similar). No support for SSL is planned in the `avatarad` service.

Currently there are some neat features supported:

healthz:
```
# curl -fsS http://192.168.1.1:8080/healthz; echo
OK
```

version:
```
# curl -fsS http://192.168.1.1:8080/version
{"version":"0.3.0.117"}
```

## available options

Currently the `avatarad` service is configured through environment variables. No command line options and no plans for them.

- `LDAP_SERVER_FQDN` (**required**) – fully qualified domain name of the LDAP server
- `LDAP_PORT` (optional, default: `636`) – TCP port the LDAP server listens on (may be 389 for ldap:// and 636 for ldaps://)
- `LDAP_SSL_CACERT_FILE` (optional) – path to root CA certificate file
- `LDAP_SSL` (optional, default: `true`) – whether SSL should be used to connect the LDAP server (ldaps://)
- `LDAP_TLS` (optional, default: `false`) – whether TLS should be used to connect the LDAP server (ldap:// + TLS)
- `LDAP_VERIFY_CERT` (optional, default: `true`) – whether the LDAP server SSL certificate should be verified
- `LDAP_BIND_USER` (**required**) – LDAP manager user dn (e.g. `cn=admin,dc=example,dc=org`)
- `LDAP_BIND_PASSWORD` (**required**) – LDAP manager password
- `LDAP_USER_BASE` (**required**) – LDAP subtree holding user accounts (e.g. `ou=People,dc=example,dc=org`)
- `LDAP_USER_FILTER` (optional, default: `(objectclass=inetOrgPerson)`) – filter users accounts
- `LDAP_AVATAR_ATTRIBUTE` (optional, default: `jpegPhoto`) – user avatar attribute
- `LDAP_EMAIL_ATTRIBUTE` (optional, default: `mail`) – user E-mail attribute
- `GRAVATAR_ENABLED` (optional, default: `false`) – whether to try fetching avatars from Gravatar service
- `GRAVATAR_URL` (optional, default: `https://secure.gravatar.com/avatar`) – base URL for Gravatar service

If Gravatar is *disabled* (`GRAVATAR_ENABLED = false`), the `avatarad` service tries to fetch a userpic from LDAP. If the userpic is not found the default avatar is used.

If Gravatar is *enabled* and local (LDAP) userpic is not found, the `avatarad` service tries to cache Gravatar userpic locally.
