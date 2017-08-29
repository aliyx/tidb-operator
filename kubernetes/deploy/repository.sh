#!/bin/bash

set -e
# set -x

root=$(id -u)
if [ "$root" -ne 0 ]; then
	echo must run as root
	exit 1
fi

script_root=$(pwd)

ngx::start() {
	nginx -v >/dev/null 2>&1 || yum install -y nginx
	if pgrep -x "nginx" >/dev/null; then
		echo "stop file server"
		nginx -s stop
	fi
	if [ ! -d "/srv/kube" ]; then
		mkdir -p /srv/kube
	fi
	cp * /srv/kube/
	nginx -c $script_root/nginx.conf
}

ngx::stop() {
	if pgrep -x "nginx" >/dev/null; then
		echo "stop file server"
		nginx -s stop
	fi
}

ngx::restart() {
	if pgrep -x "nginx" >/dev/null; then
		echo "restart file server"
		cp * /srv/kube/
		nginx -s reload
	else
		echo "file server not started"
	fi
}

main() {
	case $1 in
	"start")
		ngx::start
		;;
	"stop")
		ngx::stop
		;;
	"restart")
		ngx::restart
		;;
	*)
		echo "usage: $0 start | stop | restart "
		echo "       $0 start to setup the file server "
		echo "       $0 stop to stop server "
		echo "       $0 restart to update server "
		echo "       unkown command $0 $@"
		;;
	esac
}

main $@
