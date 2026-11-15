package server

import (
	"context"

	"fmt"
	"github.com/Autumn-27/artex/intercept"
	"log"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/Autumn-27/artex/agent"
	"github.com/Autumn-27/artex/db"
)

// 任务级超时协调器(见 docs/任务级超时与收尾设计.md §4/§8.5)。
// 绝对墙钟:每个带 timeout 的任务一个定时 goroutine,到点驱动有序收尾时序:
//   ① settling → ② worker 停领新意图 / ③ planner 丢弃普通 notify
//   ④ 等在跑 worker drain(受 grace) → ⑤ 终局一轮 planner 判定 → ⑥ 定终态(带守卫)

const (
	// settleDrainGrace 是等在跑 worker 优雅收尾的上限;超过则硬 cancel。
	//
	// 为什么从 90s 提到 300s(2026-10-21 实测修正)：任务到点时可能同时有
	// **12 个 worker** 在跑(worker 数就是并发上限),每个都要走一遍 SDK 的
	// settlement(注入收尾词 + 禁 Bash + 写回成果 + 收尾总结)——那至少是
	// 1 轮 LLM 调用,而思考型模型(minimax-m3.1)一轮就要 30~60 秒。
	//
	// 实测线上任务 3：13:42:23 到点，13:43:53 就 drain 超时硬砍——90 秒里
	// 12 个 worker 只来得及各写回 1~3 条事实，正在写的成果直接丢了。
	// 收尾窗口必须按「并发 worker 数 × 单轮 LLM 耗时」来给，不能按单 worker 给。
	settleDrainGrace     = 300 * time.Second // 等在跑 worker 优雅收尾的上限;超过则硬 cancel
	deadlinePollInterval = 2 * time.Second   // deadline 未盖章/LLM 未就绪时的轮询间隔
	deadlineMaxSleep     = 30 * time.Second  // 单次最长睡眠(便于周期复查终态)
)

// ---------- settling 状态 ----------

func (e *Engine) isSettling(taskID string) bool {
	_, ok := e.settling.Load(taskID)
	return ok
}

// markSettling records the settle reason; returns true only for the first caller.
func (e *Engine) markSettling(taskID, reason string) bool {
	if reason == "" {
		reason = "timeout"
	}
	_, loaded := e.settling.LoadOrStore(taskID, reason)
	return !loaded
}

// ---------- 在跑计数(worker.Execute + planner.Plan),用于 drain ----------

func (e *Engine) inflightCounter(taskID string) *int64 {
	v, _ := e.inflight.LoadOrStore(taskID, new(int64))
	return v.(*int64)
}

// beginTaskOperation atomically registers a task-owned operation unless deletion
// has already installed its barrier. The delete handler can therefore wait for
// inflight==0 without a check-then-start race recreating files after cleanup.
func (e *Engine) beginTaskOperation(taskID string) bool {
	e.deleteMu.RLock()
	defer e.deleteMu.RUnlock()
	if e.IsDeleting(taskID) {
		return false
	}
	atomic.AddInt64(e.inflightCounter(taskID), 1)
	return true
}

func (e *Engine) decInflight(taskID string) { atomic.AddInt64(e.inflightCounter(taskID), -1) }
func (e *Engine) inflightCount(taskID string) int64 {
	return atomic.LoadInt64(e.inflightCounter(taskID))
}

// ---------- deadline ----------

// taskDeadline returns the task's absolute deadline (unix). Prefers the in-process
// map (stamped this session); falls back to the DB-loaded value (restart), seeding
// the map. 0 = no timeout / not yet stamped.
func (e *Engine) taskDeadline(t *Task) int64 {
	if v, ok := e.deadline.Load(t.ID); ok {
		return v.(int64)
	}
	deadlineAt := t.lifecycleSnapshot().DeadlineAt
	if deadlineAt > 0 {
		e.deadline.Store(t.ID, deadlineAt)
		return deadlineAt
	}
	return 0
}

// resetTerminalRevival clears per-run state after PostgreSQL has atomically
// committed <terminal status> -> running and reset first_run_at/deadline_at.
// The configured TimeoutSeconds remains on Task, so the next real Planner/Worker
// run stamps a fresh full budget. coordStarted is reset because the coordinator
// that settled the previous run has already completed (or is in its final return
// path) — a revived task must get a live coordinator again, otherwise its new
// deadline would never be enforced.
//
// Covers every terminal status (timeout / done / failed), not just timeout:
// reviving a "done" task hours after its original deadline used to leave an
// expired deadline_at behind, and the restarted coordinator settled the revived
// task within seconds — silently swallowing the human follow-up intent (task 13,
// 2026-11-05: follow-up #7470 created 16:36:16, final wrap-up round 16:36:17).
func (e *Engine) resetTerminalRevival(taskID string) {
	e.settling.Delete(taskID)
	e.deadline.Delete(taskID)
	e.stamped.Delete(taskID)
	e.coordStarted.Delete(taskID)
}

// stampFirstRun records first_run_at + deadline_at on the FIRST real run (LLM ready)
// of a timeout task, once per process. No-op when the task has no timeout.
func (e *Engine) stampFirstRun(t *Task) {
	if t.TimeoutSeconds <= 0 {
		return
	}
	if _, loaded := e.stamped.LoadOrStore(t.ID, true); loaded {
		return
	}
	dl, err := e.m.StampTaskFirstRun(t.ID)
	if err != nil {
		log.Printf("[deadline] task %s 盖章 first_run 失败: %v", t.ID, err)
		e.stamped.Delete(t.ID) // 允许下次重试
		return
	}
	if dl > 0 {
		e.deadline.Store(t.ID, dl)
		log.Printf("[deadline] task %s 首次运行,截止于 %s", t.ID, time.Unix(dl, 0).Format("2006-01-02 15:04:05"))
	}
}

// clockCtx layers the task's TaskClock (absolute deadline) onto a run's context so
// worker/planner can clamp their wall-clock budget and pick per-run vs task-timeout
// wrap-up words. final marks the coordinator-driven terminal planner round.
func (e *Engine) clockCtx(base context.Context, t *Task, final bool, reason string) context.Context {
	dl := e.taskDeadline(t)
	if dl <= 0 && !final {
		return base // no timeout → unchanged behavior
	}
	return agent.WithTaskClock(base, agent.TaskClock{DeadlineUnix: dl, Final: final, FinalReason: reason})
}

// ---------- 协调器 ----------

// needsNoResultCoordinator（fork 无成果止损）：即使没有 deadline，
// 只要配了 no_result 且尚未检查/未止损，也需要协调器盯守。
func needsNoResultCoordinator(t *Task) bool {
	state := t.lifecycleSnapshot()
	return t.NoResultSeconds > 0 && (state.NoResultCheckedAt == 0 || state.StopReason == db.StopReasonNoResult)
}

// startDeadlineCoordinator launches the per-task deadline timer once (idempotent).
// Called from Run() and from the restart reload path, so non-active timeout tasks
// still get settled after their deadline even without live planner/worker loops.
func (e *Engine) startDeadlineCoordinator(ctx context.Context, t *Task) {
	if t == nil || (t.TimeoutSeconds <= 0 && !needsNoResultCoordinator(t)) {
		return
	}
	e.deleteMu.RLock()
	if e.IsDeleting(t.ID) {
		e.deleteMu.RUnlock()
		return
	}
	if _, loaded := e.coordStarted.LoadOrStore(t.ID, true); loaded {
		e.deleteMu.RUnlock()
		return
	}
	rt := e.registerTaskRoutines(ctx, t.ID, 1)
	e.deleteMu.RUnlock()
	runTaskRoutine(rt, func(loopCtx context.Context) { e.deadlineCoordinator(loopCtx, t) })
}

// deadlineCoordinator waits until the task's absolute deadline, then runs the settle
// sequence. Absolute wall-clock: it keeps counting through pauses.
func (e *Engine) deadlineCoordinator(ctx context.Context, t *Task) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		// fork 无成果止损恢复：进程重启前已提交失败检查点（NoResultCheckedAt>0
		// 且 StopReason=no_result）但没收尾——恢复后由协调器补收尾。
		state := t.lifecycleSnapshot()
		if state.NoResultCheckedAt > 0 && state.StopReason == db.StopReasonNoResult && !isTerminalStatus(e.m.TaskStatus(t.ID)) {
			e.settleTaskWithReason(ctx, t, db.StopReasonNoResult, "无成果止损（恢复）")
			e.coordStarted.Delete(t.ID)
			return
		}
		if isTerminalStatus(e.m.TaskStatus(t.ID)) {
			// fork：终态退出时清掉本协调器的启动标记——复活（resetTerminalRevival
			// 只在走 HTTP 复活路径时清）之外的路径（终验直接判完成）不能留下
			// 残留标记，否则同进程内任务重启后永远起不了新协调器。
			e.coordStarted.Delete(t.ID)
			return // already finished (goals met / failed) — nothing to time out
		}
		dl := e.taskDeadline(t)
		if dl <= 0 {
			// fork：没有 deadline 也没有待办的 no-result 职责时协调器直接退出——
			// 否则纯 no-result 任务在检查完成后仍每 2s 空转直到 ctx 取消（测试
			// 用 t.Context() 无超时，直接挂死）。有 no-result 待办时保持轮询。
			if t.TimeoutSeconds <= 0 {
				s := t.lifecycleSnapshot()
				if t.NoResultSeconds <= 0 || s.NoResultCheckedAt > 0 {
					e.coordStarted.Delete(t.ID)
					return
				}
			}
			if sleepCtx(ctx, deadlinePollInterval) { // not yet stamped (task hasn't really run)
				return
			}
			continue
		}
		if remaining := time.Until(time.Unix(dl, 0)); remaining > 0 {
			nap := remaining
			if nap > deadlineMaxSleep {
				nap = deadlineMaxSleep
			}
			if sleepCtx(ctx, nap) {
				return
			}
			continue
		}
		e.settleTask(ctx, t)
		return
	}
}

// taskGoalProgress 返回本任务目标的达成数与总数（无目标时 0/0，视为无需证明）。
func taskGoalProgress(t *Task) (met, total int) {
	if t == nil || t.Store == nil {
		return 0, 0
	}
	goals, err := t.Store.ListByKind(db.KindGoal, 1000)
	if err != nil {
		return 0, 0
	}
	for _, g := range goals {
		total++
		if g.State == "met" {
			met++
		}
	}
	return met, total
}

// settleTerminalStatus 计算收尾终态（红队版 v0.6.31 下放）。
//
// 语义：只有「目标全部达成」才允许 done；存在未达成目标时，无论收尾原因是什么
// （超时 / 自动空转 / 覆盖度 / 轮数上限），终态一律 timeout——UI 据此显示
// 「未完成」，而不是把做了一半的任务标成完成。totalGoals==0 的任务沿用旧语义。
func settleTerminalStatus(allGoalsMet bool, reason string, finalRoundMet bool) string {
	// fork 无成果止损：这是「人工设定的止损停车」，不是「跑完没达标」——
	// 状态复用 timeout（终态语义不变），但 settle 本身已表明任务被主动停止。
	if reason == db.StopReasonNoResult {
		return "timeout"
	}
	if !allGoalsMet {
		return "timeout"
	}
	if reason == "timeout" && !finalRoundMet {
		return "timeout"
	}
	return "done"
}

// checkNoResultCheckpoint serializes the decision with terminal transitions and
// revival. Once marked, later findings cannot reverse the decision.
// （RuoJi6 无成果止损嫁接进 Pojun reason 架构：命中即以 StopReasonNoResult 收尾。）
func (e *Engine) checkNoResultCheckpoint(t *Task) (bool, error) {
	e.m.taskStateMu.Lock()
	defer e.m.taskStateMu.Unlock()
	state := t.lifecycleSnapshot()
	if state.NoResultCheckedAt > 0 || isTerminalStatus(state.Status) || noResultRemaining(t.NoResultSeconds, state, time.Now().Unix()) > 0 {
		return false, nil
	}
	id, err := strconv.ParseInt(t.ID, 10, 64)
	if err != nil {
		return false, err
	}
	minimum := max(1, t.NoResultMinFindings)
	count, stop, checked, err := e.m.pg.CheckNoResultCheckpoint(context.Background(), id, minimum, t.NoResultMinSeverity)
	if err != nil || checked == nil {
		return false, err
	}
	t.updateLifecycle(func(s *taskLifecycleState) {
		s.NoResultCheckedAt = checked.Unix()
		if stop {
			s.StopReason = db.StopReasonNoResult
		}
	})
	severity := t.NoResultMinSeverity
	if severity == "" {
		severity = "不限"
	} else {
		severity += "及以上"
	}
	summary := fmt.Sprintf("无成果检查通过：达标漏洞 %d/%d（等级%s），任务继续运行，之后不再检查", count, minimum, severity)
	if stop {
		runtime := taskRunSeconds(state.ActiveRunSeconds, state.RunSegmentStartedAt, checked.Unix(), state.Status, state.Paused, state.Queued) - state.NoResultBaseSeconds
		summary = fmt.Sprintf("无成果止损：已运行 %s，达标漏洞 %d/%d（等级%s），任务自动停止", time.Duration(runtime)*time.Second, count, minimum, severity)
	}
	e.emitActivity(t, db.Activity{Worker: "system", Kind: "text", Summary: summary})
	return stop, nil
}

// settleTask runs the ordered settle sequence once (§4 steps ①–⑥).
func (e *Engine) settleTask(ctx context.Context, t *Task) {
	e.settleTaskWithReason(ctx, t, "timeout", "到达任务超时上限")
}

// settleTaskWithReason 是统一收尾入口。reason:
//   - timeout      任务绝对 deadline（终态 met?done:timeout）
//   - goal_met     planner 明确判定目标达成且无在跑意图（不再跑终局轮，直接 done）
//   - auto_idle / auto_coverage / auto_rounds 程序化自动收尾（终态 done）
func (e *Engine) settleTaskWithReason(ctx context.Context, t *Task, reason, detail string) {
	if !e.markSettling(t.ID, reason) {
		return
	}
	t.clearSettle()
	label := reason
	switch reason {
	case "timeout":
		label = "到达超时上限"
	case "goal_met":
		label = "planner 判定目标达成且无在跑意图"
	case "auto_idle", "auto_coverage", "auto_rounds":
		label = "自动收尾：" + detail
	case db.StopReasonNoResult:
		label = "无成果止损：运行满设定时长且达标漏洞数不足"
	}
	log.Printf("[settle] task %s 进入收尾（%s）", t.ID, label)
	if reason != "timeout" {
		e.emitActivity(t, db.Activity{Worker: "system", Kind: "text",
			Summary: fmt.Sprintf("自动收尾：%s", detail)})
	}
	// ④ 等在跑 worker/planner drain(在跑 run 因夹逼的 MaxDuration 自行进收尾);
	// 超过 grace 仍未清空 → 硬 cancel 该任务 exec ctx(settling-aware 分支正确归类)。
	hardStop := time.Now().Add(settleDrainGrace)
	for e.inflightCount(t.ID) > 0 {
		if time.Now().After(hardStop) {
			log.Printf("[settle] task %s drain 超时(%s),硬取消在跑 run", t.ID, settleDrainGrace)
			// 无成果止损停车的任务，drain 硬取消原因要归到 no-result（终态归类与统计）。
			cause := agent.AbortSettleDrainTimeout
			if reason == db.StopReasonNoResult {
				cause = agent.AbortNoResultStop
			}
			e.cancelExec(t.ID, cause)
			_ = sleepCtx(ctx, 3*time.Second) // 给 worker 分支一点时间落库/归类
			break
		}
		if sleepCtx(ctx, 500*time.Millisecond) {
			return // 引擎整体关停
		}
	}

	// ⑤ 终局一轮 planner。goal_met 已经是 planner 的最终判定，不再多跑一轮；
	// timeout 用超时收尾词，自动收尾用“按策略收尾”词。
	met := reason == "goal_met"
	if reason != "goal_met" {
		met = e.runFinalPlannerRound(ctx, t, reason)
	}
	if !e.beginTaskOperation(t.ID) {
		return
	}
	defer e.decInflight(t.ID)

	// ⑤′ 落终态前点名缺报告的漏洞（终局后不再派 worker，只能提示人补齐）。
	e.warnFindingsWithoutReport(t)

	// ⑥ 定终态(带守卫)：只有「目标全部被证明达成」才算 done。
	// 红队版 v0.6.31 下放：自动收尾（idle/coverage/轮数）与 goal_met 在目标未达成时
	// 一律落 timeout，由界面显示「未完成·已停止」——旧的「自动收尾统一 done」正是
	// 「只做了一部分却显示完成」的来源。
	metGoals, totalGoals := taskGoalProgress(t)
	allGoalsMet := totalGoals == 0 || metGoals >= totalGoals
	status := settleTerminalStatus(allGoalsMet, reason, met)
	if status == "timeout" && !allGoalsMet {
		log.Printf("[settle] task %s 目标未全部达成(%d/%d)，终态记 timeout（未完成）而非 done", t.ID, metGoals, totalGoals)
	}
	// fork 无成果止损：StopReason 必须随终态落库（SetTaskStatusGuarded 不带
	// reason 会清掉 no-result 语义，界面与复活判断都依赖它）。
	stopReasonForSettle := ""
	if reason == db.StopReasonNoResult {
		stopReasonForSettle = db.StopReasonNoResult
	}
	won, err := e.m.SetTaskStatusGuardedReason(t.ID, status, stopReasonForSettle)
	switch {
	case err != nil:
		log.Printf("[settle] task %s 落终态失败: %v", t.ID, err)
	case won:
		log.Printf("[settle] task %s 收尾完成,终态=%s（%s）", t.ID, status, label)
		e.emitActivity(t, db.Activity{Worker: "system", Kind: "text",
			Summary: fmt.Sprintf("任务收尾完成：%s；终态 %s", label, status)})
	default:
		log.Printf("[settle] task %s 收尾时已是终态,保留原状态", t.ID)
	}
}

// runFinalPlannerRound drives exactly ONE terminal planner round with the
// task-timeout planner words (final goal judgment; no new intents). Waits for the
// LLM to be ready (bounded by ctx) so a completable task isn't mis-judged timeout.
func (e *Engine) runFinalPlannerRound(ctx context.Context, t *Task, reason string) (met bool) {
	if e.IsDeleting(t.ID) {
		return false
	}
	planner, _ := e.snapshotFor(t)
	for planner == nil {
		if sleepCtx(ctx, deadlinePollInterval) {
			return false
		}
		if isTerminalStatus(e.m.TaskStatus(t.ID)) {
			return false
		}
		if e.IsDeleting(t.ID) {
			return false
		}
		planner, _ = e.snapshotFor(t)
	}
	// 独立 ctx(不挂 execCancel,避免 pause/硬 cancel 打断这最后一轮),带 Final 注入
	// 收尾词：timeout 用任务超时词，自动收尾用“按策略结束”词。
	fctx := e.clockCtx(ctx, t, true, reason)
	fctx = intercept.WithAutopilot(fctx, t.AutopilotEnabled) // 自主推进提示词尾巴（settle 轮）
	if !e.beginTaskOperation(t.ID) {
		return false
	}
	defer e.decInflight(t.ID)
	emit := func(r db.Activity) { e.emitActivity(t, r) }
	label := "任务超时收尾"
	switch {
	case reason == db.StopReasonNoResult:
		label = "无成果止损收尾"
	case reason != "timeout":
		label = fmt.Sprintf("自动收尾(原因 %s)", reason)
	}
	summary := fmt.Sprintf("%s·终局判定(第 %d 轮)", label, e.nextPlannerRound(t.ID))
	e.emitActivity(t, db.Activity{Worker: "planner", Kind: "round", Summary: summary})
	tTaskID, _ := strconv.ParseInt(t.ID, 10, 64)
	e.BeginLLMCall(t.ID)
	// 终局轮与普通规划轮同款墙钟护栏：fctx 是无 deadline 的 server 根 ctx，而
	// settleTask 在等这一步落终态——Plan 若卡死（上游半开连接/compactor 自死锁，
	// 两类都是线上发生过的事故），任务会永远停在 settling。超时=按「未达成」
	// 返回，走正常的超时收尾流程；卡死的 goroutine 泄漏是两害取其轻。
	roundCtx, roundCancel := context.WithTimeout(fctx, plannerRoundLimit())
	defer roundCancel()
	res := runPlannerRound(roundCtx, func(pctx context.Context) (bool, string, error) {
		return planner.Plan(pctx, tTaskID, e.m.assets, t.Store, t.Goal, t.drainTriggers(), emit)
	})
	e.EndLLMCall(t.ID)
	if res.err != nil && roundCtx.Err() != nil {
		log.Printf("[deadline] task %s 终局规划超时(%s)，按未达成继续收尾", t.ID, plannerRoundLimit())
		return false
	}
	if res.err != nil {
		log.Printf("[deadline] task %s 终局规划出错: %v", t.ID, res.err)
	} else if res.met {
		log.Printf("[deadline] task %s 终局判定目标达成: %s", t.ID, res.reason)
	}
	return res.met
}
