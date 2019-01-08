# -*- encoding: utf8 -*-
# date: Tue Jan  8 14:50:47 CST 2019
# auth: tanb

import yaml
import argparse
import uuid
import sys
import os
import stat
import pdb

template = '''
team_id: xxxxxxxxxxxxxxxxxxxxxx
cloud_asset_id: xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
ak: xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
sk: xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
port: xxxx
collectors:
  arp: true
  bcache: true
  bonding: true
  buddyinfo: true
  conntrack: true
  cpu: true
  diskstats: true
  drbd: true
  edac: true
  entropy: true
  filefd: true
  filesystem: true
  hwmon: true
  infiniband: true
  interrupts: true
  ipvs: true
  ksmd: true
  loadavg: true
  logind: true
  mdadm: true
  meminfo: true
  meminfo_numa: true
  mountstats: true
  netclass: true
  netdev: true
  netstat: true
  nfs: true
  nfsd: true
  nginx: true
  ntp: true
  processes: true
  qdisc: true
  runit: true
  sockstat: true
  stat: true
  supervisord: true
  systemd: true
  tcpstat: true
  textfile: true
  time: true
  timex: true
  uname: true
  vmstat: true
  wifi: true
  xfs: true
  zfs: true
single_mode: 1
host: default
scrap_metric_interval: xxx
scrap_env_info_interval: xxx
remote_host: xxxxxxxxxxxxxxxxxxxxxxxx
enable_all: 1
env_cfg_file: /usr/local/cloudcare/env.json
provider: xxxxxx
'''

def main():
    parser = argparse.ArgumentParser(description="")
    parser.add_argument('--team-id', action='store', type=str, dest='team_id', default='H3HrChAjUDvzY72sY338Zn')
    parser.add_argument('--node-count', action='store', type=int, dest='node_count', default=32)
    parser.add_argument('--kodo-host', action='store', type=str, dest='kodo_host', default='http://kodo-testing.prof.wang')
    parser.add_argument('--metric-interval', action='store', type=int, dest='metric_interval', default=1)
    parser.add_argument('--env-info-interval', action='store', type=int, dest='env_info_interval', default=15)
    parser.add_argument('--dir', action='store', type=str, dest='out_dir', default='/tmp/cfg')
    parser.add_argument('--ak', action='store', type=str, dest='ak', default='7bba77b11f7a4e889ec0084ce3353ae7')
    parser.add_argument('--sk', action='store', type=str, dest='sk', default='+xISxtlqOrhRA+YxxA7I0eslqTu3E5awU+847toUrXoLKGiTizou3kmfy1TO88stOcWBKUCP8JA+B3taTwFF8oSgQ8BPSYLwScqd9a1X0o13ccX8VzTygFqeYOCbcs3WYiwl3Ivwq2cKgoqyqkxWqHK/BbkkH9reJJllSINiUKM=')
    parser.add_argument('--provider', action='store', type=str, dest='provider', default='aliyun')

    args = parser.parse_args(sys.argv[1:])

    try:
        os.mkdir(args.out_dir)
    except OSError:
        pass
    except Exception as ex:
        print ex
        return

    cfg = yaml.load(template)

    #pdb.set_trace()

    for i in range(args.node_count):
        with open('%s/cfg.%d.yml' % (args.out_dir, i), 'w') as fd:
            cfg['scrap_metric_interval'] = args.metric_interval
            cfg['scrap_env_info_interval'] = args.env_info_interval
            cfg['remote_host'] = args.kodo_host
            cfg['port'] = 9100 + i
            cfg['cloud_asset_id'] = 'clas-' + str(uuid.uuid4())
            cfg['ak'] = args.ak
            cfg['sk'] = args.sk
            cfg['team_id'] = args.team_id
            cfg['provider'] = args.provider

            yaml.dump(cfg, fd, default_flow_style=False) # 生成 yaml 配置文件

    # 生成启动脚本
    run_sh = "%s/run.sh" % (args.out_dir)
    with open(run_sh, 'w') as fd:
        fd.write('#!/bin/bash\n')
        fd.write('ps -ef | pgrep corsair | xargs sudo kill &> /dev/null\n')
        for i in range(args.node_count):
            fd.write('(sudo /usr/local/cloudcare/corsair --cfg %s/cfg.%d.yml --log.level="info" &> /dev/null &)\n' % (args.out_dir, i))

    # 启动脚本转换成可直接运行
    st = os.stat(run_sh)
    os.chmod(run_sh, st.st_mode | stat.S_IEXEC)


if __name__ == '__main__':
    main()
