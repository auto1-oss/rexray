# Arm 64 fork of rexray

Used in graviton based images.

How to build and push.

> **WARNING:** `docker plugin create`/`docker plugin push` is broken when Docker Desktop uses
> the containerd image store (Settings → General → "Use containerd for pulling and storing
> images", enabled by default). The pushed layer gets UTF-8-mangled (gzip magic `1f 8b` becomes
> `1f ef bf bd`) and plugin install fails on hosts with
> `unpigz: corrupted -- invalid deflate data (invalid block type)`.
> Disable the containerd image store (classic `overlay2` store) before building/pushing,
> verify with `docker info | grep -i driver-type` (must NOT show `io.containerd.snapshotter.v1`).

1. Backup and delete existing `049736579808.dkr.ecr.eu-west-1.amazonaws.com/docker-plugins/rexray-ebs-arm64:stable`, as repo is immutable.
2. Build and push plugin.

```shell
DRIVER=ebs make docker-build-plugin
docker plugin push 049736579808.dkr.ecr.eu-west-1.amazonaws.com/docker-plugins/rexray-ebs-arm64:stable
```



## Build and push from a Mac via docker-in-docker

Docker Desktop's containerd image store corrupts plugin pushes (see warning above), and newer
Docker Desktop releases may drop the classic-store toggle entirely. Workaround: build and push
from a `docker:dind` container — its inner Docker engine runs in Docker Desktop's Linux VM with
the classic `overlay2` store, and the host image store never touches the plugin blobs.

1. Start dind and install build tools (`docker:28-dind` pinned on purpose — engine 29 defaults
   to the containerd store):

```shell
docker run -d --privileged --name rexray-dind docker:28-dind
docker exec rexray-dind apk add --no-cache bash make sudo git
docker exec rexray-dind docker info --format '{{.Driver}}'   # must print: overlay2
```

2. Copy the repo into the container, including `.git` (the build runs `git describe`).
   `COPYFILE_DISABLE=1` stops macOS tar from emitting `._*` AppleDouble files that break git:

```shell
cd <repo root>
COPYFILE_DISABLE=1 tar --exclude './.docker/plugins/ebs/rootfs' -cf - . \
  | docker exec -i rexray-dind sh -c 'mkdir -p /build && tar -xf - -C /build'
docker exec rexray-dind git config --global --add safe.directory /build
docker exec rexray-dind rm -f /build/core/core_generated.go   # stale/empty generated file breaks go generate
```

3. Build the plugin. `-t` is required because the Makefile invokes `docker run -it` internally:

```shell
docker exec -t -w /build rexray-dind bash -c 'DRIVER=ebs make docker-build-plugin'
```

4. The Makefile creates the plugin as `:stable`. For the `:bottlerocket` tag, remove `:stable`
   and re-create (docker refuses two plugins with identical content):

```shell
docker exec -w /build rexray-dind sh -c '
  docker plugin rm 049736579808.dkr.ecr.eu-west-1.amazonaws.com/docker-plugins/rexray-ebs-arm64:stable
  docker plugin create 049736579808.dkr.ecr.eu-west-1.amazonaws.com/docker-plugins/rexray-ebs-arm64:bottlerocket .docker/plugins/ebs'
```

5. Delete the existing tag from ECR (repo is immutable), log in to ECR inside dind and push:

```shell
aws ecr batch-delete-image --registry-id 049736579808 \
  --repository-name docker-plugins/rexray-ebs-arm64 --image-ids imageTag=bottlerocket
aws ecr get-login-password --region eu-west-1 \
  | docker exec -i rexray-dind docker login --username AWS --password-stdin 049736579808.dkr.ecr.eu-west-1.amazonaws.com
docker exec rexray-dind docker plugin push 049736579808.dkr.ecr.eu-west-1.amazonaws.com/docker-plugins/rexray-ebs-arm64:bottlerocket
```

6. Verify the pushed layer is a valid gzip blob (catches the containerd-store corruption):

```shell
DIGEST=$(aws ecr batch-get-image --registry-id 049736579808 \
  --repository-name docker-plugins/rexray-ebs-arm64 --image-ids imageTag=bottlerocket \
  --query 'images[0].imageManifest' --output text | jq -r '.layers[0].digest')
URL=$(aws ecr get-download-url-for-layer --registry-id 049736579808 \
  --repository-name docker-plugins/rexray-ebs-arm64 --layer-digest "$DIGEST" \
  --query downloadUrl --output text)
curl -s "$URL" | head -c 4 | xxd     # must start: 1f 8b 08
curl -s "$URL" | gunzip -t && echo GZIP_OK
```

7. Clean up:

```shell
docker rm -f rexray-dind
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

