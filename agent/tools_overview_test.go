package agent

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/Autumn-27/artex/db"
	"github.com/Autumn-27/norma/noa"
)

func TestOverviewTextBudgetIsFairAndUTF8Safe(t *testing.T) {
	if got := relatedOverviewBudgetForSources(1); got != relatedOverviewMaxTextPerSource {
		t.Fatalf("single source budget=%d want=%d", got, relatedOverviewMaxTextPerSource)
	}
	perSource := relatedOverviewBudgetForSources(db.MaxTaskSourceCount)
	if perSource*db.MaxTaskSourceCount > relatedOverviewTotalTextRunes {
		t.Fatalf("aggregate budget exceeded: per_source=%d", perSource)
	}

	budget := overviewTextBudget{remaining: 5}
	got := budget.take(strings.Repeat("中", 10), 20)
	if !utf8.ValidString(got) {
		t.Fatalf("budget truncation produced invalid UTF-8: %q", got)
	}
	if utf8.RuneCountInString(got) != 5 || !budget.truncated || budget.remaining != 0 {
		t.Fatalf("unexpected truncation: got=%q budget=%+v", got, budget)
	}
	if tail := budget.take("more", 20); tail != "" {
		t.Fatalf("exhausted budget returned more text: %q", tail)
	}
}

func TestRenderWorkerGraphOverviewSlimsNonWorkerFieldsAndPreservesSchema(t *testing.T) {
	longText := strings.Repeat("探", 400)
	data := map[string]any{
		"coverage":      map[string]any{"pct": 42},
		"open_intents":  []map[string]any{{"id": int64(1), "summary": "queued"}},
		"frontier_open": 5,
		"recent_done_intents": []map[string]any{
			{"id": int64(1)}, {"id": int64(2)}, {"id": int64(3)},
			{"id": int64(4)}, {"id": int64(5)}, {"id": int64(6)}, {"id": int64(7)},
		},
		"recent_facts": make([]map[string]any, 18),
		"finding_list": []map[string]any{
			{"id": int64(10), "summary": longText},
		},
		"cold_digests": []map[string]any{
			{"id": int64(20), "body": longText, "member_count": 4},
		},
		"hints": []map[string]any{
			{"id": int64(1), "state": "active", "text": "【覆盖度仅 33%——规划者排期】继续排期"},
			{"id": int64(2), "state": "active", "text": "【现有漏洞降级清单·报告阶段执行】降级"},
			{"id": int64(3), "state": "active", "text": "【范围红线】禁止探测第三方域"},
		},
	}
	out := renderWorkerGraphOverview(data)
	if strings.Contains(out, `"open_intents"`) || strings.Contains(out, `"coverage"`) {
		t.Fatalf("worker overview retained planner-only fields: %s", out)
	}
	if !strings.Contains(out, `"frontier_open":5`) {
		t.Fatalf("worker overview lost frontier_open count: %s", out)
	}
	if strings.Contains(out, "规划者排期") || strings.Contains(out, "报告阶段执行") {
		t.Fatalf("worker overview retained non-worker hints: %s", out)
	}
	if !strings.Contains(out, "【范围红线】禁止探测第三方域") {
		t.Fatalf("worker overview dropped actionable worker hint: %s", out)
	}
	if rdi := data["recent_done_intents"].([]map[string]any); len(rdi) != 5 {
		t.Fatalf("recent_done_intents len=%d, want 5", len(rdi))
	}
	if rf := data["recent_facts"].([]map[string]any); len(rf) != 12 {
		t.Fatalf("recent_facts len=%d, want 12", len(rf))
	}
}

func TestBuildNoaConfigValidatesCleanly(t *testing.T) {
	for _, win := range []int{0, 32_000, 64_000, 200_000} {
		cfg := buildNoaConfig(win)
		if cfg.ModelContextLimit <= 0 || cfg.ModelContextLimit > defaultNoaContextLimit {
			t.Fatalf("win=%d produced unexpected ModelContextLimit=%d", win, cfg.ModelContextLimit)
		}
		if warns := noa.ValidateConfig(*cfg); len(warns) != 0 {
			t.Fatalf("win=%d produced noa config warnings: %v", win, warns)
		}
	}
}

func TestComputeCVSS31AndNormalizeCWE(t *testing.T) {
	cases := []struct {
		vec     string
		want    float64
		wantSev string
	}{
		{"CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:L/I:N/A:N", 5.3, "medium"},
		{"CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:L/A:N", 8.2, "high"},
		{"CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:C/C:H/I:H/A:H", 10.0, "critical"},
		{"CVSS:3.1/AV:N/AC:H/PR:N/UI:N/S:U/C:L/I:N/A:N", 3.7, "low"},
		{"CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:N/I:N/A:N", 0.0, "none"},
	}
	for _, tc := range cases {
		score, sev, norm, err := ComputeCVSS31(tc.vec)
		if err != nil {
			t.Fatalf("vec=%q unexpected err: %v", tc.vec, err)
		}
		if score != tc.want || sev != tc.wantSev || norm != tc.vec {
			t.Fatalf("vec=%q got (%.1f, %q, %q), want (%.1f, %q, %q)", tc.vec, score, sev, norm, tc.want, tc.wantSev, tc.vec)
		}
	}
	if got := NormalizeCWE("cwe-200"); got != "CWE-200" {
		t.Fatalf("NormalizeCWE=%q, want CWE-200", got)
	}
	if !isUpstreamFilterRefusal("The prompt could not be submitted. The prompt contains sensitive words that violate Google's Generative AI Prohibited Use policy.") {
		t.Fatal("expected Gemini prohibited use refusal to be detected")
	}
	if !isUpstreamFilterRefusal("This request was blocked by Gemini's filters.") {
		t.Fatal("expected Gemini filter block to be detected")
	}
	if isUpstreamFilterRefusal("已完成探测，未发现漏洞。") {
		t.Fatal("normal conclusion falsely flagged as upstream refusal")
	}
}

func TestEvaluateFindingSeverityCalibratesInflatedRatings(t *testing.T) {
	// 1. Model claims "high" and passes C:H for a Prometheus /metrics exposure → calibrated to 5.3 medium (C:L).
	score, sev, vec, cwe, err := EvaluateFindingSeverity(
		"high", "200", "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:N/A:N",
		"敏感信息泄露", "api.example.com /metrics 未授权暴露", "Prometheus 指标暴露", "GET /metrics 200",
	)
	if err != nil || sev != "medium" || score != 5.3 || cwe != "CWE-200" {
		t.Fatalf("/metrics calibration: score=%.1f sev=%q vec=%q cwe=%q err=%v", score, sev, vec, cwe, err)
	}

	// 2. Model omits cvss_vector and claims "high" for a path disclosure / autoindex → calibrated to 3.7 low.
	score, sev, _, _, err = EvaluateFindingSeverity(
		"high", "CWE-209", "",
		"信息泄露", "404 响应泄露服务器绝对路径", "泄露 /usr/lib/code-server 绝对路径", "",
	)
	if err != nil || sev != "low" || score != 3.7 {
		t.Fatalf("path disclosure calibration: score=%.1f sev=%q err=%v", score, sev, err)
	}

	// 3. Model claims "high" for a real credential leak (appSecret verified) → stays high (8.2).
	score, sev, _, _, err = EvaluateFindingSeverity(
		"high", "CWE-798", "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:L/A:N",
		"凭据泄露", "apifox 泄露商户 appId+appSecret", "离线复算验签通过", "appSecret=18b22b",
	)
	if err != nil || sev != "high" || score != 8.2 {
		t.Fatalf("credential leak calibration: score=%.1f sev=%q err=%v", score, sev, err)
	}
}
