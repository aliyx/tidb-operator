package models

import (
	"testing"

	"fmt"

	"github.com/tidwall/gjson"
)

var pdSt = `
{
  "count": 3,
  "stores": [
    {
      "store": {
        "id": 1,
        "address": "10.34.0.25:20160",
        "state": 0,
        "state_name": "Up"
      },
      "status": {
        "store_id": 1,
        "capacity": "537 GB",
        "available": "514 GB",
        "leader_count": 1,
        "region_count": 1,
        "sending_snap_count": 0,
        "receiving_snap_count": 0,
        "applying_snap_count": 0,
        "is_busy": false,
        "start_ts": "2017-04-12T03:55:04Z",
        "last_heartbeat_ts": "2017-04-12T05:27:59.267461067Z",
        "uptime": "1h32m55.267461067s"
      }
    },
    {
      "store": {
        "id": 4,
        "address": "10.40.0.19:20160",
        "state": 0,
        "state_name": "Up"
      },
      "status": {
        "store_id": 4,
        "capacity": "537 GB",
        "available": "514 GB",
        "leader_count": 0,
        "region_count": 0,
        "sending_snap_count": 0,
        "receiving_snap_count": 0,
        "applying_snap_count": 0,
        "is_busy": false,
        "start_ts": "2017-04-12T05:28:34Z",
        "last_heartbeat_ts": "2017-04-12T05:29:15.989162869Z",
        "uptime": "41.989162869s"
      }
    },
    {
      "store": {
        "id": 7,
        "address": "10.40.0.25:20160",
        "state": 0,
        "state_name": "Up"
      },
      "status": {
        "store_id": 7,
        "capacity": "537 GB",
        "available": "514 GB",
        "leader_count": 0,
        "region_count": 0,
        "sending_snap_count": 0,
        "receiving_snap_count": 0,
        "applying_snap_count": 0,
        "is_busy": false,
        "start_ts": "2017-04-12T05:31:31Z",
        "last_heartbeat_ts": "2017-04-12T05:32:42.385571871Z",
        "uptime": "1m11.385571871s"
      }
    }
  ]
}
`

func TestTikv_waitForComplete(t *testing.T) {
	ret := gjson.Get(pdSt, "stores.#[store.state==0]#.status.store_id")
	fmt.Printf("%v \n", ret)

	// result := gjson.Get(pdSt, "stores.#.store")
	// result.ForEach(func(key, value gjson.Result) bool {
	// 	value.Get("state")
	// 	fmt.Printf("key:%v value:%v \n", key, value.Get("state"))
	// 	return true
	// })
}
