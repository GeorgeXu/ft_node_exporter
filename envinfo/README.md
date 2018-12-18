# intro

这里主要说明一下主机环境信息的收集. 所谓环境信息, 是指那些无法直接用数值(整数/浮点)来衡量的系统指标, 这些指标范围相对一些数值指标(比如 CPU 利用率/网卡吞吐量这些都是可以用纯数值来衡量)更为广泛和庞杂.

## 数据格式

由于环境信息指标无法用数值来表示(如系统中所有用户信息), 所以采用文本(字符串)的形式来收集这些数据. 但是由于现在我们采用的 OpenTSDB 无法支持字符串作为指标值, 所以, 我们将这些环境信息的数值全部置为 -1, 而其指标值作为指标 tag 来存放, 比如用户列表信息和 sshd 分别表示为:

	envinfo_user_list{json="nV1aWQiOiIifQpdCg=="} -1
	envinfo_sshd_configure{raw="nLgpVc2VQQU0geWVzCg=="} -1

其中:

- `envinfo_user_list` 为指标名称
- `json` 为该指标值 base64 编码后的值, 且该值解码后的格式是 json
- `raw` 表示后面的 base64 编码的为原始文件的内容, base64 解码后得到的就是原始的 `/etc/ssh/sshd_config` 的内容
- `-1` 指该指标的值, 这里的实际上为无意义, 只是为了兼容 OpenTSDB 的数值协议

## 基本的采集方式

目前我们采用如下几种方式来获取这些环境信息的数据

- `osquery` 通过开源的 [osquery](https://github.com/facebook/osquery), 我们可以通过 SQL 语句获取一些系统的环境信息
- `cat` 直接获取相关指标的配置文件
- 在 Windows 上, 可以通过一些既定的方式获取环境信息(比如注册表等)
- 对某些既定服务而言, 通过发送某种请求(如 HTTP) 到指定的地址, 可以获取其配置/运行信息

## 如何编写收集器

通过在 `envinfo` 模块下, 添加具体的代码文件即可, 参见 `sshd.go` 或 `user.go`. 其中几个地方需要注意:

- 每个 xxx.go 中需要有一个 `init()` 函数用于注册收集器
- 其中 `prometheus.NewDesc()` 这个方法传参数需要注意, 如果是收集到的数据已经 JSON 序列化好了, 则 `variableLabels` 这个参数应该传 `[]string{"json"}`, 否则传 `[]string{"raw"}`, 即收集到的数据是原始格式.  这也暗示着, 中心只接收这两种类型的数据, 而且分别以 `json` 和 `raw` 来区分解析
- 每个收集器都需要定义一个 `xxxCollector`, 且实现 `Update` 接口