import logs
from subprocess import call
import shlex
import config
import shutil
from os import path
import sys
import getopt
import rest


class Migrator:
    def __init__(self, src, dest, notice=None):
        if src == None and dest == None:
            logs.critical("both src and dest database is nil")
        self.src = src
        self.dest = dest

    # dump remote mysql data to /tmp/{db} dir
    def dump(self):
        if self.src == None:
            logs.critical("src database is nil")

        datadir = self.src.getDataDir()

        # remove all old data
        if path.isdir(datadir):
            shutil.rmtree(datadir)

        cmds = self.src.toDumper()
        logs.info("dumper: %s", cmds)
        return call(shlex.split(cmds))

    # load local data to tidb
    def load(self):
        datadir = self.dest.getDataDir()
        if not path.isdir(datadir):
            logs.critical("no data loaded: %s", datadir)

        cmds = self.dest.toLoader()
        logs.info("loader: %s", cmds)
        return call(shlex.split(cmds))

    def sync(self):
        self.src.genSyncConfigFile(self.dest)
        if not path.isfile(self.src.getMeta()):
            self.src.genSyncMetaFile()

        cmds = self.dest.toSyncer()
        logs.info("syncer: %s", cmds)
        return call(shlex.split(cmds))


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

    help = 'migrator.py --operator <dump,load,sync> --notice --database <db> --src-host <host> --src-port <port> --src-user <user> --src-password <password> --dest-host <host> --dest-port <port> --dest-user <user> --dest-password <password>'
    try:
        opts, args = getopt.getopt(
            argv, "h:", ["operator=", "notice=", "database=", "src-host=", "src-port=", "src-user=", "src-password=", "dest-host=", "dest-port=", "dest-user=", "dest-password="])
    except getopt.GetoptError:
        logs.critical("Error: %s", help)

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
        rest.sync_stat(notice, 'Dumping')
        if m.dump() != 0:
            rest.sync_stat(notice, 'DumpError')
    elif operator == 'load':
        if path.isfile(src.getCheckpoint()):
            logs.critical("maybe has loaded.")
        if not path.isdir(src.getDataDir()):
            m.dump()
        m.load()
    elif operator == 'sync':
        if not path.isfile(src.getCheckpoint()):
            m.load()
        m.sync()
    else:
        print 'Unsupport operator: ' + operator
        sys.exit(2)

if __name__ == "__main__":
    main(sys.argv[1:])

# migrator.py --database xinyang1 --src-host 10.213.124.194 --src-port 13306 --src-user root --src-password EJq4dspojdY3FmVF?TYVBkEMB --dest-host 10.213.44.128 --dest-port 13213 --dest-user xinyang1 --dest-password xinyang1 --operator sync --notice http://10.213.44.128:12808/tidb/api/v1/tidbs/006-xinyang1

