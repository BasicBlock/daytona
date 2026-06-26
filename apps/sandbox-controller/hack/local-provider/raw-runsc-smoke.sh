#!/usr/bin/env bash
set -euo pipefail

VM_NAME="${1:-daytona-sandbox}"

limactl shell --workdir / "$VM_NAME" sudo bash -lc '
set -euo pipefail
RUNSC_ROOT=/tmp/daytona-raw-runsc-root
WORKDIR=/tmp/daytona-oci-smoke
ARTIFACT=/var/lib/daytona-localrunsc/raw-smoke
rm -rf "$RUNSC_ROOT" "$WORKDIR" "$ARTIFACT"
mkdir -p "$RUNSC_ROOT" "$WORKDIR/rootfs/bin" "$ARTIFACT"
cp /usr/bin/busybox "$WORKDIR/rootfs/bin/busybox"
ln -s busybox "$WORKDIR/rootfs/bin/sh"
ln -s busybox "$WORKDIR/rootfs/bin/sleep"
cd "$WORKDIR"

runsc spec -- /bin/sh -c "while true; do sleep 5; done"

runsc --root "$RUNSC_ROOT" run --bundle "$WORKDIR" --detach --pid-file /tmp/daytona-raw.pid daytona-raw
runsc --root "$RUNSC_ROOT" state daytona-raw
runsc --root "$RUNSC_ROOT" checkpoint --image-path "$ARTIFACT" daytona-raw
test -f "$ARTIFACT/checkpoint.img"
test -f "$ARTIFACT/pages.img"
test -f "$ARTIFACT/pages_meta.img"
runsc --root "$RUNSC_ROOT" delete daytona-raw
runsc --root "$RUNSC_ROOT" restore --bundle "$WORKDIR" --image-path "$ARTIFACT" --detach --pid-file /tmp/daytona-raw-restore.pid daytona-raw-restored
runsc --root "$RUNSC_ROOT" state daytona-raw-restored
runsc --root "$RUNSC_ROOT" kill daytona-raw-restored KILL || true
runsc --root "$RUNSC_ROOT" delete daytona-raw-restored || true
'
