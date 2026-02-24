# Arm 64 fork of rexray

Used in graviton based images.

How to build and push.

1. Backup and delete existing `049736579808.dkr.ecr.eu-west-1.amazonaws.com/docker-plugins/rexray-ebs-arm64:stable`, as repo is immutable.
2. Build and push plugin.

```shell
DRIVER=ebs make docker-build-plugin
docker plugin push 049736579808.dkr.ecr.eu-west-1.amazonaws.com/docker-plugins/rexray-ebs-arm64:stable
```



To build for amd64

```bash
docker run --rm -ti --entrypoint bash --platform linux/amd64 -v `pwd`:/go/src/github.com/rexray/rexray golang:1.9.1
go generate
go build -tags 'ebs' -o rexray
exit
cp rexray .docker/plugins/ebs/
docker build --platform linux/amd64 --label driver="ebs" --label semver="0.11.4-a1" -t rootfsimage .docker/plugins/ebs/
sudo rm -rf .docker/plugins/ebs/rootfs/
sudo mkdir .docker/plugins/ebs/rootfs/
iid=$(docker create --platform linux/amd64 rootfsimage true)
sudo docker export $iid | sudo tar -x -C .docker/plugins/ebs/rootfs/
sudo docker plugin create 049736579808.dkr.ecr.eu-west-1.amazonaws.com/docker-plugins/rexray-ebs:bottlerocket .docker/plugins/ebs/
```

