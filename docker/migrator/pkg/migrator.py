import logs
import subprocess
from subprocess import call
import shlex
import config
import shutil
from os import path
import sys
import getopt
import rest
import re
import shell


class Migrator:
    def __init__(self, src, dest, notice=None):
        if src == None and dest == None:
            logs.critical("both src and dest database is nil")
        self.src = src
        self.dest = dest
        self.notice = notice

    # dump remote mysql data to /tmp/{db} dir
    def dump(self):
        if self.src == None:
            logs.critical("src database is nil")

        datadir = self.src.getDataDir()

        # remove all old data
        if path.isdir(datadir):
            shutil.rmtree(datadir)

        rest.sync_stat(self.notice, 'Dumping')

        cmds = self.src.toDumper()
        logs.info("dumper: %s", cmds)
        try:
            shell.run('CRITICAL', cmds)
        except  subprocess.CalledProcessError as e:
            rest.sync_stat(self.notice, 'DumpError', reason=e.output)
            logs.critical("dump error")

    # load local data to tidb
    def load(self):
        if not path.isfile(self.src.getDumpedMeta()):
            self.dump()

        rest.sync_stat(self.notice, 'Loading')
        cmds = self.dest.toLoader()
        logs.info("loader: %s", cmds)
        try:
            shell.run('[fatal]', cmds)
        except  subprocess.CalledProcessError as e:
            rest.sync_stat(self.notice, 'LoadError', reason=e.output)
            logs.critical("load error")

    def sync(self):
        self.src.genSyncConfigFile(self.dest)
        if not path.isfile(self.src.getMeta()):
            self.src.genSyncMetaFile()

        rest.sync_stat(self.notice, 'Syncing')
        cmds = self.dest.toSyncer()
        logs.info("syncer: %s", cmds)
        try:
             shell.run('[fatal]', cmds)
        except  subprocess.CalledProcessError as e:
            err = e.output
        finally:
            if err == None:
                err = 'Unknow'
            rest.sync_stat(self.notice, 'SyncError', reason=err)


def main(argv):
    operator = ''
    notice = None

    db = ''
    s_host = ''
    s_port = 0
    s_user = ''
    s_password = ''

    d_host = ''
    d_port = 0
    d_user = ''
    d_password = ''

    tables = ''

    help = 'migrator --operator <dump,load,sync> --notice --database <db> --src-host <host> --src-port <port> --src-user <user> --src-password <password> --dest-host <host> --dest-port <port> --dest-user <user> --dest-password <password> --tables <tables>'
    try:
        opts, args = getopt.getopt(
            argv, "h:", ["operator=", "notice=", "database=", "src-host=", "src-port=", "src-user=", "src-password=", "dest-host=", "dest-port=", "dest-user=", "dest-password=", "tables="])
    except getopt.GetoptError:
        print "Error: " + help
        sys.exit(2)

    for opt, arg in opts:
        logs.info(opt + ": " + arg)
        if opt == "-h":
            print help
        elif opt in ("--operator") and arg in ("dump", "load", "sync"):
            operator = arg
        elif opt in ("--notice"):
            notice = arg
        elif opt in ("--database"):
            db = arg
        elif opt in ("--src-host"):
            s_host = arg
        elif opt in ("--src-port"):
            s_port = arg
        elif opt in ("--src-user"):
            s_user = arg
        elif opt in ("--src-password"):
            s_password = arg
        elif opt in ("--dest-host"):
            d_host = arg
        elif opt in ("--dest-port"):
            d_port = arg
        elif opt in ("--dest-user"):
            d_user = arg
        elif opt in ("--dest-password"):
            d_password = arg
        elif opt in ("--tables"):
            tables = arg
    src = config.Config(db, s_host, s_port, s_user, s_password)
    dest = config.Config(db, d_host, d_port, d_user, d_password)
    if len(tables) > 0:
        src.tables = tables
        dest.tables = tables
    m = Migrator(src, dest, notice)
    if operator == 'dump':
        m.dump()
        rest.sync_stat(notice, 'Finished')
    elif operator == 'load':
        m.load()
        rest.sync_stat(notice, 'Finished')
    elif operator == 'sync':
        m.sync()
    else:
        print 'Unsupport operator: ' + operator
        sys.exit(2)


if __name__ == "__main__":
    main(sys.argv[1:])

# migrator.py --database xinyang1 --src-host 10.213.125.85 --src-port 13306 --src-user root --src-password EJq4dspojdY3FmVF?TYVBkEMB --dest-host 10.213.44.128 --dest-port 14988 --dest-user xinyang1 --dest-password xinyang1 --operator sync --notice http://10.213.44.128:12808/tidb/api/v1/tidbs/006-xinyang1 --tables t1,t2
