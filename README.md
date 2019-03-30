# `ft_node_exporter` 说明

## 简介

`ft_node_exporter` 是驻云基于开源的 Prometheus [node_exporter](https://github.com/prometheus/node_exporter) 项目扩展，用于支持对 Linux 服务器进行数据收集并上报到 `王教授` 的分析诊断平台进行分析的 exporter 项目。

## 支持操作系统

| 系统 | 支持的版本 |
| ----- | -----   |
| CentOS | >= 6.8 |
| Ubuntu | >= 14.04 |

## 安装

### 快速安装

下载快速安装脚本到需要监控的 Linux 的主机上，最新版脚本下载地址：[http://cloudcare-files.oss-cn-hangzhou.aliyuncs.com/ft_node_exporter/linux/release/install.sh](http://cloudcare-files.oss-cn-hangzhou.aliyuncs.com/ft_node_exporter/linux/release/install.sh)

	wget http://cloudcare-files.oss-cn-hangzhou.aliyuncs.com/ft_node_exporter/linux/release/install.sh](http://cloudcare-files.oss-cn-hangzhou.aliyuncs.com/ft_node_exporter/linux/release/install.sh
	
下载成功后，给安装脚本加上可执行权限：

	chmod +x install.sh
	
执行安装脚本：

	./install.sh --bind-addr 0.0.0.0:9100

执行安装脚本时可通过 bind-addr 参数指定绑定的 IP 和端口，如果不指定，默认绑定
`0.0.0.0:9100`

### 手动安装

下载 ft_node_exporter 最新的二进制文件，最新版本下载地址[http://cloudcare-files.oss-cn-hangzhou.aliyuncs.com/ft_node_exporter/linux/release/ft_node_exporter-v0.17.0-rc.0-144-ga107c90.tar.gz](http://cloudcare-files.oss-cn-hangzhou.aliyuncs.com/ft_node_exporter/linux/release/ft_node_exporter-v0.17.0-rc.0-144-ga107c90.tar.gz)

	wget http://cloudcare-files.oss-cn-hangzhou.aliyuncs.com/ft_node_exporter/linux/release/ft_node_exporter-v0.17.0-rc.0-144-ga107c90.tar.gz

解压后运行 `ft_node_exporter` 文件

	 tar -zxvf ft_node_exporter-v0.17.0-rc.0-144-ga107c90.tar.gz 

## 配置 Forethought


部署好 `ft_wmi_exporter`后，在 Forethought 的 `Prometheus` 的配置文件中需要配置抓取的 `target`。

**配置抓取时序数据**

- metrics_path：默认 `/metrics`
- targets：即安装 `ft_node_exporter` 服务器的IP地址和监听的端口，需要保证Forethought 的服务器可访问该地址
- scrape_interval：抓取的频率，推荐设置为 `1m` ，及 1 分钟抓取 1 次
- labels：如果没有配置 `uploader_uid` 和 `group_name`，抓取的数据默认不会上传到 `王教授` 的分析诊断平台，`uploader_uid` 和 `group_name` 的获取和设置方法 参见 Forethought 的使用文档
 
示例：

    - job_name: 'metrics_job'
      scrape_interval: 1m
      metrics_path: /metrics
      static_configs:
      - targets: ['172.16.0.85:9200']
      labels:
        uploader_uid: 'uid-3fb91f37-59af-4d3b-bf66-44426fc4afb3'
        group_name: 'demogroup'



**配置抓取kv数据**

- metrics_path：默认 `/kvs/json`
- targets：即安装 `ft_node_exporter` 服务器的IP地址和监听的端口，需要保证Forethought 的服务器可访问该地址
- scrape_interval：抓取的频率，推荐设置为 `15m` ，及 15 分钟抓取 1 次
- labels：如果没有配置 `uploader_uid` 和 `group_name`，抓取的数据默认不会上传到 `王教授` 的分析诊断平台，`uploader_uid` 和 `group_name` 的获取和设置方法 参见 Forethought 的使用文档

示例：

    - job_name: 'kvs_job'
    	scrape_interval: 15m
    	metrics_path: /kvs/json
    	static_configs:
    	- targets: ['172.16.0.85:9200']
      	labels:
         uploader_uid: 'uid-3fb91f37-59af-4d3b-bf66-44426fc4afb3'
         group_name: 'demogroup'
         
**注意**

同一个 `ft_node_exporter` 抓取 `target` 的 `uploader_uid` 和 `group_name` 必须相同
