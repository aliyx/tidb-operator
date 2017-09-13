#!/bin/bash
# http://aliacs-k8s.oss-cn-hangzhou.aliyuncs.com/installer/kubemgr-1.6.7.sh

# set -x
set -e

root=$(id -u)
if [ "$root" -ne 0 ]; then
	echo must run as root
	exit 1
fi

version=1.7.5
# private docker registries
registries=10.209.224.13:10500
# file server
fserver=http://10.213.44.128:12701/kube

RPM_KUBEADM=$fserver/rpm/kubeadm-$version-0.x86_64.rpm
RPM_KUBECTL=$fserver/rpm/kubectl-$version-0.x86_64.rpm
RPM_KUBELET=$fserver/rpm/kubelet-$version-0.x86_64.rpm
RPM_KUBECNI=$fserver/rpm/kubernetes-cni-0.5.1-0.x86_64.rpm

images=(gcr.io/google_containers/kube-apiserver-amd64:v$version gcr.io/google_containers/kube-controller-manager-amd64:v$version gcr.io/google_containers/kube-scheduler-amd64:v$version gcr.io/google_containers/kube-proxy-amd64:v$version gcr.io/google_containers/etcd-amd64:3.0.17 gcr.io/google_containers/pause-amd64:3.0 gcr.io/google_containers/k8s-dns-sidecar-amd64:1.14.4 gcr.io/google_containers/k8s-dns-kube-dns-amd64:1.14.4 gcr.io/google_containers/k8s-dns-dnsmasq-nanny-amd64:1.14.4 gcr.io/google_containers/kubernetes-dashboard-amd64:v1.6.3)
weave=(weaveworks/weave-kube:1.9.8 weaveworks/weave-npc:1.9.8 weaveworks/weaveexec:1.9.8)

IpAddressRegex="^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$"
HostnameRegex="^(([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\-]*[a-zA-Z0-9])\.)*([A-Za-z0-9]|[A-Za-z0-9][A-Za-z0-9\-]*[A-Za-z0-9])$"

kube::common::is_ip() {
    echo "$1" | grep -E "^$IpAddressRegex$" >/dev/null || echo "$1" | grep -E "^$HostnameRegex$" >/dev/null
}

kube::common::os() {
	ubu=$(cat /etc/issue | grep "Ubuntu 16.04" | wc -l)
	cet=$(cat /etc/centos-release | grep "CentOS Linux release 7" | wc -l)
	if [ "$ubu" == "1" ]; then
		export OS="ubuntu"
	elif [ "$cet" == "1" ]; then
		export OS="CentOS"
	else
		echo "unkown os...   exit"
		exit 1
	fi
}

kube::common::yum_config() {
	sudo cat > /etc/yum.repos.d/kubernetes.repo <<-EOF
[kubernetes]
name=Kubernetes
baseurl=http://yum.kubernetes.io/repos/kubernetes-el7-x86_64
enabled=1
gpgcheck=1
repo_gpgcheck=1
gpgkey=https://packages.cloud.google.com/yum/doc/yum-key.gpg
       https://packages.cloud.google.com/yum/doc/rpm-package-key.gpg
EOF
}

kube::prepare_rpm() {
	kube::common::yum_config

	yumdownloader -v >/dev/null 2>&1 || yum install -y yum-utils
	yumdownloader --destdir=/tmp kubelet kubeadm kubectl kubernetes-cni
}

# https://kubernetes.io/docs/admin/kubeadm/
kube::prepare_image() {
	kube::install_docker
	echo starting pull images...

	# download kube depentent images
	for imageName in ${images[@]} ${weave[@]}; do
		dest=$registries/$imageName
		have=$(docker images | grep $(echo ${dest%:*}) | grep $(echo ${dest##*:}) | wc -l)
		if [ $have -lt 1 ]; then
			docker pull $imageName
			docker tag $imageName $dest
			docker rmi $imageName
			docker push $dest
		fi
	done
}

kube::install_docker() {
	docker version >/dev/null 2>&1 && exist=0
	if ! [ $exist ]; then
		# curl -sSL http://acs-public-mirror.oss-cn-hangzhou.aliyuncs.com/docker-engine/internet | sh -
		yum info docker-engine.x86_64
		yum install -y ebtables docker-engine-1.12.6

		# Set access to the docker registry protocol: https -> http
		if [ ! -d "/etc/docker" ]; then
			sudo mkdir /etc/docker
		fi
		echo { \"insecure-registries\":[\"$registries\"] } >/etc/docker/daemon.json

		systemctl enable docker && systemctl start docker
	fi

	set +e
	# Disabling SELinux by running setenforce 0 is required in order to allow containers to access the host filesystem
	setenforce 0
	set -e

	echo docker has been installed
}

kube::pull_images() {
	for imageName in ${images[@]}; do
		have=$(docker images | grep $(echo ${imageName%:*}) | wc -l)
		if [ $have -lt 1 ]; then
			docker pull $registries/$imageName
			docker tag $registries/$imageName $imageName
			docker rmi $registries/$imageName
		fi
	done
}

kube::install_binaries() {
	kubeadm version >/dev/null 2>&1 && kubeadm reset

	yum install -y socat
	rm -rf /tmp/kube && mkdir -p /tmp/kube
	curl -sS -L $RPM_KUBEADM >/tmp/kube/kubeadm.rpm
	curl -sS -L $RPM_KUBECTL >/tmp/kube/kubectl.rpm
	curl -sS -L $RPM_KUBELET >/tmp/kube/kubelet.rpm
	curl -sS -L $RPM_KUBECNI >/tmp/kube/kube-cni.rpm

	rpm -ivh /tmp/kube/kubectl.rpm /tmp/kube/kubelet.rpm /tmp/kube/kube-cni.rpm /tmp/kube/kubeadm.rpm

	systemctl enable kubelet

	# https://github.com/kubernetes/kubernetes/issues/43805
	sed -i 's#Environment="KUBELET_CGROUP_ARGS=-.*#Environment="KUBELET_CGROUP_ARGS=--cgroup-driver=cgroupfs"#g' /etc/systemd/system/kubelet.service.d/10-kubeadm.conf

	systemctl daemon-reload && systemctl start kubelet && rm -rf /etc/kubernetes

	echo 1 >/proc/sys/net/bridge/bridge-nf-call-iptables

	# check SELinux
	echo $(sestatus)
	# vi /etc/sysconfig/selinux
	# SELINUX=disabled
	# reboot

	# set apiserver port, as the company below 10000 port can not access
	echo alias kubectl='kubectl --server=127.0.0.1:10218' >/etc/profile.d/k8s.sh
	echo "export KUBERNETES_API_SERVER=127.0.0.1:10218" | tee -a /etc/profile.d/k8s.sh
	echo "export PATH=$PATH:/usr/local/bin/" | tee -a /etc/profile.d/k8s.sh

	# sync system time: ntp.api.bz is china
	ntpdate -v >/dev/null 2>&1 || yum -y install ntpdate
	ntpdate -u 10.209.100.2
	# write system time to CMOS
	clock -w
}

kube::upgrade_binaries() {
	rm -rf /tmp/kube && mkdir -p /tmp/kube
	curl -sS -L $RPM_KUBEADM >/tmp/kube/kubeadm.rpm
	curl -sS -L $RPM_KUBECTL >/tmp/kube/kubectl.rpm
	curl -sS -L $RPM_KUBELET >/tmp/kube/kubelet.rpm
	curl -sS -L $RPM_KUBECNI >/tmp/kube/kube-cni.rpm

	rpm -Uvh /tmp/kube/kubectl.rpm /tmp/kube/kubelet.rpm /tmp/kube/kube-cni.rpm /tmp/kube/kubeadm.rpm --nofiledigest --replacepkgs

	systemctl daemon-reload && systemctl start kubelet
}

# https://kubernetes.io/docs/tasks/administer-cluster/kubeadm-upgrade-1-7/
kube::master_upgrade() {
	kube::pull_images

	kube::upgrade_binaries

	KUBECONFIG=/etc/kubernetes/admin.conf kubectl delete daemonset kube-proxy -n kube-system

	curl -sS -L $fserver/master.yaml >/tmp/kube/master.yaml
	kubeadm init --skip-preflight-checks --config /tmp/kube/master.yaml

	# https://www.weave.works/docs/net/latest/kubernetes/kube-addon/
	kubectl apply -f $fserver/weave-daemonset-k8s-1.6.yaml
}

kube::node_upgrade() {
	kube::pull_images
	kube::upgrade_binaries
}

# see logs
# journalctl -xeu kubelet
kube::master_up() {
	kube::common::os

	kube::install_docker

	kube::pull_images

	kube::install_binaries

	curl -sSL $fserver/master.yaml >/tmp/kube/master.yaml
	kubeadm init --config /tmp/kube/master.yaml

	# To start using your cluster, you need to run (as a regular user):
	#	mkdir -p $HOME/.kube
	#   sudo cp -i /etc/kubernetes/admin.conf $HOME/.kube/config
	#   sudo chown $(id -u):$(id -g) $HOME/.kube/config
	chmod 666 /etc/kubernetes/admin.conf
	export KUBECONFIG=/etc/kubernetes/admin.conf
	echo "export KUBECONFIG=/etc/kubernetes/admin.conf" | tee -a /etc/profile.d/k8s.sh

	#install weave network
	# use weave as pod network https://www.weave.works/docs/net/latest/kube-addon/
	# https://github.com/weaveworks/weave/releases/
	# 2.0.2 requires ipset version 6.29 or higher, that set types support the optional comment extension.
	kubectl apply -f $fserver/weave-daemonset-k8s-1.6.yaml
	curl -sSL $fserver/weave-1.9.8.sh >/usr/local/bin/weave
	chmod a+x /usr/local/bin/weave

	kubectl apply -f $fserver/kubernetes-dashboard.yaml

	#show pods
	kubectl -n kube-system get po
	echo kubectl -n kube-system get po
}
kube::node_up() {
	kube::common::os

	kube::install_docker

	kube::pull_images

	kube::install_binaries

	kubeadm join --token 997ea0.40e5c1218d0afc50 $@:6443
}

kube::weae_reset() {
	echo clear weave net...
	# weave reset need docker
	docker version >/dev/null 2>&1 || kube::install_docker
	export DOCKERHUB_USER=10.209.224.13:10500/weaveworks
	curl -sSL $fserver/weave-1.9.8.sh >/tmp/weave && chmod a+x /tmp/weave && /tmp/weave reset
}

kube::tear_down() {
	# yum remove -y ebtables docker docker-common container-selinux docker-selinux docker-engine socat
	kubelet --version >/dev/null 2>&1 && systemctl stop kubelet
	docker ps -aq | xargs -I '{}' docker stop {}
	docker ps -aq | xargs -I '{}' docker rm {}
	# docker images -q | xargs -I '{}' docker rmi -f {}
	df | grep /var/lib/kubelet | awk '{ print $6 }' | xargs -I '{}' umount {}
	rm -rf /var/lib/kubelet && rm -rf /var/lib/dockershim && rm -rf /etc/kubernetes/ && rm -rf /var/lib/etcd
	yum remove -y kubectl kubeadm kubelet kubernetes-cni
	rm -rf /var/lib/cni
	rm -rf /etc/cni/

	kube::weae_reset
	# ip link del weave
}

main() {
	case $1 in
	"prepare")
		shift
		if [ $# -lt 1 ]; then
			kube::prepare_rpm
			kube::prepare_rpm
		elif [ "$@" == "rpm" ]; then
			kube::prepare_rpm
		elif [ "$@" == "image" ]; then
			kube::prepare_image
		else
			echo "unkown command $0 prepare $@"
			exit 1
		fi
		;;
	"master")
		kube::master_up
		;;
	"upgrade")
		shift
		if [ "$@" == "master" ]; then
			kube::master_upgrade
		elif [ "$@" == "node" ]; then
			kube::node_upgrade
		else
			echo "unkown command $0 upgrade $@"
			exit 1
		fi
		;;
	"join")
		shift
		if ! kube::common::is_ip "$@"; then
			echo "please specify master ip"
			exit 1
		else
			kube::node_up $@
		fi
		;;
	"down")
		kube::tear_down
		;;
	*)
		echo "usage: $0 prepare [rpm|image] | master | join ip | down | upgrade [master|node] "
		echo ""
		echo "       $0 prepare  to download k8s rpm and dependency images "
		echo "       $0 master   to setup master "
		echo "       $0 upgrade  to upgrade master "
		echo "       $0 join     to join master with ip "
		echo "       $0 down     to tear all down ,inlude all data! so becarefull"
		echo "       unkown command $0 $@"
		;;
	esac
}

main $@
