import logs
from os import path


class Config:

    def __init__(self, db, host, port, user, password):
        if db == None or host == None or port == None or user == None or password == None:
            logs.critical("db or host or port... is nil.")
        self.db = db
        self.host = host
        self.port = port
        self.user = user
        self.password = password

    def getDataDir(self):
        return '/tmp/' + self.db

    def getDumpedMeta(self):
        return self.getDataDir() + "/metadata"

    def getConfig(self):
        return self.getDataDir() + "/config.toml"

    def getMeta(self):
        return self.getDataDir() + "/syncer.meta"

    def getCheckpoint(self):
        return self.getDataDir() + "/loader.checkpoint"

    def genSyncMetaFile(self):
        f = self.getMeta()
        if path.isfile(f):
            return

        dump = self.getDumpedMeta()
        if not path.isfile(dump):
            raise NameError("can't get dump metadata file: " + dump)

        with open(dump) as f:
            lines = f.readlines()
        for line in lines:
            if 'Log: ' in line:
                name = line.strip()[5:]
            elif 'Pos: ' in line:
                pos = line.strip()[5:]
            else:
                continue
        if name == None or pos == None:
            raise NameError("can't get dump binlog name and position")

        f = open(self.getMeta(), 'w+')
        logs.info("binlog-name:%s binlog-pos:%s", name, pos)
        f.write('binlog-name = "' + name + '"\n')
        f.write('binlog-pos = ' + pos + '\n')

    def genSyncConfigFile(self, to):
        if to == None:
            raise NameError("to db is nil")
        
        if path.isfile(self.getConfig()):
            return

        f = open(self.getConfig(), 'w+')
        f.write('log-level = "info"\n')
        f.write('server-id = 101\n')
        f.write('\n')

        f.write('meta = "' + self.getMeta() + '"\n')
        f.write('worker-count = 1\n')
        f.write('batch = 1\n')
        f.write('\n')

        f.write('[from]\n')
        f.write('host = "' + self.host + '"\n')
        f.write('port = ' + str(self.port) + '\n')
        f.write('user = "' + self.user + '"\n')
        f.write('password = "' + self.password + '"\n')
        f.write('\n')

        f.write('[to]\n')
        f.write('host = "' + to.host + '"\n')
        f.write('port = ' + str(to.port) + '\n')
        f.write('user = "' + to.user + '"\n')
        f.write('password = "' + to.password + '"\n')
        f.close()

    def toDumper(self):
        cmds = '/usr/local/mydumper-linux-amd64/bin/mydumper '
        cmds += '-t 2 -F 128 --no-views --skip-tz-utc --no-locks --less-locking --verbose 3'
        cmds += ' -h ' + self.host
        cmds += ' -P ' + str(self.port)
        cmds += ' -u ' + self.user
        cmds += ' -p ' + self.password
        cmds += ' -B ' + self.db
        cmds += ' -o ' + self.getDataDir()
        return cmds

    def toLoader(self):
        cmds = '/usr/local/tidb-enterprise-tools-latest-linux-amd64/bin/loader '
        cmds += '-t 2'
        cmds += ' -h ' + self.host
        cmds += ' -P ' + str(self.port)
        cmds += ' -u ' + self.user
        cmds += ' -p ' + self.password
        # will save checkpoint tidb tidb_loader
        # cmds += ' -checkpoint ' + self.getCheckpoint()
        cmds += ' -d ' + self.getDataDir()
        return cmds

    def toSyncer(self):
        cmds = '/usr/local/tidb-enterprise-tools-latest-linux-amd64/bin/syncer '
        cmds += '-config ' + self.getConfig()
        return cmds
