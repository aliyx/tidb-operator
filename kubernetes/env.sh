# This is an include file used by the other scripts in this directory.

# Most clusters will just be accessed with 'kubectl' on $PATH.
# However, some might require a different command. For example, GKE required
# KUBECTL='gcloud container kubectl' for a while. Now that most of our
# use cases just need KUBECTL=kubectl, we'll make that the default.
KUBECTL=${KUBECTL:-kubectl}

# Kubernetes api address for $KUBECTL 
# The default value is 127.0.0.1:8080
# When the Kubernetes api server is not local, We can easily access the api by edit KUBERNETES_API_SERVER's value
KUBERNETES_API_SERVER=${KUBERNETES_API_SERVER:-'127.0.0.1:10218'}

# Kubernetes namespace for tidb and components.
NS=${NS:-'default'}

# Docker registry for rds images
REGISTRY=${REGISTRY-'10.209.224.13:10500/ffan/rds'}

# Kubernetes options config
KUBECTL_OPTIONS="--namespace=$NS --server=$KUBERNETES_API_SERVER"

# CELLS should be a comma separated list of cells
CELL=${CELL:-'test'}

DATA_VOLUME=${DATA_VOLUME:-'/tmp/tidb'}

# image version
VERSION=${VERSION:-'latest'}