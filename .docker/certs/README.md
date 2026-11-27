# AWS RDS CA bundle

`rds-global-bundle.pem` is the public AWS RDS global trust bundle. RDS PostgreSQL serves
certificates signed by a CA that is not in the OS trust store, so the api and auth images ship
this bundle at `/etc/ssl/certs/rds-global-bundle.pem`. Production connection URLs reference it:

```
postgres://...?sslmode=verify-full&sslrootcert=/etc/ssl/certs/rds-global-bundle.pem
```

Local development and the staging VPS use `sslmode=disable` or their own URLs and never read it.

|             |                                                                                |
| ----------- | ------------------------------------------------------------------------------ |
| Source      | <https://truststore.pki.rds.amazonaws.com/global/global-bundle.pem>            |
| Fetched     | 2026-11-27                                                                     |
| Certificates | 114                                                                           |
| sha256      | `90a6f1a7978a1b1b096dca6c57511ca48d3b8cf052f8e704baf358f34ea2f21b`             |

It holds public certificates only, no private keys. Image builds never fetch it: they copy it from
the repository. Docker builds use `api/` and `auth/` as their contexts (see
`.docker/compose.build.yaml` and `.github/workflows/ci.yml`), so each image needs its own copy and the
same file is committed twice and the copies must stay byte-identical:

- `api/certs/rds-global-bundle.pem` (also copied into the migrate image, which builds from the repository root)
- `auth/certs/rds-global-bundle.pem`

## Refresh

AWS rotates and adds RDS CAs, so refresh the bundle when AWS announces a new CA or before a
certificate in it expires. From the repository root:

```sh
curl -fsSL https://truststore.pki.rds.amazonaws.com/global/global-bundle.pem -o /tmp/rds-global-bundle.pem
grep -c 'BEGIN CERTIFICATE' /tmp/rds-global-bundle.pem   # expect more than 100
for dir in api/certs auth/certs; do install -m 0644 /tmp/rds-global-bundle.pem "$dir/rds-global-bundle.pem"; done
sha256sum api/certs/rds-global-bundle.pem auth/certs/rds-global-bundle.pem
```

Both sums must match. Then update the fetch date, certificate count and sha256 above and
commit. Rebuilt images pick the new bundle up on their next deploy.
