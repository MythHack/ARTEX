package server

import (
	"context"
	"strings"
	"testing"

	"github.com/Autumn-27/artex/agent"
)

// v0.4.60：技能速查块必须注入 system prompt——Skill 是元工具，模型不被
// system prompt 显式提醒就几乎不调（56k 调用里 Skill 仅 8 次、任务 18 零次）。
func TestSkillIndexBlockCoversBindings(t *testing.T) {
	m, err := NewManager(t.TempDir(), "")
	if err != nil {
		t.Skipf("postgres unavailable (%v)", err)
	}
	defer m.Close()
	s := New(context.Background(), m, "../skills", t.TempDir(), t.TempDir())
	defer s.Close()

	worker, err := s.m.pg.GetAgentByKey("worker")
	if err != nil {
		t.Fatal(err)
	}
	// 给 worker 绑两个 skill（模拟用户/默认绑定）。
	if err := s.m.pg.SetAgentSkillVisibility(worker.ID, []string{"vuln-hunting", "web-attack-methods"}); err != nil {
		t.Fatal(err)
	}
	blk := agent.SkillIndexBlock("worker")
	if blk == "" {
		t.Fatal("worker 有绑定但速查块为空")
	}
	if !strings.Contains(blk, "vuln-hunting") || !strings.Contains(blk, "web-attack-methods") {
		t.Fatalf("速查块缺绑定项：%s", blk)
	}
	if !strings.Contains(blk, "Skill(") {
		t.Fatalf("速查块缺调用方式说明：%s", blk)
	}
	// 未绑定 agent（goals 默认无绑定）→ 空块，不污染提示。
	if blk := agent.SkillIndexBlock("goals"); blk != "" {
		t.Fatalf("无绑定 agent 不应有速查块：%s", blk)
	}
}
