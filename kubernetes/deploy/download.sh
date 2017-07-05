#!/bin/bash

set -ex

# local proxy
PROXY="http://192.168.14.1:1080/"

# docker private registries
registries=10.209.224.13:10500

# set proxy
sudo cat > /etc/systemd/system/docker.service.d/http-proxy.conf <<-EOF
[Service]
Environment="http_proxy=$PROXY"
EOF

sudo systemctl daemon-reload
sudo systemctl restart docker

# get all images
images=(gcr.io/google_containers/kube-apiserver-amd64:v1.6.0 gcr.io/google_containers/kube-controller-manager-amd64:v1.6.0 gcr.io/google_containers/kube-scheduler-amd64:v1.6.0  gcr.io/google_containers/kube-proxy-amd64:v1.6.0 gcr.io/google_containers/etcd-amd64:3.0.17 gcr.io/google_containers/pause-amd64:3.0 gcr.io/google_containers/k8s-dns-sidecar-amd64:1.14.1 gcr.io/google_containers/k8s-dns-kube-dns-amd64:1.14.1  gcr.io/google_containers/k8s-dns-dnsmasq-nanny-amd64:1.14.1 gcr.io/google_containers/kubernetes-dashboard-amd64:v1.6.0)
for imageName in ${images[@]} ; do
  docker pull $imageName
  docker tag $imageName $registries/$imageName
  docker rmi $imageName
  docker push $registries/$imageName
done


# check kubeadm rpm
yum info kubeadm

# As GFW can not connect google, you need to download rpm to the local upload to the server and then install
# or https://github.com/kubernetes/release manually compile the rpm package
# kubernetes version 1.6.0

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

sudo yumdownloader kubelet kubeadm kubectl kubernetes-cni
