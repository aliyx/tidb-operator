# 构建kubernetes集群

局域网中安装kubernetes集群

## Install

先上传k8s需要的4个rpm包(*kubectl-1.6.1-0.x86_64.rpm,*kubeadm-1.6.1-0.x86_64.rpm,*kubelet-1.6.1-0.x86_64.rpm,*kubernetes-cni-0.5.1-0.x86_64.rpm)到需要安装的服务器，再执行以下脚本:

```bash
./install.sh . # .表示rpm的位置
```

## Master

在指定的master节点上执行：

```bash
./kube-master.sh
```

## Node

在指定的node节点上执行：

```bash
./kube-node.sh ip # ip指的是master节点的ip地址
```

## 设置proxy and prometheus

```bash
# add
kubectl label nodes name node-role.proxy=
kubectl label nodes name node-role.prometheus=
# or
kubectl label nodes name --overwrite  node-role=proxy
kubectl label nodes name --overwrite  node-role=prometheus

#remove
kubectl label nodes name node-role.proxy-
kubectl label nodes name node-role.prometheus-
# or
kubectl label nodes name --overwrite  node-role-
kubectl label nodes name --overwrite  node-role-
```

## 访问

k8s apiserver对外暴露的port是10218, dashboard的port是12801，pod端口范围是12800-14999，由于公司的出口端口范围是10000-14999.
访问dashboard：masterip:12801