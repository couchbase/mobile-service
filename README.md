
## Overview

Prototype for a component of SG that would reside in the NS-Server process tree and facilitate:

- Tighter integration
- Config management
- UI 
- Etc

![architecture](docs/diagrams/sg-cbs-integration-option5.jpg)

## Status

Under construction.  This is just early prototyping code.

## NS-Server version

This code expects the ns-server fork at:

https://github.com/tleyden/ns_server

on the branch: **sg_cbs_integration3**

## How to run standalone

1. Start Couchbase Server
1. Create a Couchbase Server user with Cluster Admin access and credentials:
   - Username: testuser
   - Password: testpass

Startup command:

```
CBAUTH_REVRPC_URL=http://testuser:testpass@127.0.0.1:8091/mobile_mds go run main.go
```

If you're running multiple nodes via `./cluster_run`, then change the port above to `9000` or `9001`

The following errors are expected when running the mobile service standalone while also embedded inside Couchbase Server, if the mobile service binary has been deployed there:

```
2018/08/22 16:04:13 revrpc: Got error (EOF) and will retry in 1s
```

which happens because the embedded mobile service is already connected to NS-Server revprc, which only supports _one_ connection per service, and so it ends with a connection/disconnection flip-flop loop.

## How to run embedded inside Couchbase Server

1. Run `./deploy_to_nsserver.sh`
1. Go to ns_server subdir under couchbase_src
1. Run `./cluster_run -n 2`
1. See logs in `logs/n_0/non_existent_mds_logs` and `logs/n_1/non_existent_mds_logs`