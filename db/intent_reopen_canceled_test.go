package db

import "testing"

// TestReopenCanceledIntents 只重开「服务停止被取消」的 blocked 意图
// （部署重启遗留物），用户手动 block 的（原因不含 context canceled）保持不动。
// 背景：v0.4.56 复盘发现任务 6/8/9 共 11 条 canceled blocked 意图永久卡死。
func TestReopenCanceledIntents(t *testing.T) {
	d, err := Open(testDSN(t))
	if err != nil {
		t.Skipf("postgres unavailable (%v) — skipping", err)
	}
	defer d.Close()
	expID, err := d.CreateExploration("reopen canceled", "复活自愈回归")
	if err != nil {
		t.Fatal(err)
	}
	es := d.Exploration(expID)

	canceled1 := mustIntent(t, es, "重启时被取消 1")
	// 真实链路：worker 因服务停止把意图置 blocked 并写取消原因。
	if err := es.SetIntentBlockedReason(canceled1, "error: context canceled"); err != nil {
		t.Fatal(err)
	}
	if err := es.SetIntentState(canceled1, "blocked"); err != nil {
		t.Fatal(err)
	}
	if _, err := es.db.Exec(
		`UPDATE exploration_nodes SET blocked_reason='error: context canceled' WHERE id=$1`, canceled1); err != nil {
		t.Fatal(err)
	}

	userBlocked := mustIntent(t, es, "用户手动阻止")
	if _, err := es.db.Exec(
		`UPDATE exploration_nodes SET state='blocked', blocked_reason='用户要求暂停该方向' WHERE id=$1`, userBlocked); err != nil {
		t.Fatal(err)
	}

	doneIntent := mustIntent(t, es, "已完成的意图不受影响")
	if err := es.SetIntentState(doneIntent, "done"); err != nil {
		t.Fatal(err)
	}

	n, err := es.ReopenCanceledIntents()
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("应只重开 1 条 canceled 意图，实际 %d", n)
	}
	after, err := es.GetNode(canceled1)
	if err != nil {
		t.Fatal(err)
	}
	if after == nil || after.State != "open" || after.BlockedReason != "" {
		t.Fatalf("canceled 意图未恢复 open: %+v", after)
	}
	ub, err := es.GetNode(userBlocked)
	if err != nil {
		t.Fatal(err)
	}
	if ub == nil || ub.State != "blocked" {
		t.Fatalf("用户手动 block 的意图不应被重开: %+v", ub)
	}
	dn, err := es.GetNode(doneIntent)
	if err != nil {
		t.Fatal(err)
	}
	if dn == nil || dn.State != "done" {
		t.Fatalf("done 意图不应受影响: %+v", dn)
	}
}
