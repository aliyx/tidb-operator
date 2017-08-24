#!/bin/bash










# start docker
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

echo "Sync os time"
# sync system time: ntp.api.bz is china
ntpdate -u  10.209.100.2
# write system time to CMOS
clock -w

echo "Install kubernets...finished"