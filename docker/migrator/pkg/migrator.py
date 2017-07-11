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


class Migrator:
    def __init__(self, src, dest, notice=None):
        if src == None and dest == None:
            logs.critical("both src and dest database is nil")
        self.src = src
        self.dest = dest

    # dump remote mysql data to /tmp/{db} dir
    def dump(self, api):
        if self.src == None:
            logs.critical("src database is nil")

        datadir = self.src.getDataDir()

        # remove all old data
        if path.isdir(datadir):
            shutil.rmtree(datadir)

        rest.sync_stat(api, 'Dumping')

        cmds = self.src.toDumper()
        logs.info("dumper: %s", cmds)
        try:
            subprocess.check_output(shlex.split(cmds), stderr=subprocess.STDOUT)
        except  subprocess.CalledProcessError as e:
            print e.output
            try:
                err = re.search('\*\*: (.*)', e.output).group(1)
            except:
                err = e.output
            rest.sync_stat(api, 'DumpError', reason=err)
            logs.critical("dump error")

    # load local data to tidb
    def load(self, api):
        if not path.isfile(self.src.getDumpedMeta()):
            self.dump(api)

        rest.sync_stat(api, 'Loading')
        cmds = self.dest.toLoader()
        logs.info("loader: %s", cmds)
        try:
            subprocess.check_call(shlex.split(cmds), stderr=subprocess.STDOUT)
        except  subprocess.CalledProcessError as e:
            try:
                # can't get error message from stderr
                err = re.search('[error] (.*)', e.output).group(1)
            except:
                err = 'exit status ' + str(e.returncode)
            rest.sync_stat(api, 'LoadError', reason=err)
            logs.critical("load error")

    def sync(self, api):
        if not path.isfile(self.src.getCheckpoint()):
            self.load(api)

        self.src.genSyncConfigFile(self.dest)
        if not path.isfile(self.src.getMeta()):
            self.src.genSyncMetaFile()

        rest.sync_stat(api, 'Syncing')
        cmds = self.dest.toSyncer()
        logs.info("syncer: %s", cmds)
        try:
            subprocess.check_call(shlex.split(cmds), stderr=subprocess.STDOUT)
        except  subprocess.CalledProcessError as e:
            try:
                # can't get error message from stderr
                err = re.search('[error] (.*)', e.output).group(1)
            except:
                err = 'exit status ' + str(e.returncode)
            rest.sync_stat(api, 'SyncStoped', reason=err)


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

    help = 'migrator --operator <dump,load,sync> --notice --database <db> --src-host <host> --src-port <port> --src-user <user> --src-password <password> --dest-host <host> --dest-port <port> --dest-user <user> --dest-password <password>'
    try:
        opts, args = getopt.getopt(
            argv, "h:", ["operator=", "notice=", "database=", "src-host=", "src-port=", "src-user=", "src-password=", "dest-host=", "dest-port=", "dest-user=", "dest-password="])
    except getopt.GetoptError:
        print "Error: " + help
        sys.exit(2)

    for opt, arg in opts:
        print opt + ":" + arg
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

    src = config.Config(db, s_host, s_port, s_user, s_password)
    dest = config.Config(db, d_host, d_port, d_user, d_password)
    m = Migrator(src, dest, notice)
    if operator == 'dump':
        m.dump(notice)
        rest.sync_stat(notice, 'Finished')
    elif operator == 'load':
        m.load(notice)
        rest.sync_stat(notice, 'Finished')
    elif operator == 'sync':
        m.sync(notice)
    else:
        print 'Unsupport operator: ' + operator
        sys.exit(2)


if __name__ == "__main__":
    main(sys.argv[1:])

# migrator.py --database xinyang1 --src-host 10.213.124.194 --src-port 13306 --src-user root --src-password EJq4dspojdY3FmVF?TYVBkEMB --dest-host 10.213.44.128 --dest-port 13213 --dest-user xinyang1 --dest-password xinyang1 --operator sync --notice http://10.213.44.128:12808/tidb/api/v1/tidbs/006-xinyang1
