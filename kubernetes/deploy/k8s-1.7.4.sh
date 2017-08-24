#!/bin/bash
# set -x
set -e
root=$(id -u)
if [ "$root" -ne 0 ]; then
	echo must run as root
	exit 1
fi

version=1.7.4
# private docker registries
registries=10.209.224.13:10500
# file server
fserver=http://10.213.44.128:12701/kube

RPM_KUBEADM=$fserver/rpm/kubeadm-$version-0.x86_64.rpm
RPM_KUBECTL=$fserver/rpm/kubeadm-$version-0.x86_64.rpm
RPM_KUBELET=$fserver/rpm/kubelet-$version-0.x86_64.rpm
RPM_KUBECNI=$fserver/rpm/kubernetes-cni-0.5.1-0.x86_64.rpm

images=(gcr.io/google_containers/kube-apiserver-amd64:v$version gcr.io/google_containers/kube-controller-manager-amd64:v$version gcr.io/google_containers/kube-scheduler-amd64:v$version gcr.io/google_containers/kube-proxy-amd64:v$version gcr.io/google_containers/etcd-amd64:3.0.17 gcr.io/google_containers/pause-amd64:3.0 gcr.io/google_containers/k8s-dns-sidecar-amd64:1.14.4 gcr.io/google_containers/k8s-dns-kube-dns-amd64:1.14.4 gcr.io/google_containers/k8s-dns-dnsmasq-nanny-amd64:1.14.4 gcr.io/google_containers/kubernetes-dashboard-amd64:v1.6.3 weaveworks/weave-kube:2.0.2 weaveworks/weave-npc:2.0.2)

kube::install_docker() {
	set +e
	which docker >/dev/null 2>&1
	i=$?
	if [ $i -ne 0 ]; then
		# curl -sSL http://acs-public-mirror.oss-cn-hangzhou.aliyuncs.com/docker-engine/internet | sh -
		yum info docker-engine.x86_64
		yum install -y ebtables docker-engine-1.12.6

		# Set access to the docker registry protocol: https -> http
		if [ ! -d "/etc/docker" ]; then
			sudo mkdir /etc/docker
		fi
		echo { "insecure-registries":["$registries"] } >/etc/docker/daemon.json

		systemctl enable docker.service && systemctl start docker.service
	fi

	# Disabling SELinux by running setenforce 0 is required in order to allow containers to access the host filesystem
	setenforce 0

	echo docker has been installed
}

# https://kubernetes.io/docs/admin/kubeadm/
kube::prepare() {
	yumdownloader -v >/dev/null 2>&1 || yum install -y yum-utils

	# download kube rpm
	cat <<EOF >/etc/yum.repos.d/kubernetes.repo
[kubernetes]
name=Kubernetes
baseurl=https://packages.cloud.google.com/yum/repos/kubernetes-el7-x86_64
enabled=1
gpgcheck=1
repo_gpgcheck=1
gpgkey=https://packages.cloud.google.com/yum/doc/yum-key.gpg
        https://packages.cloud.google.com/yum/doc/rpm-package-key.gpg
EOF

	yumdownloader --destdir=/tmp kubelet kubeadm kubectl kubernetes-cni

	# download kube depentent images
	for imageName in ${images[@]}; do
		have=$(docker images | $registries/$imageName | wc -l)
		if [ $have -lt 1 ]; then
			docker pull $imageName
			docker tag $imageName $registries/$imageName
			docker rmi $imageName
			docker push $registries/$imageName
		fi
	done
}

kube::pull_images() {
	for imageName in ${images[@]}; do
		have=$(docker images | $imageName | wc -l)
		if [ $have -lt 1 ]; then
			docker pull $registries/$imageName
			docker tag $registries/$imageName $imageName
			docker rmi $registries/$imageName
		fi
	done
}

kube::install_bin() {
	kubeadm version >/dev/null 2>&1 && kubeadm reset
	
	yum install -y socat
	rm -rf /tmp/kube && mkdir -p /tmp/kube
	curl -sS -L $RPM_KUBEADM >/tmp/kube/kubeadm.rpm
	curl -sS -L $RPM_KUBECTL >/tmp/kube/kubectl.rpm
	curl -sS -L $RPM_KUBELET >/tmp/kube/kubelet.rpm
	curl -sS -L $RPM_KUBECNI >/tmp/kube/kube-cni.rpm

	rpm -ivh /tmp/kube/kubectl.rpm /tmp/kube/kubelet.rpm /tmp/kube/kube-cni.rpm /tmp/kube/kubeadm.rpm

	systemctl enable kubelet.service && systemctl start kubelet.service && rm -rf /etc/kubernetes

	echo 1 >/proc/sys/net/bridge/bridge-nf-call-iptables

	# set apiserver port, as the company below 10000 port can not access
	echo alias kubectl='kubectl --server=127.0.0.1:10218' >/etc/profile.d/k8s.sh

	# sync system time: ntp.api.bz is china
	ntpdate -v >/dev/null 2>&1 || yum -y install ntpdate
	ntpdate -u 10.209.100.2
	# write system time to CMOS
	clock -w
}

# https://kubernetes.io/docs/tasks/administer-cluster/kubeadm-upgrade-1-7/
kube::master_upgrade() {
	rm -rf /tmp/kube && mkdir -p /tmp/kube
	curl -sS -L $RPM_KUBEADM >/tmp/kube/kubeadm.rpm
	curl -sS -L $RPM_KUBECTL >/tmp/kube/kubectl.rpm
	curl -sS -L $RPM_KUBELET >/tmp/kube/kubelet.rpm
	curl -sS -L $RPM_KUBECNI >/tmp/kube/kube-cni.rpm
	curl -sS -L $fserver/master.yaml >/tmp/kube/master.yaml

	rpm -Uvh /tmp/kube/kubectl.rpm /tmp/kube/kubelet.rpm /tmp/kube/kube-cni.rpm /tmp/kube/kubeadm.rpm --nofiledigest --replacepkgs

	# Pull the base image of kubernetes 
	for imageName in ${images[@]}; do
		docker pull $registries/$imageName
		docker tag $registries/$imageName $imageName
		docker rmi $registries/$imageName
	done

	systemctl daemon-reload && systemctl start kubelet

	KUBECONFIG=/etc/kubernetes/admin.conf kubectl delete daemonset kube-proxy -n kube-system

	kubeadm init --skip-preflight-checks --config /tmp/kube/master.yaml

	# https://www.weave.works/docs/net/latest/kubernetes/kube-addon/
	kubectl apply -f $fserver/weave-daemonset-1.7.yaml
}

kube::node_upgrade() {
	rm -rf /tmp/kube && mkdir -p /tmp/kube
	curl -sS -L $RPM_KUBEADM >/tmp/kube/kubeadm.rpm
	curl -sS -L $RPM_KUBECTL >/tmp/kube/kubectl.rpm
	curl -sS -L $RPM_KUBELET >/tmp/kube/kubelet.rpm
	curl -sS -L $RPM_KUBECNI >/tmp/kube/kube-cni.rpm

	rpm -Uvh /tmp/kube/kubectl.rpm /tmp/kube/kubelet.rpm /tmp/kube/kube-cni.rpm /tmp/kube/kubeadm.rpm

	systemctl daemon-reload && systemctl start kubelet
}

kube::master_up() {
	kube::install_docker

	kube::pull_images

	kube::install_bin

	curl -sS -L $fserver/master.yaml > /tmp/kube/master.yaml
	kubeadm init --config /tmp/kube/master.yaml

	# To start using your cluster, you need to run (as a regular user):
	export KUBECONFIG=/etc/kubernetes/admin.conf
    echo "export KUBECONFIG=/etc/kubernetes/admin.conf" >> /etc/profile

	#install weave network
	kubectl taint nodes --all dedicated-
	# set pod network
	# use weave as pod network https://www.weave.works/docs/net/latest/kube-addon/
	# kubectl apply -f https://github.com/weaveworks/weave/releases/download/latest_release/weave-daemonset-k8s-1.6.yaml
	kubectl apply -f $fserver/weave-daemonset-1.7.yaml

	kubectl apply -f $fserver/kubernetes-dashboard.yaml
	#show pods
	kubectl --namespace=kube-system get po
	echo kubectl --namespace=kube-system get po
}
kube::node_up() {
	kube::install_docker

	kube::pull_images

	kube::install_bin

	kubeadm join --token 997ea0.40e5c1218d0afc50 $@:6443
}
kube::tear_down() {
	systemctl stop kubelet.service
	docker ps -aq | xargs -I '{}' docker stop {}
	docker ps -aq | xargs -I '{}' docker rm {}
	docker images -q | xargs -I '{}' docker rmi -f {}
	df | grep /var/lib/kubelet | awk '{ print $6 }' | xargs -I '{}' umount {}
	rm -rf /var/lib/kubelet && rm -rf /etc/kubernetes/ && rm -rf /var/lib/etcd
	yum remove -y kubectl kubeadm kubelet kubernetes-cni
	rm -rf /var/lib/cni
	rm -rf /etc/cni/
	ip link del cni0
}

main() {
	case $1 in
	"-p" | "--prepare")
		kube::prepare
		;;
	"-m" | "--master")
		kube::master_up
		;;
	"-u" | "--upgrade")
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
	"-j" | "--join")
		shift
		kube::node_up $@
		;;
	"-d" | "--down")
		kube::tear_down
		;;
	*)
		echo "usage: $0 -m[--master] | -j[--join] token | -d[--down] | -u[--upgrade] node | -p[--prepare] "
		echo "       $0 prepare  to download k8s rpm and images "
		echo "       $0 master   to setup master "
		echo "       $0 upgrade  to upgrade master "
		echo "       $0 join	 to join master with token "
		echo "       $0 down     to tear all down ,inlude all data! so becarefull"
		echo "       unkown command $0 $@"
		;;
	esac
}

main $@
