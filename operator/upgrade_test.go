package operator

import "testing"

import "fmt"

func Test_upgradeOne(t *testing.T) {
	type args struct {
		name  string
		image string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		name:    "pd-006-test-001",
		image:   fmt.Sprintf("pd-%s-%03d", "006-test", 1),
		wantErr: false,
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := upgradeOne(tt.args.name, tt.args.image); (err != nil) != tt.wantErr {
				t.Errorf("upgradeOne() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
