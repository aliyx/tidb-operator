package k8sutil

import (
	"os"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	masterHost = "http://10.213.44.128:10218"
	os.Exit(m.Run())
}

func Test_delDeployment(t *testing.T) {
	type args struct {
		name    string
		cascade bool
		timeout time.Duration
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "delDeployment",
			args: args{
				name:    "tikv-test-",
				cascade: false,
				timeout: 3 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "cascadeDelDeployment",
			args: args{
				name:    "tikv-test-",
				cascade: true,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := DelDeployment(tt.args.name, tt.args.cascade); (err != nil) != tt.wantErr {
				t.Errorf("delDeployment() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_getServiceProperties(t *testing.T) {
	str, err := GetServiceProperties("tidb-cqjtest0", `{{index (index .spec.ports 0) "nodePort"}}:{{index (index .spec.ports 1) "nodePort"}}`)
	if err != nil {
		t.Errorf("%v", err)
	}
	println(str)
}

func Test_listPodNames(t *testing.T) {
	type args struct {
		cell      string
		component string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "listPodNames",
			args: args{
				cell:      "test",
				component: "pd",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ListPodNames(tt.args.cell, tt.args.component)
			if (err != nil) != tt.wantErr {
				t.Errorf("listPodNames() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func Test_isPodOk(t *testing.T) {
	ret, ok := isPodOk("tikv-test-3")
	if !ok {
		t.Errorf("%v", ret)
	}
}

func Test_WaitComponentRuning(t *testing.T) {
	if err := WaitComponentRuning(10*time.Second, "test", "pd"); err != nil {
		t.Errorf("%v", err)
	}
}
