package models

import "github.com/tidwall/gjson"
import "fmt"
import "github.com/ghodss/yaml"

var k8sNs = `
kind: Namespace
apiVersion: v1
metadata:
  name: {{namespace}}
`

var k8sPdService = `
kind: Service
apiVersion: v1
metadata:
  name: pd-{{cell}}
  labels:
    component: pd
    cell: {{cell}}
    app: tidb
spec:
  ports:
    - port: 2379
  selector:
    component: pd
    cell: {{cell}}
    app: tidb
  type: NodePort
`
var k8sPdHeadlessService = `
kind: Service
apiVersion: v1
metadata:
  name: pd-{{cell}}-srv
  labels:
    component: pd
    cell: {{cell}}
    app: tidb
spec:
  clusterIP: None
  ports:
    - name: pd-server
      port: 2380
  selector:
    component: pd
    cell: {{cell}}
    app: tidb
`
var k8sPdRc = `
apiVersion: v1
kind: ReplicationController
metadata:
  name: pd-{{cell}}
spec:
  replicas: {{replicas}}
  template:
    metadata:
      labels:
        component: pd
        cell: {{cell}}
        app: tidb
    spec:
      volumes:
        - name: datadir
          emptyDir: {}
        - name: syslog
          hostPath: {path: /dev/log}
        - name: zone
          hostPath: {path: /etc/localtime}
      # 默认是30s
      terminationGracePeriodSeconds: 30
      containers:
        - name: pd
          image: {{registry}}/pd:{{version}}
          # imagePullPolicy: IfNotPresent
          volumeMounts:
            - name: datadir
              mountPath: /data
            - name: syslog
              mountPath: /dev/log
            - name: zone
              mountPath: /etc/localtime
          resources:
            limits:
              memory: "{{mem}}Mi"
              cpu: "{{cpu}}m"
          command:
            - bash
            - "-c"
            - |
              ipaddr=$(hostname -i)
              client_urls="http://0.0.0.0:2379"
              advertise_client_urls="http://$ipaddr:2379"
              peer_urls="http://0.0.0.0:2380"
              advertise_peer_urls="http://$ipaddr:2380"

              export CLIENT_URLS=$client_urls
              export ADVERTISE_CLIENT_URLS=$advertise_client_urls
              export PEER_URLS=$peer_urls
              export ADVERTISE_PEER_URLS=$advertise_peer_urls
              export PD_DATA_DIR=/data/$HOSTNAME

              # Gets the id of the pod in dns
              _pid=$(getid {{cell}})
              until [ $(echo -e "$_pid" | wc -l) -eq 1 ]; do
                echo "Cannt get pod id in SRV record, $(echo $_pid | tr '\n' ' ')"
                sleep 1
                _pid=$(getid {{cell}})
              done
              export PD_NAME=$_pid
              # Save PD_NAME to the local file, prestop hook will be used to delete member
              echo $PD_NAME > pod
              echo -e "\033[31mCurrent pod id is $PD_NAME\033[0m"

              if [ -d $PD_DATA_DIR ]; then
                echo "Resuming with existing data dir:$PD_DATA_DIR"
                pd-server \
                --name="$PD_NAME" \
                --data-dir="$PD_DATA_DIR" \
                --client-urls="$CLIENT_URLS" \
                --advertise-client-urls="$ADVERTISE_CLIENT_URLS" \
                --peer-urls="$PEER_URLS" \
                --advertise-peer-urls="$ADVERTISE_PEER_URLS" \
                --join="http://pd-{{cell}}:2379" \
                --config="/etc/pd/config.toml"
              else
                echo "First run for this member"
                # If there's already a functioning cluster, join it.
                # Get the leader of the cluster, try again 3 times (at the scale will lead to can not access), each time no more than 3 seconds
                resp=''
                for i in $(seq 1 3)  
                do  
                  resp=$(curl --connect-timeout 1 --write-out %{http_code} --silent --output /dev/null http://pd-{{cell}}:2379/pd/api/v1/leader)
                  if [ $resp == 200 ]; then
                    break
                  fi
                  sleep 1   
                done  
                if [ $resp == 200 ]; then
                  echo "Joining existing cluster:http://pd-{{cell}}:2379"
                  pd-server \
                  --name="$PD_NAME" \
                  --data-dir="$PD_DATA_DIR" \
                  --client-urls="$CLIENT_URLS" \
                  --advertise-client-urls="$ADVERTISE_CLIENT_URLS" \
                  --peer-urls="$PEER_URLS" \
                  --advertise-peer-urls="$ADVERTISE_PEER_URLS" \
                  --join="http://pd-{{cell}}:2379" \
                  --config="/etc/pd/config.toml"
                else
                  # Join failed. Assume we're trying to bootstrap.
                  echo "Create a new pd cluster"
                  # First wait for the desired number of replicas to show up.
                  echo "Waiting for {{replicas}} replicas in SRV record for pd-{{cell}}-srv..."
                  until [ $(getpods {{cell}} | wc -l) -eq {{replicas}} ]; do
                    echo "[$(date)] waiting for {{replicas}} entries in SRV record for pd-{{cell}}-srv"
                    sleep 1
                  done

                  urls=$(getpods {{cell}} | tr '\n' ',')
                  urls=${urls%,}
                  echo "Initial-cluster:$urls"

                  # Now run
                  pd-server \
                  --name="$PD_NAME" \
                  --data-dir="$PD_DATA_DIR" \
                  --client-urls="$CLIENT_URLS" \
                  --advertise-client-urls="$ADVERTISE_CLIENT_URLS" \
                  --peer-urls="$PEER_URLS" \
                  --advertise-peer-urls="$ADVERTISE_PEER_URLS" \
                  --initial-cluster=$urls \
                  --config="/etc/pd/config.toml"
                fi
              fi
          lifecycle:
            preStop:
              exec:
                command:
                  - bash
                  - "-c"
                  - |
                    PD_NAME=$(cat pod)
                    # clear
                    resp=''
                    for i in $(seq 1 3)  
                    do  
                      resp=$(curl -X DELETE --write-out %{http_code} --silent --output /dev/null http://pd-{{cell}}:2379/pd/api/v1/members/$PD_NAME)
                      if [ $resp == 200 ]
                      then
                        break
                      fi 
                      sleep 1  
                    done
                    if [ $resp == 200 ]
                    then
                      echo 'Delete pd "$PD_NAME" success'
                    fi             
`

var k8sTikvPod = `
apiVersion: v1
kind: Pod
metadata:
  name: tikv-{{cell}}-{{id}}
  labels:
    app: tidb
    cell: {{cell}}
    component: tikv
    id: "{{id}}"
spec:
  affinity:
    # 防止数据丢失,设置亲缘的目的是该pod只能在该node上更新
    podAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
      - labelSelector:
          matchExpressions:
          - key: id
            operator: In
            values:
            - "{{id}}"
        topologyKey: kubernetes.io/hostname
    # PD 和 TiKV 实例，建议每个实例单独部署一个硬盘，避免 IO 冲突，影响性能
    podAntiAffinity:
      preferredDuringSchedulingIgnoredDuringExecution:
      - weight: 80
        podAffinityTerm:
          labelSelector:
            matchExpressions:
            - key: component
              operator: In
              values:
              - "pd"
          topologyKey: kubernetes.io/hostname
  volumes:
    - name: syslog
      hostPath: {path: /dev/log}
    - name: datadir
      {{tidbdata_volume}}
    - name: zone
      hostPath: {path: /etc/localtime}
  terminationGracePeriodSeconds: 30
  containers:
  - name: tikv
    image: {{registry}}/tikv:{{version}}
    resources:
      # 初始化requests和limits相同的值，是为了防止memory超过requests时，node资源不足，导致该pod被重新安排到其它node
      requests:
        memory: "{{mem}}Mi"
        cpu: "{{cpu}}m"
      limits:
        memory: "{{mem}}Mi"
        cpu: "{{cpu}}m"
    ports:
    - containerPort: 20160
    volumeMounts:
      - name: datadir
        mountPath: /data
      - name: zone
        mountPath: /etc/localtime
    command:
      - bash
      - "-c"
      - |
        /tikv-server \
        --store="/data/tikv-{{cell}}-{{id}}" \
        --addr="0.0.0.0:20160" \
        --capacity={{capacity}}GB \
        --advertise-addr="$POD_IP:20160" \
        --pd="pd-{{cell}}:2379" \
        --config="/etc/tikv/config.toml"
    env: 
      - name: POD_IP
        valueFrom:
          fieldRef:
            fieldPath: status.podIP
    lifecycle:
      preStop:
        exec:
          command:
            - bash
            - "-c"
            - |
              rm -rf /data/tikv-{{cell}}-{{id}}
`

var k8sTidbService = `
kind: Service
apiVersion: v1
metadata:
  name: tidb-{{cell}}
  labels:
    component: tidb
    cell: {{cell}}
    app: tidb
spec:
  ports:
    - name: mysql
      port: 4000
    - name: web
      port: 10080
  selector:
    component: tidb
    cell: {{cell}}
    app: tidb
  type: NodePort
`

var k8sTidbRc = `
kind: ReplicationController
apiVersion: v1
metadata:
  name: tidb-{{cell}}
spec:
  replicas: {{replicas}}
  template:
    metadata:
      labels:
        component: tidb
        cell: {{cell}}
        app: tidb
    spec:
      affinity:
        # TiDB 和 TiKV 实例，建议分开部署，以免竞争 CPU 资源，影响性能
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
          - weight: 80
            podAffinityTerm:
              labelSelector:
                matchExpressions:
                - key: component
                  operator: In
                  values:
                  - "tikv"
              topologyKey: kubernetes.io/hostname
      volumes:
        - name: syslog
          hostPath: {path: /dev/log}
        - name: zone
          hostPath: {path: /etc/localtime}
      terminationGracePeriodSeconds: 10
      containers:
        - name: tidb
          image: {{registry}}/tidb:{{version}}
          livenessProbe:
            httpGet:
              path: /status
              port: 10080
            initialDelaySeconds: 30
            timeoutSeconds: 5
          volumeMounts:
            - name: syslog
              mountPath: /dev/log
            - name: zone
              mountPath: /etc/localtime
          resources:
            limits:
              memory: "{{mem}}Mi"
              cpu: "{{cpu}}m"
          command: ["/tidb-server"]
          args: 
            - -P=4000
            - --store=tikv
            - --path=pd-{{cell}}:2379
            - --metrics-addr=prom-gateway:9091
            - --metrics-interval=15
`
var k8sMigrate = `
apiVersion: v1
kind: Pod
metadata:
  name: migration-{{cell}}
  labels:
    app: tidb
    cell: {{cell}}
    component: migration
spec:
  volumes:
    - name: syslog
      hostPath: {path: /dev/log}
    - name: zone
      hostPath: {path: /etc/localtime}
  terminationGracePeriodSeconds: 10
  containers:
  - name: migration
    image: {{image}}
    resources:
      limits:
        cpu: "200m"
        memory: "512Mi"
    command:
      - bash
      - "-c"
      - |
        migrate {{sync}}
        while true; do
          echo "Waiting for the pod to closed"
          sleep 10
        done
    volumeMounts:
      - name: zone
        mountPath: /etc/localtime
    env: 
    - name: M_S_HOST
      value: "{{sh}}"
    - name: M_S_PORT
      value: "{{sP}}"
    - name: M_S_USER
      value: "{{su}}"
    - name: M_S_PASSWORD
      value: "{{sp}}"
    - name: M_S_DB
      value: "{{db}}"
    - name: M_D_HOST
      value: "{{dh}}"
    - name: M_D_PORT
      value: "{{dP}}"
    - name: M_D_USER
      value: "{{duser}}"
    - name: M_D_PASSWORD
      value: "{{dp}}"
    - name: M_STAT_API
      value: "{{api}}"
`

func getResourceName(s string) string {
	j, _ := yaml.YAMLToJSON([]byte(s))
	return fmt.Sprintf("%s", gjson.Get(string(j), "metadata.name"))
}
