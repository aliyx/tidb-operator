package operator

import "testing"

import "fmt"

func Test_upgradeOne(t *testing.T) {
	type args struct {
		name    string
		image   string
		version string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "upgrade",
			args: args{
				name:    "pd-006-test-001",
				image:   fmt.Sprintf("%s/pd:%s", ImageRegistry, "latest"),
				version: "latest",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := upgradeOne(tt.args.name, tt.args.image, tt.args.version); (err != nil) != tt.wantErr {
				t.Errorf("upgradeOne() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
