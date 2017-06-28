package prometheusutil

import (
	"fmt"

	"github.com/astaxie/beego/logs"
	"gopkg.in/resty.v0"
)

const (
	jobAPIDelete = "http://prom-gateway:9091/metrics/job/%s"
)

// DeleteMetricsByJob delete all metrics grouped by job only
func DeleteMetricsByJob(job string) error {
	url := fmt.Sprintf(jobAPIDelete, job)
	logs.Info("delete metrics by job: %s", url)
	resp, err := resty.R().Delete(url)
	if err != nil {
		return err
	}
	if resp.StatusCode() != 200 {
		return fmt.Errorf("fail to delete metrics: %v", resp)
	}
	return nil
}
