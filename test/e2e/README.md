# End-to-End Tests

## IPv6

Enter the following in `daemon.json` (or in the Docker for Mac UI under Preferences -> Docker Engine):

```json
{
  "ipv6": true,
  "fixed-cidr-v6": "2001:db8:1::/64"
}
```

## Databases

Build as `BUILD_TAGS=rocksdb,boltdb,badgerdb make build-linux`.
