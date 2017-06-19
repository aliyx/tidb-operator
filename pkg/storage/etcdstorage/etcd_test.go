package etcdstorage

import (
	"context"
	"log"
	"os"
	"reflect"
	"testing"
)

var (
	testEtcdAddr = "127.0.0.1:2379"

	testEc *etcdClient
)

func TestMain(m *testing.M) {
	ec, err := newEtcdClient(testEtcdAddr)
	if err != nil {
		log.Fatalln("cannt create etct client url=%s, %v", testEtcdAddr, err)
	}
	testEc = ec
	os.Exit(m.Run())
}

func TestStorage_Create(t *testing.T) {
	type fields struct {
		ec *etcdClient
	}
	type args struct {
		ctx      context.Context
		path     string
		contents []byte
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		// want    models.Version
		wantErr bool
	}{
		{
			name:   "Create()",
			fields: fields{ec: testEc},
			args: args{
				ctx:      context.Background(),
				path:     "abc",
				contents: []byte("abc"),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Storage{
				ec: tt.fields.ec,
			}
			_, err := s.Create(tt.args.ctx, tt.args.path, tt.args.contents)
			if (err != nil) != tt.wantErr {
				t.Errorf("Storage.Create() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestStorage_Get(t *testing.T) {
	type fields struct {
		ec *etcdClient
	}
	type args struct {
		ctx  context.Context
		path string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []byte
		wantErr bool
	}{
		{
			name:   "GetData",
			fields: fields{ec: testEc},
			args: args{
				ctx:  context.Background(),
				path: "abc",
			},
			want:    []byte("abc"),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Storage{
				ec: tt.fields.ec,
			}
			got, _, err := s.Get(tt.args.ctx, tt.args.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("Storage.Get() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Storage.Get() got = %v, want %v", got, tt.want)
			}
		})
	}
}
