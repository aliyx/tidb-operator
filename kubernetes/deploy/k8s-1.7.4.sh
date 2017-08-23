#!/bin/bash
set -x
set -e
root=$(id -u)
if [ "$root" -ne 0 ]; then
	echo must run as root
	exit 1
fi

version=1.7.4
# docker private registries
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
		curl -sSL http://acs-public-mirror.oss-cn-hangzhou.aliyuncs.com/docker-engine/internet | sh -
		systemctl enable docker.service && systemctl start docker.service
	fi
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

	# download depentent images
	for imageName in ${images[@]}; do
		docker pull $imageName
		docker tag $imageName $registries/$imageName
		docker rmi $imageName
		docker push $registries/$imageName
	done
}

kube::pause_pod() {
	pause=$(docker images | grep gcr.io/google_containers/pause-amd64:3.0 | wc -l)
	if [ $pause -lt 1 ]; then
		docker pull registry.cn-hangzhou.aliyuncs.com/google-containers/pause-amd64:3.0
		docker tag registry.cn-hangzhou.aliyuncs.com/google-containers/pause-amd64:3.0 gcr.io/google_containers/pause-amd64:3.0
	fi
}

kube::install_bin() {
	yum install -y socat
	rm -rf /tmp/kube && mkdir -p /tmp/kube
	curl -sS -L $RPM_KUBEADM >/tmp/kube/kubeadm.rpm
	curl -sS -L $RPM_KUBECTL >/tmp/kube/kubectl.rpm
	curl -sS -L $RPM_KUBELET >/tmp/kube/kubelet.rpm
	curl -sS -L $RPM_KUBECNI >/tmp/kube/kube-cni.rpm

	rpm -ivh /tmp/kube/kubectl.rpm /tmp/kube/kubelet.rpm /tmp/kube/kube-cni.rpm /tmp/kube/kubeadm.rpm

	systemctl enable kubelet.service && systemctl start kubelet.service && rm -rf /etc/kubernetes
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
}

kube::node_upgrade() {
	rm -rf /tmp/kube && mkdir -p /tmp/kube
	curl -sS -L $RPM_KUBEADM >/tmp/kube/kubeadm.rpm
	curl -sS -L $RPM_KUBECTL >/tmp/kube/kubectl.rpm
	curl -sS -L $RPM_KUBELET >/tmp/kube/kubelet.rpm
	curl -sS -L $RPM_KUBECNI >/tmp/kube/kube-cni.rpm

	rpm -Uvh /tmp/kube/kubectl.rpm /tmp/kube/kubelet.rpm /tmp/kube/kube-cni.rpm /tmp/kube/kubeadm.rpm

	sudo systemctl restart kubelet
}

kube::master_up() {
	export KUBE_REPO_PREFIX=registry.cn-hangzhou.aliyuncs.com/google-containers \
		KUBE_HYPERKUBE_IMAGE=registry.cn-hangzhou.aliyuncs.com/google-containers/hyperkube-amd64:v1.5.1 \
		KUBE_DISCOVERY_IMAGE=registry.cn-hangzhou.aliyuncs.com/google-containers/kube-discovery-amd64:1.0 \
		KUBE_ETCD_IMAGE=registry.cn-hangzhou.aliyuncs.com/google-containers/etcd-amd64:3.0.4
	kube::install_docker

	kube::pause_pod

	kube::install_bin

	kubeadm init --pod-network-cidr="10.24.0.0/16"

	#install weave network
	#kubectl create -f $NET_WORKING_PLUGIN
	kubectl taint nodes --all dedicated-
	kubectl apply -f http://k8s.oss-cn-shanghai.aliyuncs.com/kube/kubernetes-dashboard1.5.0.yaml
	#show pods
	kubectl --namespace=kube-system get po
	echo kubectl --namespace=kube-system get po
}
kube::node_up() {
	export KUBE_REPO_PREFIX=registry.cn-hangzhou.aliyuncs.com/google-containers \
		KUBE_HYPERKUBE_IMAGE=registry.cn-hangzhou.aliyuncs.com/google-containers/hyperkube-amd64:v1.5.1 \
		KUBE_DISCOVERY_IMAGE=registry.cn-hangzhou.aliyuncs.com/google-containers/kube-discovery-amd64:1.0 \
		KUBE_ETCD_IMAGE=registry.cn-hangzhou.aliyuncs.com/google-containers/etcd-amd64:3.0.4
	kube::install_docker

	kube::pause_pod

	kube::install_bin

	kubeadm join $@
}
kube::tear_down() {
	systemctl stop kubelet.service
	docker ps -aq | xargs -I '{}' docker stop {}
	docker ps -aq | xargs -I '{}' docker rm {}
	df | grep /var/lib/kubelet | awk '{ print $6 }' | xargs -I '{}' umount {}
	rm -rf /var/lib/kubelet && rm -rf /etc/kubernetes/ && rm -rf /var/lib/etcd
	yum remove -y kubectl kubeadm kubelet kubernetes-cni
	rm -rf /var/lib/cni
	rm -rf /etc/cni/
	ip link del cni0
}

main() {
	case $1 in
	"p" | "prepare")
		kube::prepare
		;;
	"m" | "master")
		kube::master_up
		;;
	"u" | "upgrade")
		shift
		node=$@
		if [ "$node" == "master" ]; then
			kube::master_upgrade
		elif [ "$node" == "node" ]; then
			kube::node_upgrade
		else
			echo "unkown command $0 upgrade $@"
			exit 1
		fi
		;;
	"j" | "join")
		shift
		kube::node_up $@
		;;
	"d" | "down")
		kube::tear_down
		;;
	*)
		echo "usage: $0 m[master] | j[join] token | d[down] | u[upgrade] node | p[prepare] "
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
