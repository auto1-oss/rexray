# Arm 64 fork of rexray

Used in graviton based images.

How to build and push.

1. Backup and delete existing `049736579808.dkr.ecr.eu-west-1.amazonaws.com/docker-plugins/rexray-ebs-arm64:stable`, as repo is immutable.
2. Build and push plugin.

```shell
DRIVER=ebs make docker-build-plugin
docker plugin push 049736579808.dkr.ecr.eu-west-1.amazonaws.com/docker-plugins/rexray-ebs-arm64:stable
```

