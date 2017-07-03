package garbagecollection

import (
	"testing"

	"github.com/ffan/tidb-operator/operator"
)

func Test_gc(t *testing.T) {
	type args struct {
		o  *operator.Db
		n  *operator.Db
		pv PVProvisioner
	}
	hostpath := &HostPathPVProvisioner{"localhost.localdomain", "/tmp"}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{name: "decrease",
			args: args{
				o: &operator.Db{
					Tikv: &operator.Tikv{
						Stores: map[string]*operator.Store{
							"tikv-md-test-001": &operator.Store{
								Node: "localhost.localdomain",
								Name: "tikv-md-test-001",
							},
							"tikv-md-test-002": &operator.Store{
								Node: "localhost.localdomain",
								Name: "tikv-md-test-002",
							},
						},
					},
				},
				n: &operator.Db{
					Tikv: &operator.Tikv{
						Stores: map[string]*operator.Store{
							"tikv-md-test-001": &operator.Store{
								Node: "localhost.localdomain",
								Name: "tikv-md-test-001",
							},
						},
					},
				},
				pv: hostpath,
			},
		},
		{name: "increase",
			args: args{
				o: &operator.Db{
					Tikv: &operator.Tikv{
						Stores: map[string]*operator.Store{
							"tikv-mi-test-001": &operator.Store{
								Node: "localhost.localdomain",
								Name: "tikv-mi-test-001",
							},
						},
					},
				},
				n: &operator.Db{
					Tikv: &operator.Tikv{
						Stores: map[string]*operator.Store{
							"tikv-mi-test-001": &operator.Store{
								Node: "localhost.localdomain",
								Name: "tikv-mi-test-001",
							},
							"tikv-mi-test-002": &operator.Store{
								Node: "localhost.localdomain",
								Name: "tikv-mi-test-002",
							},
						},
					},
				},
				pv: hostpath,
			},
		},
		{name: "delete",
			args: args{
				o: &operator.Db{
					Tikv: &operator.Tikv{
						Stores: map[string]*operator.Store{
							"tikv-d-test-001": &operator.Store{
								Node: "localhost.localdomain",
								Name: "tikv-d-test-001",
							},
						},
					},
				},
				n: &operator.Db{
					Tikv: &operator.Tikv{},
				},
				pv: hostpath,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := gc(tt.args.o, tt.args.n, tt.args.pv); (err != nil) != tt.wantErr {
				t.Errorf("gc() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
