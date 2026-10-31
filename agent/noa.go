package agent

import (
	"log"
	"path/filepath"

	"github.com/Autumn-27/norma/agentcore"
	"github.com/Autumn-27/norma/noa"
	"github.com/Autumn-27/norma/noaadapter"
)

// noaWarn returns a diagnostics sink tagging non-fatal noa messages with the
// session, routed through the package logger (agents have no per-instance one).
func noaWarn(session string) func(string) {
	return func(msg string) { log.Printf("[noa] %s: %s", session, msg) }
}

// defaultNoaContextLimit caps noa's effective context budget so tool-heavy
// sessions start compressing/truncating around ~38k–56k tokens rather than
// drifting past 60k–130k under the 200k default, while preserving message
// byte identity, CoT thinking replay, and prompt-cache prefixes between compressions.
const defaultNoaContextLimit = 80_000

func buildNoaConfig(windowTokens int) *noa.Config {
	limit := windowTokens
	if limit <= 0 || limit > defaultNoaContextLimit {
		limit = defaultNoaContextLimit
	}
	if limit < minWindowK*1000 {
		limit = minWindowK * 1000
	}
	cfg := noa.DefaultConfig(limit)
	cfg.Nudge.MinContextLimitPct = 0.40
	cfg.Nudge.MaxContextLimitPct = 0.65
	cfg.Nudge.EmergencyThresholdPct = 0.88
	cfg.Nudge.GrowthFloor = 15_000
	cfg.Nudge.GrowthCap = 25_000
	cfg.Nudge.MinGrowthFloor = 8_000
	cfg.Truncate.Threshold = 0.78
	cfg.Compress.MinCompressRange = 3_000
	cfg.PreserveRecentMessages = 5
	cfg.PreserveRecentTokens = 4_000
	return &cfg
}

// noa 是 norma v0.4.0 引入的「模型驱动上下文压缩」机制,作为平台实验功能由用户在
// 系统设置中开关。它与内置 compaction 互斥:noaadapter.Enable 是唯一入口,一次挂上
// 上下文接管器(Compactor)、Compress 工具与三段常驻提示词,不调用 Enable 即为关闭
// (内置 compaction 照常工作)。开关由每个 agent 注入的 noaEnabledFn 解析,每 run 读
// 一次,故切换只影响之后启动的 run,无需重建 agent。

// enableNoa 在解析器报告开启时把 noa 接入 opts。archiveRoot 是压缩原文的持久化基目录
// (取全局 workDir,各 agent 统一落在 <workDir>/noa 下,不随任务/意图目录分散),sessionID
// 命名其下的归档子目录(全局唯一,故同一基目录内不冲突)。
//
// noa 是实验功能:接入失败不得中断真实任务。发生错误时经 onWarn 上报并回退内置压缩。
// 启用成功时清掉 opts.Compaction,避免 agentcore 因「两个上下文管理器同时设置」告警。
func enableNoa(opts *agentcore.Options, enabled func() bool, archiveRoot, sessionID string, onWarn func(string)) {
	if enabled == nil || !enabled() {
		return
	}
	if opts.OnWarn == nil {
		opts.OnWarn = onWarn
	}
	windowTokens := 0
	if opts.Compaction != nil {
		windowTokens = opts.Compaction.ContextWindow
	}
	if err := noaadapter.Enable(opts, noaadapter.Options{
		ArchiveBaseDir: filepath.Join(archiveRoot, "noa"),
		SessionID:      sessionID,
		Config:         buildNoaConfig(windowTokens),
		OnWarn:         onWarn,
	}); err != nil {
		if onWarn != nil {
			onWarn("noa 压缩启用失败,回退内置压缩:" + err.Error())
		}
		return
	}
	// Compactor 覆盖 Compaction,但两者并存时 agentcore 每次会告警;明确清掉。
	opts.Compaction = nil
}
