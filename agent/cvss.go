package agent

import (
	"fmt"
	"math"
	"regexp"
	"strings"
)

var cwePattern = regexp.MustCompile(`(?i)^(?:CWE-)?(\d+)$`)

// NormalizeCWE normalizes a CWE identifier such as "200" or "cwe-200" to "CWE-200".
func NormalizeCWE(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if m := cwePattern.FindStringSubmatch(raw); len(m) == 2 {
		return "CWE-" + m[1]
	}
	return strings.ToUpper(raw)
}

// ComputeCVSS31 parses a CVSS v3.0 / v3.1 vector string (with or without the
// "CVSS:3.1/" prefix), calculates the exact Base Score in [0.0, 10.0] using the
// official FIRST CVSS v3.1 specification (including Roundup to 1 decimal place),
// and maps it to ARTEX severity ("critical", "high", "medium", "low", or "none").
func ComputeCVSS31(vector string) (score float64, severity string, normalizedVector string, err error) {
	v := strings.TrimSpace(vector)
	if v == "" {
		return 0, "", "", fmt.Errorf("cvss_vector is empty")
	}
	upper := strings.ToUpper(v)
	upper = strings.TrimPrefix(upper, "CVSS:3.1/")
	upper = strings.TrimPrefix(upper, "CVSS:3.0/")

	metrics := map[string]string{}
	for _, part := range strings.Split(upper, "/") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, ":", 2)
		if len(kv) != 2 {
			return 0, "", "", fmt.Errorf("invalid CVSS metric %q in vector %q", part, vector)
		}
		metrics[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
	}

	required := []string{"AV", "AC", "PR", "UI", "S", "C", "I", "A"}
	for _, k := range required {
		if metrics[k] == "" {
			return 0, "", "", fmt.Errorf("missing required CVSS v3.1 metric %q in %q", k, vector)
		}
	}

	var av float64
	switch metrics["AV"] {
	case "N":
		av = 0.85
	case "A":
		av = 0.62
	case "L":
		av = 0.55
	case "P":
		av = 0.20
	default:
		return 0, "", "", fmt.Errorf("invalid AV value %q", metrics["AV"])
	}

	var ac float64
	switch metrics["AC"] {
	case "L":
		ac = 0.77
	case "H":
		ac = 0.44
	default:
		return 0, "", "", fmt.Errorf("invalid AC value %q", metrics["AC"])
	}

	scopeChanged := false
	switch metrics["S"] {
	case "U":
		scopeChanged = false
	case "C":
		scopeChanged = true
	default:
		return 0, "", "", fmt.Errorf("invalid S value %q", metrics["S"])
	}

	var pr float64
	switch metrics["PR"] {
	case "N":
		pr = 0.85
	case "L":
		if scopeChanged {
			pr = 0.68
		} else {
			pr = 0.62
		}
	case "H":
		if scopeChanged {
			pr = 0.50
		} else {
			pr = 0.27
		}
	default:
		return 0, "", "", fmt.Errorf("invalid PR value %q", metrics["PR"])
	}

	var ui float64
	switch metrics["UI"] {
	case "N":
		ui = 0.85
	case "R":
		ui = 0.62
	default:
		return 0, "", "", fmt.Errorf("invalid UI value %q", metrics["UI"])
	}

	ciaWeight := func(name, val string) (float64, error) {
		switch val {
		case "H":
			return 0.56, nil
		case "L":
			return 0.22, nil
		case "N":
			return 0.0, nil
		default:
			return 0, fmt.Errorf("invalid %s value %q", name, val)
		}
	}
	cWeight, err := ciaWeight("C", metrics["C"])
	if err != nil {
		return 0, "", "", err
	}
	iWeight, err := ciaWeight("I", metrics["I"])
	if err != nil {
		return 0, "", "", err
	}
	aWeight, err := ciaWeight("A", metrics["A"])
	if err != nil {
		return 0, "", "", err
	}

	normalizedVector = fmt.Sprintf("CVSS:3.1/AV:%s/AC:%s/PR:%s/UI:%s/S:%s/C:%s/I:%s/A:%s",
		metrics["AV"], metrics["AC"], metrics["PR"], metrics["UI"],
		metrics["S"], metrics["C"], metrics["I"], metrics["A"])

	iss := 1.0 - (1.0-cWeight)*(1.0-iWeight)*(1.0-aWeight)
	if iss <= 0 {
		return 0.0, "none", normalizedVector, nil
	}

	var impact float64
	if !scopeChanged {
		impact = 6.42 * iss
	} else {
		impact = 7.52*(iss-0.029) - 3.25*math.Pow(iss-0.02, 15)
	}
	if impact <= 0 {
		return 0.0, "none", normalizedVector, nil
	}

	exploitability := 8.22 * av * ac * pr * ui
	var rawScore float64
	if !scopeChanged {
		rawScore = math.Min(impact+exploitability, 10.0)
	} else {
		rawScore = math.Min(1.08*(impact+exploitability), 10.0)
	}
	score = cvssRoundUp(rawScore)

	switch {
	case score >= 9.0:
		severity = "critical"
	case score >= 7.0:
		severity = "high"
	case score >= 4.0:
		severity = "medium"
	case score > 0.0:
		severity = "low"
	default:
		severity = "none"
	}
	return score, severity, normalizedVector, nil
}

// cvssRoundUp implements the official FIRST CVSS v3.1 Roundup function using
// integer arithmetic: round_to_nearest_int(input * 100000), and if not divisible
// by 10000, step up to the next tenth.
func cvssRoundUp(input float64) float64 {
	intInput := int64(math.Round(input * 100000.0))
	if intInput%10000 == 0 {
		return float64(intInput) / 100000.0
	}
	return (math.Floor(float64(intInput)/10000.0) + 1.0) / 10.0
}

func severityRank(sev string) int {
	switch strings.ToLower(strings.TrimSpace(sev)) {
	case "critical":
		return 4
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

func containsAnyFold(s string, patterns ...string) bool {
	lower := strings.ToLower(s)
	for _, p := range patterns {
		if strings.Contains(lower, strings.ToLower(p)) {
			return true
		}
	}
	return false
}

// hasHighValueCredentialOrBatchProof checks whether the finding text/evidence
// documents actual high-impact exploitation (working credentials/keys, RCE,
// admin takeover, or bulk sensitive records) rather than metadata/primitives.
func hasHighValueCredentialOrBatchProof(combined string) bool {
	return containsAnyFold(combined,
		"appsecret", "private_key", "rsa2048", "私钥", "数据库凭据",
		"验签通过", "离线复算", "登录成功", "后台接管", "rce", "命令执行",
		"远程代码执行", "webshell", "翻转成功", "状态翻转",
	)
}

// calibrateCVSSVector enforces evidence-anchored C/I/A impact ceilings on the
// parsed CVSS v3.1 metrics so the model cannot inflate metadata exposure or
// unverified primitives to High/Critical by setting C:H or I:H without proof.
func calibrateCVSSVector(cwe, vulnClass, name, summary, evidence, normVec string) string {
	combined := strings.Join([]string{cwe, vulnClass, name, summary, evidence}, " ")
	if hasHighValueCredentialOrBatchProof(combined) {
		return normVec
	}
	upper := strings.TrimPrefix(normVec, "CVSS:3.1/")
	metrics := map[string]string{}
	for _, part := range strings.Split(upper, "/") {
		kv := strings.SplitN(part, ":", 2)
		if len(kv) == 2 {
			metrics[kv[0]] = kv[1]
		}
	}

	// Rule 1: Stack trace / path disclosure / directory listing / PHP error / self-signed cert → Low (3.7).
	if containsAnyFold(combined,
		"cwe-209", "cwe-548", "cwe-295", "display_errors", "autoindex", "目录列表",
		"路径泄露", "绝对路径", "自签名证书", "堆栈泄露", "debug 模式", "yii_debug",
	) {
		metrics["AC"] = "H"
		metrics["S"] = "U"
		if metrics["C"] == "H" {
			metrics["C"] = "L"
		}
		metrics["I"] = "N"
		metrics["A"] = "N"
	} else if containsAnyFold(combined,
		"/metrics", "prometheus", "actuator", "swagger", "openapi", "autodiscover",
		"存在性枚举", "存在性 oracle", "用户枚举", "枚举 oracle", "白名单",
		"cwe-203", "cwe-204",
	) {
		// Rule 2: Monitoring metrics, Actuator, API docs, or existence/enumeration oracle → cap C at L, I/A at N (5.3 Medium).
		metrics["S"] = "U"
		if metrics["C"] == "H" {
			metrics["C"] = "L"
		}
		if metrics["I"] == "H" {
			metrics["I"] = "N"
		}
		metrics["A"] = "N"
	} else if containsAnyFold(combined,
		"短信轰炸", "sendcaptcha", "requestsms", "开放重定向", "open redirect",
		"cwe-601", "未改单", "未驱动", "无读回链", "假回调", "空盐",
	) {
		// Rule 3: SMS bombing, open redirect, or unverified callback/upload primitive → cap C/I/A at L (Medium).
		if containsAnyFold(combined, "开放重定向", "open redirect", "cwe-601") {
			metrics["UI"] = "R"
		}
		if metrics["C"] == "H" {
			metrics["C"] = "L"
		}
		if metrics["I"] == "H" {
			metrics["I"] = "L"
		}
		if metrics["A"] == "H" {
			metrics["A"] = "N"
		}
	}

	return fmt.Sprintf("CVSS:3.1/AV:%s/AC:%s/PR:%s/UI:%s/S:%s/C:%s/I:%s/A:%s",
		metrics["AV"], metrics["AC"], metrics["PR"], metrics["UI"],
		metrics["S"], metrics["C"], metrics["I"], metrics["A"])
}

// defaultVectorForSeverity derives a baseline CVSS v3.1 vector when a caller
// omits cvss_vector, then passes it through the evidence-anchored calibrator.
func defaultVectorForSeverity(rawSeverity string) string {
	switch strings.ToLower(strings.TrimSpace(rawSeverity)) {
	case "critical":
		return "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:N" // 9.1 Critical
	case "high":
		return "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:N/A:N" // 7.5 High
	case "low":
		return "CVSS:3.1/AV:N/AC:H/PR:N/UI:N/S:U/C:L/I:N/A:N" // 3.7 Low
	default:
		return "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:L/I:N/A:N" // 5.3 Medium
	}
}

// EvaluateFindingSeverity computes and calibrates a finding's CVSS v3.1 score,
// vector, CWE tag, and authoritative severity level on every report_finding call.
func EvaluateFindingSeverity(rawSeverity, rawCWE, rawVector, vulnClass, name, summary, evidence string) (score float64, severity, normVec, normCWE string, err error) {
	normCWE = NormalizeCWE(rawCWE)
	vec := strings.TrimSpace(rawVector)
	if vec == "" {
		vec = defaultVectorForSeverity(rawSeverity)
	}
	score, severity, normVec, err = ComputeCVSS31(vec)
	if err != nil {
		return 0, "", "", normCWE, err
	}
	if severity == "none" || score <= 0 {
		return 0, "none", normVec, normCWE, nil
	}
	calibratedVec := calibrateCVSSVector(normCWE, vulnClass, name, summary, evidence, normVec)
	if calibratedVec != normVec {
		if cScore, cSev, cNorm, cErr := ComputeCVSS31(calibratedVec); cErr == nil && cScore > 0 {
			score, severity, normVec = cScore, cSev, cNorm
		}
	}
	return score, severity, normVec, normCWE, nil
}
