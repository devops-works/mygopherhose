# mygopherhose

Parallel importer for mysqldumps.

## What

mygopherhose uses a dump produced by `mysqldump` and imports it trying to
parallelize INSERT statements.

From [benchmarks](#gcp-benchmarks), it seems to perform 3x faster on small
instances (should be even better on high end machines).

## Caveats

- tables are not locked
- does not support SETting things
- does not support stored procedures

**Use at your own risks in production.**

## Usage

```
mygopherhose [-h host] -u user -p [password] [-P port] [-d dbname] [-b bufsize] dumpfile
        -h defaults to 127.0.0.1
        -P defaults to 3306
        -b defaults to 10485760 bytes
        -d can be omitted is dump contains `USE DATABASE foo;` stanza
        -p if parameter is empty, password will be asked interactively
```

## GCP benchmarks

- Client VM: n2-standard-2
- CloudSQL: db-n1-standard-8 / 260GB
- Dump: 
  - 20GB
  - 95 tables
  - 68781981 rows
  - data size 23.04G
  - index size 2.48 G

| description         |   duration | db CPU |
| ------------------- | ---------: | -----: |
| mysql cli           | 30m24.209s |   ~11% |
| mysql + source      |            |        |
| mygopherhose -c 10  |  13m2.016s | 54-87% |
| mygopherhose -c 20  | 12m51.301s | 60-90% |
| mygopherhose -c 40  | 12m51.799s | 60-93% |
| mygopherhose -c 100 |  13m9.446s | 69-97% |
