import subprocess
import shlex
import io
import sys

def run(flag, cmd):
    process = subprocess.Popen(shlex.split(cmd), stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
    errs = []
    while True:
        line = process.stdout.readline()
        if flag in line:
            errs.append(line)
        sys.stdout.write(line)
        retcode = process.poll()
        if retcode != None:
            sys.stdout.flush()
            if retcode:
                if len(errs) == 0:
                    errs.append(process.communicate()[0])
                out = ';'.join(errs)
                raise subprocess.CalledProcessError(retcode, cmd, output=out)
            else:
                return

# run("[fatal]","/usr/local/tidb-enterprise-tools-latest-linux-amd64/bin/loader -t 2 -h 10.213.44.128 -P 12835 -u xinyang1 -p xinyang1 -d /data/xinyang1")
# /usr/local/tidb-enterprise-tools-latest-linux-amd64/bin/loader -t 2 -h 10.213.44.128 -P 13213 -u xinyang1 -p xinyang1 -d /data/xinyang1
