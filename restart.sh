#!/bin/bash

# set -x

# stop
if [ -f "/tmp/tidb-operator.pid" ]; then
	pid=$(cat /tmp/tidb-operator.pid)
	if [ ! -z "$pid" ]; then
		kill -9 $pid
	else
		killall tidb-operator
	fi
	rm /tmp/tidb-operator.pid
	echo "stop old tidb-operator server...ok"
fi

fail() {
	echo "ERROR: $1"
	exit 1
}

warn() {
  echo -e "\033[33m$1 \033[0m"
}

# The command line help
display_help() {
	echo "Usage: $0 [option...]" >&2
	echo
	echo "   -r, --run-mode         Run mode {dev|test|prod}. (default dev)"
	echo "   -i, --init-md          Force init default metadata. (default false)"
	echo "   -p, --host-path        The volume of pod host path, (default '/data')"
	echo "   -m, --mount            The prefix of pod mount path, default('')"
  echo "   -k, --k8s-address      The Kubernetes api server, default('http://10.213.44.128:10218')"
	echo
	exit 1
}

runMode="dev"
hostPath=/
mount=data
initMd=false
k8sAddress=http://10.213.44.128:10218

# Check if parameter is set too execute
while :; do
	case "$1" in
	-r | --run-mode)
		runMode="$2"
		shift 2
		;;
  -i | --init-md)
		initMd="$2"
		shift 2
		;;
  -p | --host-path)
		hostPath="$2"
		shift 2
		;;
   -m | --mount)
		mount="$2"
		shift 2
		;;
  -k | ----k8s-address)
		k8sAddress="$2"
		shift 2
		;;
	-h | --help)
		display_help
		exit 0
		;;
	--) # End of all options
		shift
		break
		;;
	-*)
		echo "Error: Unknown option: $1" >&2
		exit 1
		;;
	*) # No more options
		break
		;;
	esac
done

case "$runMode" in
  dev)
    warn "Current environment: dev" 
    ;;
  test)
    warn "Current environment: test" 
    ;;
  prod)
    warn "Current environment: prod" 
    ;;
  *)
    warn "No environment: $runMode"
    exit 1
esac

ip=$(/sbin/ifconfig eth0 | grep 'netmask ' | cut -d: -f2 | awk '{print $2}')
if [ -z "$ip" ]; then
	ip=0.0.0.0
fi

top=$(pwd)
# top sanity check
if [[ "$top" == "${top/\/src\/github.com\/ffan\/tidb-operator/}" ]]; then
	fail "top($top) does not contain src/github.com/ffan/tidb-operator"
fi

go version 2>&1 >/dev/null || fail "Go is not installed or is not on \$PATH"

set -e

echo "Start building tidb-operator..."
CGO_ENABLED=0 go build -ldflags '-d -w -s'

if ! [ -f tidb-operator ]; then
	fail "build failed"
fi

# start
./tidb-operator \
	-runmode=$runMode \
	-k8s-address=$k8sAddress \
	-host-path=$hostPath \
	-mount=$mount \
	-init-md=$initMd \
	-http-addr=$ip &
echo $! >/tmp/tidb-operator.pid
