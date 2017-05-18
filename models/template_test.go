package models

import "testing"

func Test_getResourceName(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "getName",
			args: args{k8sTikvPod},
			want: "tikv-{{cell}}-{{id}}",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getResourceName(tt.args.s); got != tt.want {
				t.Errorf("getResourceName() = %v, want %v", got, tt.want)
			}
		})
	}
}
