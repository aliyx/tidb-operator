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
	sc := resp.StatusCode()
	if sc >= 200 && sc < 400 {
		logs.Info("metrics %s deleted, statusCode: %d", job, sc)
	}
	return fmt.Errorf("fail to delete metrics, statusCode %d: %s", resp.StatusCode(), resp.String())
}
