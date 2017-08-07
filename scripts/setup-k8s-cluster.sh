#!/bin/bash
set -x

# Install kubernetes
curl -LO https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl && chmod +x ./kubectl && sudo mv ./kubectl /usr/local/bin/kubectl


# Note: takes some time for the http server to pop up :)
# MINIMUM 15 seconds
echo "
##########
STARTING KUBERNETES
##########
"
if [ ! -f /usr/bin/kubectl ] && [ ! -f /usr/local/bin/kubectl ]; then
  echo "No kubectl bin exists? Please install."
  return 1
fi

# Uses https://github.com/kubernetes/minikube/tree/master/deploy/docker
# In which we have to git clone and create the image..
# https://github.com/kubernetes/minikube
# Thus we are using a public docker image

IMAGE=calpicow/localkube-image:v1.5.3
sudo docker run -d \
    --volume=/:/rootfs:ro \
    --volume=/sys:/sys:rw \
    --volume=/var/lib/docker:/var/lib/docker:rw \
    --volume=/var/lib/kubelet:/var/lib/kubelet:rw \
    --volume=/var/run:/var/run:rw \
    --net=host \
    --pid=host \
    --privileged \
    --name=minikube \
    $IMAGE \
    /localkube start \
    --apiserver-insecure-address=0.0.0.0 \
    --apiserver-insecure-port=8080 \
    --logtostderr=true \
    --containerized

until curl 127.0.0.1:8080 &>/dev/null;
do
    echo ...
    sleep 1
done

# Set the appropriate .kube/config configuration
kubectl config set-cluster dev --server=http://localhost:8080
kubectl config set-context dev --cluster=dev --user=default
kubectl config use-context dev
kubectl config set-credentials default --token=foobar
kubectl get nodes

# Debug info:
# cat ~/.kube/config

# Delay due to CI being a bit too slow when first starting k8s
sleep 5
