package models

import "github.com/tidwall/gjson"
import "fmt"
import "github.com/ghodss/yaml"

var pdServiceYaml = `
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
    - name: client
      port: 2379
  selector:
    component: pd
    cell: {{cell}}
    app: tidb
  type: NodePort
`

var pdHeadlessServiceYaml = `
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

var pdPodYaml = `
apiVersion: v1
kind: Pod
metadata:
  name: pd-{{cell}}-{{id}}
  labels:
    component: pd
    cell: {{cell}}
    app: tidb
spec:
  affinity:
    nodeAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        nodeSelectorTerms:
        - matchExpressions:
          - key: node-role
            operator: NotIn
            values:
            - prometheus
  volumes:
  - name: tidb-data
    {{tidbdata_volume}}
  # default is 30s
  terminationGracePeriodSeconds: 5
  restartPolicy: Always
  # DNS A record: [m.Name].[clusterName].Namespace.svc.cluster.local.
  # For example, pd-test-001 in default namesapce will have DNS name
  # 'pd-test-001.test.default.svc.cluster.local'.
  hostname: pd-{{cell}}-{{id}}
  subdomain: pd-{{cell}}-srv
  containers:
    - name: pd
      image: 10.209.224.13:10500/ffan/rds/pd:{{version}}
      # imagePullPolicy: IfNotPresent
      volumeMounts:
      - name: tidb-data
        mountPath: /var/pd
      resources:
        limits:
          memory: {{mem}}Mi
          cpu: {{cpu}}m
      env: 
      - name: M_INTERVAL
        value: "15"
      command:
        - bash
        - "-c"
        - |
          client_urls="http://0.0.0.0:2379"
          # FQDN
          advertise_client_urls="http://pd-{{cell}}-{{id}}.pd-{{cell}}-srv.{{namespace}}.svc.cluster.local:2379"
          peer_urls="http://0.0.0.0:2380"
          advertise_peer_urls="http://pd-{{cell}}-{{id}}.pd-{{cell}}-srv.{{namespace}}.svc.cluster.local:2380"

          export PD_NAME=$HOSTNAME
          export PD_DATA_DIR=/var/pd/$HOSTNAME/data

          export CLIENT_URLS=$client_urls
          export ADVERTISE_CLIENT_URLS=$advertise_client_urls
          export PEER_URLS=$peer_urls
          export ADVERTISE_PEER_URLS=$advertise_peer_urls

          # set prometheus
          sed -i -e 's/{m-job}/{{cell}}/' /etc/pd/config.toml
          sed -i -e 's/{m-interval}/'"$M_INTERVAL"'/' /etc/pd/config.toml

          if [ -d $PD_DATA_DIR ]; then
            echo "Resuming with existing data dir:$PD_DATA_DIR"
          else
            echo "First run for this member"
            # First wait for the desired number of replicas to show up.
            echo "Waiting for {{replicas}} replicas in SRV record for {{cell}}..."
            until [ $(getpods {{cell}} | wc -l) -eq {{replicas}} ]; do
              echo "[$(date)] waiting for {{replicas}} entries in SRV record for {{cell}}"
              sleep 1
            done
          fi

          urls=""
          for id in {1..{{replicas}}}; do
            id=$(printf "%03d\n" $id)
            urls+="pd-{{cell}}-${id}=http://pd-{{cell}}-${id}.pd-{{cell}}-srv.{{namespace}}.svc.cluster.local:2380,"
          done
          urls=${urls%,}
          echo "Initial-cluster:$urls"

          pd-server \
          --name="$PD_NAME" \
          --data-dir="$PD_DATA_DIR" \
          --client-urls="$CLIENT_URLS" \
          --advertise-client-urls="$ADVERTISE_CLIENT_URLS" \
          --peer-urls="$PEER_URLS" \
          --advertise-peer-urls="$ADVERTISE_PEER_URLS" \
          --initial-cluster=$urls \
          --config="/etc/pd/config.toml"
      lifecycle:
        preStop:
          exec:
            command:
              - bash
              - "-c"
              - |
                # delete prometheus metrics
                curl -X DELETE http://prom-gateway:9091/metrics/job/{{cell}}/instance/$HOSTNAME

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

var tikvPodYaml = `
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

var tidbServiceYaml = `
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

var tidbRcYaml = `
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

var mysqlMigraeYaml = `
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
