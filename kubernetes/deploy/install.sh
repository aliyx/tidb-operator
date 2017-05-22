#!/bin/bash

# set -x
if (( $EUID != 0 )); then
    echo "Please run as root"
    exit
fi

# docker private registries
registries=10.209.224.13:10500

# First through the proxy pull image and push to docker server
#===========================================================================1

# sudo cat > /etc/systemd/system/docker.service.d/http-proxy.conf <<-EOF
# [Service]
# Environment="http_proxy=http://192.168.14.1:1080/"
# EOF

# sudo systemctl daemon-reload
# sudo systemctl restart docker

images=(gcr.io/google_containers/kube-apiserver-amd64:v1.6.0 gcr.io/google_containers/kube-controller-manager-amd64:v1.6.0 gcr.io/google_containers/kube-scheduler-amd64:v1.6.0  gcr.io/google_containers/kube-proxy-amd64:v1.6.0 gcr.io/google_containers/etcd-amd64:3.0.17 gcr.io/google_containers/pause-amd64:3.0 gcr.io/google_containers/k8s-dns-sidecar-amd64:1.14.1 gcr.io/google_containers/k8s-dns-kube-dns-amd64:1.14.1  gcr.io/google_containers/k8s-dns-dnsmasq-nanny-amd64:1.14.1 gcr.io/google_containers/kubernetes-dashboard-amd64:v1.6.0)
# for imageName in ${images[@]} ; do
#   docker pull $imageName
#   docker tag $imageName $registries/$imageName
#   docker rmi $imageName
# done

# push
# for imageName in ${images[@]} ; do
#   docker push $registries/$imageName
# done


# As GFW can not connect google, you need to download rpm to the local upload to the server and then install
#===========================================================================2
# kubernetes version 1.5.1

# sudo cat > /etc/yum.repos.d/kubernetes.repo <<-EOF
# [kubernetes]
# name=Kubernetes
# baseurl=http://yum.kubernetes.io/repos/kubernetes-el7-x86_64
# enabled=1
# gpgcheck=1
# repo_gpgcheck=1
# gpgkey=https://packages.cloud.google.com/yum/doc/yum-key.gpg
#        https://packages.cloud.google.com/yum/doc/rpm-package-key.gpg
# EOF

# sudo yumdownloader kubelet kubeadm kubectl kubernetes-cni
# sudo yum install -y lrzsz


# Install docker and kubernetes
#===========================================================================3
k8sRpmDir=$1

if [ ! -d "$k8sRpmDir" ]; then
  echo "Please specify k8s rpm package directory: ./instal.sh ."
  exit 1
fi

# Check k8s rpm
# http://stackoverflow.com/questions/15305556/shell-script-to-check-if-file-exists
(
  shopt -s nullglob
  files=($k8sRpmDir/*kube*.rpm)
  if [[ "${#files[@]}" -eq 4 ]] ; then
    echo "Checking kubenetes rpm ...................................ok"
  else
    echo "Checking kubenetes rpm .................................fail"
    exit 1
  fi
)

kubeadm reset
# Delete all containers
docker rm -f $(docker ps -a -q)
# Delete all images
docker rmi -f $(docker images -q)
echo "Remove all images if has............ ......................ok"

# Erase old k8s
rpm -e kubectl kubelet kubeadm kubernetes-cni
yum remove -y kubectl kubelet kubeadm kubernetes-cni
# Remove old docker
yum remove -y ebtables docker docker-common container-selinux docker-selinux docker-engine socat
echo "Cleaning old ...............................................ok"

# Install
# check docker version
# yum list|grep docker
# The company network is not set in repo       
# sudo cat > /etc/yum.repos.d/docker.repo <<-EOF
# [dockerrepo]
# name=Docker Repository
# baseurl=https://yum.dockerproject.org/repo/main/centos/7
# enabled=1
# gpgcheck=1
# gpgkey=https://yum.dockerproject.org/gpg
# EOF
#sudo yum update -y && yum upgrade -y
echo "Update centos ..............................................ok"

# Disabling SELinux by running setenforce 0 is required in order to allow containers to access the host filesystem
setenforce 0

# Docker v1.12 is recommended
yum install -y ebtables docker-engine-1.12.6
echo "Inatall docker v1.12.6 .....................................ok"

yum install -y socat
rpm -ivh $k8sRpmDir/*kube*.rpm
echo "Inatall kubernetes .........................................ok"

set -e

# https -> http
if [ ! -d "/etc/docker" ]; then
  sudo mkdir /etc/docker
fi
tee > /etc/docker/daemon.json <<- EOF
{ "insecure-registries":["$registries"] }
EOF

# start
systemctl enable docker && sudo systemctl start docker
echo "Start docker............ ...................................ok"

# pull
for imageName in ${images[@]} ; do
  docker pull $registries/$imageName
  docker tag $registries/$imageName $imageName
  docker rmi $registries/$imageName
done
echo "Pull kubernetes images............ .........................ok"

# initialize
systemctl enable kubelet && sudo systemctl start kubelet
echo "Rest k8s and start kubelet........... ......................ok"

echo 1 > /proc/sys/net/bridge/bridge-nf-call-iptables

# check SELinux
echo $(sestatus)
# vi /etc/sysconfig/selinux
# SELINUX=disabled
# reboot

tee > /etc/profile.d/k8s.sh <<- EOF
alias kubectl='kubectl --server=127.0.0.1:10218'
EOF

echo "Install kubernets.....................................finished"