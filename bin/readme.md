单机版init: ./node_exporter --init --singleton --unique_id xxx --instance_id xxx  --ak xxx --sk xxx [ --port 9100 ]
集群init: ./node_exporter --init --instance_id xxx [ --port 9100 ]

单机版启动: ./node_exporter --singleton [ --remotehost http://xxxx/v1/write ]
集群版启动: ./node_exporter
