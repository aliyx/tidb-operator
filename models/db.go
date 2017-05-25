package models

import (
	"errors"
	"fmt"

	"time"

	"github.com/astaxie/beego/logs"
	tsql "github.com/ffan/tidb-k8s/mysql"
)

var (
	// ErrRepop is returned by functions to specify the operation is executing.
	ErrRepop = errors.New("the previous operation is being executed")
)

const (
	transfering = "Transfering"
	transferErr = "Error"
	transferFin = "Finish"
)

// InitTidb 初始化
func InitTidb(cell string) (err error) {
	e := NewEvent(cell, "tidb", "init")
	defer func() {
		e.Trace(err, "Init tidb privileges")
	}()
	var td *Tidb
	td, err = GetTidb(cell)
	if err != nil {
		return err
	}
	if td.Status < TidbStarted || td.Status > TidbInited {
		return fmt.Errorf(`tidb "%s" no started`, cell)
	}
	my := tsql.NewMysql(td.Schema, td.Nets[0].IP, td.Nets[0].Port, td.User, td.Password)
	if err = my.Init(); err != nil {
		rollout(cell, TidbInitFailed)
		return err
	}
	rollout(cell, TidbInited)
	return nil
}

// Migrate the mysql data to the current tidb
func Migrate(cell string, src tsql.Mysql) error {
	td, err := GetTidb(cell)
	if err != nil {
		return err
	}
	if !td.isOk() {
		return fmt.Errorf("tidb is not available")
	}
	if td.Transfer != "" {
		return errors.New("can not migrate multiple times")
	}
	if len(src.IP) < 1 || src.Port < 1 || len(src.User) < 1 || len(src.Password) < 1 || len(src.Database) < 1 {
		return fmt.Errorf("invalid database %+v", src)
	}
	if td.Schema != src.Database {
		return fmt.Errorf("both schemas must be the same")
	}
	var net Net
	for _, n := range td.Nets {
		if n.Name == portMysql {
			net = n
			break
		}
	}
	my := &tsql.Mydumper{
		Src:  src,
		Desc: *tsql.NewMysql(td.Schema, net.IP, net.Port, td.User, td.Password),
	}
	if err := my.Check(); err != nil {
		return fmt.Errorf(`schema "%s" does not support migration error: %v`, cell, err)
	}
	td.Transfer = transfering
	if err := td.Update(); err != nil {
		return err
	}
	go func() {
		defer func() {
			if err != nil {
				td.Transfer = transferErr
			} else {
				td.Transfer = transferFin
			}
			td.Update()
		}()
		e := NewEvent(cell, "transfer", "dumper")
		if err = my.Dump(); err != nil {
			logs.Error(`Dump database "%+v" error: %v`, my.Src, err)
		}
		e.Trace(err, fmt.Sprintf(`Dump mysql %s to local`, src.IP))
		if err != nil {
			return
		}

		e = NewEvent(cell, "transfer", "loader")
		if err = my.Load(); err != nil {
			logs.Error(`Load data to tidb "%+v" error: %v`, my.Desc, err)
		}
		e.Trace(err, "Load data to tidb")
	}()
	return nil
}

// Start tidb server
func Start(cell string) (err error) {
	if started(cell) {
		return ErrRepop
	}
	go func() {
		k8sMu.Lock()
		defer k8sMu.Unlock()
		e := NewEvent(cell, "tidb", "start")
		defer func() {
			e.Trace(err, "Start deploying tidb clusters on kubernetes")
		}()
		var pd *Pd
		var tk *Tikv
		var td *Tidb
		if pd, err = GetPd(cell); err != nil {
			logs.Error("Get pd %s err: %v", cell, err)
			return
		}
		rollout(cell, PdPending)
		if err = pd.Run(); err != nil {
			logs.Error("Run pd %s on k8s err: %v", cell, err)
			return
		}
		if tk, err = GetTikv(cell); err != nil {
			logs.Error("Get tikv %s err: %v", cell, err)
			return
		}
		rollout(cell, TikvPending)
		if err = tk.Run(); err != nil {
			logs.Error("Run tikv %s on k8s err: %v", cell, err)
			return
		}
		if td, err = GetTidb(cell); err != nil {
			logs.Error("Get tidb %s err: %v", cell, err)
			return
		}
		rollout(cell, TidbPending)
		if err = td.Run(); err != nil {
			logs.Error("Run tidb %s on k8s err: %v", cell, err)
			return
		}
		if err = InitTidb(cell); err != nil {
			logs.Error("Init tidb %s privileges err: %v", cell, err)
			return
		}
	}()
	return nil
}

// Stop tidb server
func Stop(cell string, ch chan int) (err error) {
	if !started(cell) {
		return err
	}
	e := NewEvent(cell, "tidb", "stop")
	defer func() {
		if err != nil {
			e.Trace(err, fmt.Sprintf("Delete tidb pods on k8s"))
		}
	}()
	if td, _ := GetTidb(cell); td != nil {
		if err = td.stop(); err != nil {
			return err
		}
	}
	if tk, _ := GetTikv(cell); tk != nil {
		if err = tk.stop(); err != nil {
			return err
		}
	}
	if pd, _ := GetPd(cell); pd != nil {
		if err = pd.stop(); err != nil {
			return err
		}
	}
	// waitring for all pod deleted
	go func() {
		defer func() {
			if ch != nil {
				ch <- 0
			}
		}()
		for {
			if started(cell) {
				logs.Warn(`tidb "%s" has not been cleared yet`, cell)
				time.Sleep(time.Second)
			} else {
				rollout(cell, Undefined)
				break
			}
		}
		e.Trace(nil, fmt.Sprintf("Stop tidb pods on k8s"))
	}()
	return err
}

// Restart first stop tidb, second start tidb
func Restart(cell string) (err error) {
	go func() {
		td, _ := GetTidb(cell)
		e := NewEvent(cell, "tidb", "restart")
		defer func() {
			e.Trace(err, fmt.Sprintf("Restart tidb[status=%d]", td.Status))
		}()
		ch := make(chan int, 1)
		if err = Stop(cell, ch); err != nil {
			logs.Error("Delete tidb %s pods on k8s error: %v", cell, err)
			return
		}
		// waiting for all pod deleted
		select {
		case <-ch:
		}
		if err = Start(cell); err != nil {
			logs.Error("Create tidb %s pods on k8s error: %v", cell, err)
			return
		}
	}()
	return err
}
