package garbagecollection

import (
	"testing"

	"github.com/ffan/tidb-operator/models"
)

func Test_gc(t *testing.T) {
	type args struct {
		o  *models.Db
		n  *models.Db
		pv PVProvisioner
	}
	hostpath := &HostPathPVProvisioner{"/tmp"}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{name: "decrease",
			args: args{
				o: &models.Db{
					Tikv: &models.Tikv{
						Stores: map[string]*models.Store{
							"tikv-md-test-001": &models.Store{
								Node: "localhost.localdomain",
							},
							"tikv-md-test-002": &models.Store{
								Node: "localhost.localdomain",
							},
						},
					},
				},
				n: &models.Db{
					Tikv: &models.Tikv{
						Stores: map[string]*models.Store{
							"tikv-md-test-001": &models.Store{
								Node: "localhost.localdomain",
							},
						},
					},
				},
				pv: hostpath,
			},
		},
		{name: "increase",
			args: args{
				o: &models.Db{
					Tikv: &models.Tikv{
						Stores: map[string]*models.Store{
							"tikv-mi-test-001": &models.Store{
								Node: "localhost.localdomain",
							},
						},
					},
				},
				n: &models.Db{
					Tikv: &models.Tikv{
						Stores: map[string]*models.Store{
							"tikv-mi-test-001": &models.Store{
								Node: "localhost.localdomain",
							},
							"tikv-mi-test-002": &models.Store{
								Node: "localhost.localdomain",
							},
						},
					},
				},
				pv: hostpath,
			},
		},
		{name: "delete",
			args: args{
				o: &models.Db{
					Tikv: &models.Tikv{
						Stores: map[string]*models.Store{
							"tikv-d-test-001": &models.Store{
								Node: "localhost.localdomain",
							},
						},
					},
				},
				n: &models.Db{
					Tikv: &models.Tikv{},
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
