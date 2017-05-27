#!/bin/bash

script_root=`dirname "${BASH_SOURCE}"`
source $script_root/env.sh

# set -x
if (( $EUID != 0 )); then
    echo "Please run as root"
    exit
fi

images=(gcr.io/google_containers/kube-apiserver-amd64:v1.6.0 gcr.io/google_containers/kube-controller-manager-amd64:v1.6.0 gcr.io/google_containers/kube-scheduler-amd64:v1.6.0  gcr.io/google_containers/kube-proxy-amd64:v1.6.0 gcr.io/google_containers/etcd-amd64:3.0.17 gcr.io/google_containers/pause-amd64:3.0 gcr.io/google_containers/k8s-dns-sidecar-amd64:1.14.1 gcr.io/google_containers/k8s-dns-kube-dns-amd64:1.14.1  gcr.io/google_containers/k8s-dns-dnsmasq-nanny-amd64:1.14.1 gcr.io/google_containers/kubernetes-dashboard-amd64:v1.6.0)

# Install docker and kubernetes
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
    echo "Checking kubenetes rpm...ok"
  else
    echo "Checking kubenetes rpm...fail"
    exit 1
  fi
)

kubeadm reset
# Delete all containers
docker rm -f $(docker ps -a -q)
# Delete all images
docker rmi -f $(docker images -q)
echo "Remove all images if exist...ok"

# Erase old k8s
rpm -e kubectl kubelet kubeadm kubernetes-cni
yum remove -y kubectl kubelet kubeadm kubernetes-cni
# Remove old docker
yum remove -y ebtables docker docker-common container-selinux docker-selinux docker-engine socat
echo "Cleaning old...ok"

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
echo "Update centos...ok"

# Disabling SELinux by running setenforce 0 is required in order to allow containers to access the host filesystem
setenforce 0

# Docker v1.12 is recommended
yum install -y ebtables docker-engine-1.12.6
echo "Inatall docker v1.12.6...ok"

yum install -y socat
rpm -ivh $k8sRpmDir/*kube*.rpm
echo "Inatall kubernetes...ok"

set -e

# Set access to the docker registry protocol: https -> http
if [ ! -d "/etc/docker" ]; then
  sudo mkdir /etc/docker
fi
tee > /etc/docker/daemon.json <<- EOF
{ "insecure-registries":["$registries"] }
EOF

# start docker
systemctl enable docker && sudo systemctl start docker
echo "Start docker...ok"

# Pull the base image of kubernetes 
for imageName in ${images[@]} ; do
  docker pull $registries/$imageName
  docker tag $registries/$imageName $imageName
  docker rmi $registries/$imageName
done
echo "Pull kubernetes images...ok"

# initialize
systemctl enable kubelet && sudo systemctl start kubelet
echo "Reset k8s and start kubelet...ok"

echo 1 > /proc/sys/net/bridge/bridge-nf-call-iptables

# check SELinux
echo $(sestatus)
# vi /etc/sysconfig/selinux
# SELINUX=disabled
# reboot

tee > /etc/profile.d/k8s.sh <<- EOF
alias kubectl='kubectl --server=127.0.0.1:10218'
EOF

echo "Install kubernets...finished"