import requests
import time
import logs
import json
import os

operator = {"op": "replace", "path": "/operator", "value": "syncMigrateStat"}


def sync(api, arr):
    if api == None:
        logs.critical("sync state api is nil")
    st = [operator]
    for op in arr:
        st.append(op)
    j = json.dumps(st).strip()
    logs.info("current migrate state: %s", j)
    for i in range(0, 60):
        try:
            r = requests.patch(api, data=j)
            if r.status_code != 200:
                logs.critical(
                    "can't synchronize the migration status and wait for 1 minute to try again: %s", r.reason)
            else:
                return
        except requests.exceptions.ConnectionError as ce:
            logs.error("can't connect to tidb-operator: %s", ce)
            time.sleep(60)
    logs.critical("retry 60 times after exiting")


def sync_stat(api, stat, reason=""):
    if api == None:
        logs.error("sync state api is nil")
        return
    patch = [
        {"op": "replace", "path": "/status/migrateState", "value": stat},
        {"op": "replace", "path": "/status/message", "value": reason}
    ]
    sync(api, patch)

# sync_stat('http://10.213.44.128:12808/tidb/api/v1/tidbs/006-xinyang1', 'Dumping')