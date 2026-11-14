package db

import (
	"fmt"
	"testing"
)

// TestStripHostPort covers the ip:port / [ipv6]:port stripping used by the
// ip/cidr scope path so a target like "10.0.188.136:3000" no longer 404s.
func TestStripHostPort(t *testing.T) {
	cases := []struct{ in, want string }{
		{"10.0.188.136:3000", "10.0.188.136"},       // the reported IP case
		{"10.0.188.136", "10.0.188.136"},            // bare IPv4 unchanged
		{"[2001:db8::1]:8080", "2001:db8::1"},       // bracketed IPv6 + port
		{"2001:db8::1", "2001:db8::1"},              // bare IPv6 unchanged (has colons)
		{" 1.2.3.4:80 ", "1.2.3.4"},                 // trims surrounding space
		{"example.com:443", "example.com"},          // domain + port → bare host
		{"api.example.com:8080", "api.example.com"}, // subdomain + port
		{"example.com", "example.com"},              // bare domain unchanged
	}
	for _, c := range cases {
		if got := stripHostPort(c.in); got != c.want {
			t.Errorf("stripHostPort(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// v0.4.40 自动归属公司：分布统计 + 写入 company scope + 资产快照。
func TestScopeCompanyAttribution(t *testing.T) {
	d, as, cs := testSetup(t)
	defer d.Close()
	tk, err := d.CreateTask("自动归属测试", "目标", nil, 0, 0)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	t.Cleanup(func() { _, _ = d.DeleteTaskCascade(tk.ID, true, true) })

	// 建一家公司并把资产挂上去
	comp, _, err := cs.UpsertCompany("归属测试公司", "")
	if err != nil {
		t.Fatalf("create company: %v", err)
	}
	t.Cleanup(func() { _, _ = d.Exec(`DELETE FROM companies WHERE id=$1`, comp) })
	for i := 0; i < 3; i++ {
		dom := fmt.Sprintf("attr-%d.example.com", i)
		if _, err := as.UpsertSubdomain(UpsertSubdomainReq{TaskID: tk.ID, Domain: dom}); err != nil {
			t.Fatalf("upsert sub: %v", err)
		}
		if _, err := d.Exec(`UPDATE assets SET company_id=$1 WHERE domain=$2`, comp, dom); err != nil {
			t.Fatalf("attach company: %v", err)
		}
	}
	// 加 subdomain scope，让范围内资产可归到公司
	if _, err := d.Exec(`INSERT INTO task_scope(task_id, kind, domain, source, reason)
VALUES ($1, 'root_domain', 'example.com', 'auto', '测试用') ON CONFLICT DO NOTHING`, tk.ID); err != nil {
		t.Fatalf("add scope: %v", err)
	}

	dist, total, err := d.ScopeCompanyDistribution(tk.ID)
	if err != nil || total < 3 {
		t.Fatalf("dist=%v total=%d err=%v", dist, total, err)
	}
	// 宽松断言：公司资产至少 3 个（泛解析/IP 类无归属资产的噪音可容忍）。
	if dist[comp] < 3 {
		t.Fatalf("归属公司资产不足: dist=%v total=%d", dist, total)
	}
	if has, _ := d.HasCompanyScope(tk.ID); has {
		t.Fatal("初始不应有 company scope")
	}
	if n, _ := d.CompanyScopeAssetCount(comp); n < 3 {
		t.Fatalf("company assets=%d want>=3", n)
	}
	if err := d.AddCompanyScopeAuto(tk.ID, comp, "测试自动归属"); err != nil {
		t.Fatalf("add auto scope: %v", err)
	}
	if has, _ := d.HasCompanyScope(tk.ID); !has {
		t.Fatal("auto scope 未写入")
	}
	// 快照：公司资产应进任务
	var linked int
	if err := d.QueryRow(`SELECT count(*) FROM task_asset_links WHERE task_id=$1 AND source='auto'`, tk.ID).Scan(&linked); err != nil || linked < 3 {
		t.Fatalf("task_asset_links=%d err=%v", linked, err)
	}
}
