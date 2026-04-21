/*
 * @Description: vips CLI 能力探测
 * @Author: 安知鱼
 *
 * Phase 1 只返回 Available: false，以便 AutoEngine 在无 vips 的环境中正常工作。
 * Phase 2 Task 2.1 会补全：
 *   - exec.LookPath("vips")
 *   - vips --version 解析版本号
 *   - vips --suffix 或 --list classes 解析支持格式
 */
package engine

// Probe 探测当前运行环境是否有可用的 vips CLI。
// Phase 1 恒定返回 Available: false；Phase 2 会真实执行外部命令。
//
// 该函数应在启动时调用一次并缓存结果，避免重复 fork 子进程。
func Probe() VipsCapability {
	return VipsCapability{
		Available: false,
	}
}
