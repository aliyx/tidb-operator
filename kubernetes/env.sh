# This is an include file used by the other scripts in this directory.

# Most clusters will just be accessed with 'kubectl' on $PATH.
# However, some might require a different command. For example, GKE required
# KUBECTL='gcloud container kubectl' for a while. Now that most of our
# use cases just need KUBECTL=kubectl, we'll make that the default.
KUBECTL=${KUBECTL:-kubectl}

# Kubernetes api address for $KUBECTL 
# The default value is 127.0.0.1:8080
# When the Kubernetes api server is not local, We can easily access the api by edit KUBERNETES_API_SERVER's value
KUBERNETES_API_SERVER=${KUBERNETES_API_SERVER:-'127.0.0.1:8080'}

# Kubernetes namespace for tidb and components.
NS=${NS:-'default'}

# Kubernetes options config
KUBECTL_OPTIONS="--namespace=$NS --server=$KUBERNETES_API_SERVER"

# Tidb cluster name
CELL=${CELL:-'test'}

# Docker registry for rds images
REGISTRY=${REGISTRY-'10.209.224.13:10500/ffan/rds'}

# Docker image version
VERSION=${VERSION:-'latest'}

# The volume of pod host path
DATA_VOLUME=${DATA_VOLUME:-'/data'}

# The prefix of pod mount path
MOUNT=${MOUNT:-''}

#---------------------------------------------------------------------

MAX_TASK_WAIT_RETRIES=${MAX_TASK_WAIT_RETRIES:-300}

function update_spinner_value () {
  spinner='-\|/'
  cur_spinner=${spinner:$(($1%${#spinner})):1}
}

function wait_for_complete () {
  url=$1
  response=0
  counter=0

  while [ $counter -lt $MAX_TASK_WAIT_RETRIES ]; do
    response=$(curl --write-out %{http_code} --silent --output /dev/null $url)
    echo -en "\r$url: waiting for return http_code:200..."
    if [ $response -eq 200 ]
    then
      echo Complete
      return 0
    fi
    update_spinner_value $counter
    echo -n $cur_spinner
    let counter=counter+1
    sleep 1
  done
  echo Timed out
  return -1
}

function wait_for_running_tasks () {
  # This function waits for pods to be in the "Running" state
  # 1. task_name: Name that the desired task begins with
  # 2. num_tasks: Number of tasks to wait for
  # Returns:
  #   0 if successful, -1 if timed out
  task_name=$1
  num_tasks=$2
  counter=0

  echo "Waiting for ${num_tasks}x $task_name to enter state Running"

  while [ $counter -lt $MAX_TASK_WAIT_RETRIES ]; do
    # Get status column of pods with name starting with $task_name,
    # count how many are in state Running
    num_running=`$KUBECTL $KUBECTL_OPTIONS get pods | grep ^$task_name | grep Running | wc -l`

    echo -en "\r$task_name: $num_running out of $num_tasks in state Running..."
    if [ $num_running -eq $num_tasks ]
    then
      echo Complete
      return 0
    fi
    update_spinner_value $counter
    echo -n $cur_spinner
    let counter=counter+1
    sleep 1
  done
  echo Timed out
  return -1
}