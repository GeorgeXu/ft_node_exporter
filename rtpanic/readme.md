# 运行时奔溃处理

由于 agent 本身是不允许在客户环境奔溃, 所以, 对于一些第三方库/agent 本身的一些原因造成的奔溃, 需要做复活以及奔溃现场处理.

对于某个 goroutine 主体, 如果需要做恢复以及上报 crash 调用栈信息, 做如下设置即可, 如:

	// goroutine 主体函数
	func someGlobalSpaceFunc() {

		// 声明两个回调函数, 一个用于复活 goroutine, 一个用户报告 crash 调用栈给 csos

		var cb_recover, cb_clean RecoverCallback

		cb_clean = func(info string) {
			// do cleanup...
		}

		cb_recover = func(info string) {
			defer rtpanic.Recover(cb_recover, cb_clean)()

			for {
				// do jobs...
			}
		}

		f("") // 执行主要的 goroutine 代码
	}

	func main() {
		// ...

		go someGlobalSpaceFunc()

		// do other jobs...
	}

具体用例, 参见 agent 中具体的调用代码.

需设置复活回调的地方, 一般是常驻 gorouting, 比如 session 池管理/各个模块之间的
channel 通道 goroutine 等等. 其他的只跟某个具体 session 相关的 gorouting, 原则
上不应该设置复活回调, 只需要设置 panic uploader 回调即可.
