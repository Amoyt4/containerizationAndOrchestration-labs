set -euo pipefail
API_BIN="${API_BIN:-$(pwd)/api}"
SECCOMP_WRAPPER="${SECCOMP_WRAPPER:-$(pwd)/seccomp_wrapper}"
CGROUP_NAME="${CGROUP_NAME:-mydocker}"
CGROUP_ROOT="/sys/fs/cgroup"
CGROUP="$CGROUP_ROOT/$CGROUP_NAME"
MEM_LIMIT_MB="${MEM_LIMIT_MB:-100}"
CPU_QUOTA="${CPU_QUOTA:-50000 100000}"
PIDS_MAX="${PIDS_MAX:-50}"

echo "+cpu +memory +pids" > "$CGROUP_ROOT/cgroup.subtree_control" 2>/dev/null || true

mkdir -p "$CGROUP"

echo $((MEM_LIMIT_MB * 1024 * 1024)) > "$CGROUP/memory.max"
echo 0                               > "$CGROUP/memory.swap.max"
echo "$CPU_QUOTA"                    > "$CGROUP/cpu.max"
echo "$PIDS_MAX"                     > "$CGROUP/pids.max"

echo "[mydocker] cgroup $CGROUP: mem=${MEM_LIMIT_MB}M cpu=$CPU_QUOTA pids=$PIDS_MAX"
exec bash -c '
  echo "[mydocker] помещаю PID $$ в '"$CGROUP"'"
  echo $$ > "'"$CGROUP"'/cgroup.procs"
  exec unshare --user --map-root-user --pid --mount --net --uts --ipc --fork --mount-proc setpriv --bounding-set=-all --inh-caps=-all -- "'"$SECCOMP_WRAPPER"'" "'"$API_BIN"'" '