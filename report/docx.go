// Package report 的 docx 子模块:手写的最小 OOXML(.docx)生成器。
// 不引入第三方文档库——.docx 本质是 zip + XML,报告需要的元素(标题层级/
// 段落/列表/表格/分页)用一小段生成器即可覆盖,换来:零新依赖、单二进制哲学
// 不破、输出结构完全可控(内置中文报告模板的字体/字号/表格边框都在 styles.xml)。
package report

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"strings"
	"time"
)

// Docx 是一份正在组装的文档;方法按顺序追加元素。
type Docx struct {
	buf        bytes.Buffer
	headerText string // 页眉文字（SetHeaderText；空 = 默认「渗透测试报告」）
}

// NewDocx 建立空文档(自动加封面后的第一个标题也由调用方控制)。
func NewDocx() *Docx { return &Docx{} }

var xmlEsc = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&apos;")

func esc(s string) string { return xmlEsc.Replace(s) }

// H1/H2/H3 标题。
func (d *Docx) H1(text string) { d.heading(text, 1) }
func (d *Docx) H2(text string) { d.heading(text, 2) }
func (d *Docx) H3(text string) { d.heading(text, 3) }

func (d *Docx) heading(text string, level int) {
	fmt.Fprintf(&d.buf, `<w:p><w:pPr><w:pStyle w:val="Heading%d"/></w:pPr><w:r><w:t xml:space="preserve">%s</w:t></w:r></w:p>`, level, esc(text))
}

// headingColored 带标题色的标题（严重度着色用：critical 红/high 橙/medium 琥珀/low 蓝）。
func (d *Docx) headingColored(text string, level int, hex string) {
	fmt.Fprintf(&d.buf, `<w:p><w:pPr><w:pStyle w:val="Heading%d"/></w:pPr><w:r><w:rPr><w:color w:val="%s"/></w:rPr><w:t xml:space="preserve">%s</w:t></w:r></w:p>`, level, hex, esc(text))
}

// H4 四级标题（漏洞明细内的小节：报告正文里的 ## 降到这里，不再压平）。
func (d *Docx) H4(text string) { d.heading(text, 4) }

// Run 富文本片段（PRich 用）。
type Run struct {
	Text string
	Bold bool
}

// PRich 富文本段落：支持 **加粗** 级别（Markdown 富渲染 A5 的底层）。\n 拆多段。
func (d *Docx) PRich(text string) {
	for _, line := range strings.Split(text, "\n") {
		d.buf.WriteString(`<w:p>`)
		d.buf.WriteString(runsXML(parseBold(line)))
		d.buf.WriteString(`</w:p>`)
	}
}

// parseBold 把一行按 **bold** 拆成 Run 序列。不成对的 ** 按字面处理。
func parseBold(line string) []Run {
	var runs []Run
	rest := line
	bold := false
	for {
		i := strings.Index(rest, "**")
		if i < 0 {
			break
		}
		if i > 0 {
			runs = append(runs, Run{Text: rest[:i], Bold: bold})
		}
		rest = rest[i+2:]
		bold = !bold
	}
	if rest != "" || len(runs) == 0 {
		runs = append(runs, Run{Text: rest, Bold: bold})
	}
	return runs
}

func runsXML(runs []Run) string {
	var b strings.Builder
	for _, r := range runs {
		rpr := ""
		if r.Bold {
			rpr = "<w:rPr><w:b/></w:rPr>"
		}
		fmt.Fprintf(&b, `<w:r>%s<w:t xml:space="preserve">%s</w:t></w:r>`, rpr, esc(r.Text))
	}
	return b.String()
}

// P 正文段落(支持 \n 拆多段)。
func (d *Docx) P(text string) {
	for _, line := range strings.Split(text, "\n") {
		fmt.Fprintf(&d.buf, `<w:p><w:r><w:t xml:space="preserve">%s</w:t></w:r></w:p>`, esc(line))
	}
}

// Code 等宽代码块(证据/PoC 用):浅灰底、Consolas、保留空白。
func (d *Docx) Code(text string) {
	var b strings.Builder
	for _, line := range strings.Split(text, "\n") {
		b.WriteString(fmt.Sprintf(`<w:p><w:pPr><w:pStyle w:val="Code"/><w:shd w:val="clear" w:fill="F2F2F2"/></w:pPr><w:r><w:t xml:space="preserve">%s</w:t></w:r></w:p>`, esc(line)))
	}
	d.buf.WriteString(b.String())
}

// Bullet 无序列表项。
func (d *Docx) Bullet(text string) {
	fmt.Fprintf(&d.buf, `<w:p><w:pPr><w:numPr><w:numId w:val="1"/></w:numPr></w:pPr><w:r><w:t xml:space="preserve">%s</w:t></w:r></w:p>`, esc(text))
}

// Table 带表头的表格。rows 与 header 列数不一致时按最短补齐。
func (d *Docx) Table(header []string, rows [][]string) {
	d.TableFills(header, rows, nil)
}

// TableFills 带行底色的表格：fills[i] 是第 i 行数据的底色 hex（空串或越界 = 无底色）。
// 严重度分布表按严重度着色用（A2）。
func (d *Docx) TableFills(header []string, rows [][]string, fills []string) {
	var b strings.Builder
	b.WriteString(`<w:tbl><w:tblPr><w:tblStyle w:val="TableGrid"/><w:tblW w:w="0" w:type="auto"/></w:tblPr><w:tblGrid>`)
	for range header {
		b.WriteString(`<w:gridCol w:w="1800"/>`)
	}
	b.WriteString(`</w:tblGrid>`)
	cell := func(text, fill string) {
		sty := ""
		if fill != "" {
			sty = fmt.Sprintf(`<w:shd w:val="clear" w:fill="%s"/>`, fill)
		}
		fmt.Fprintf(&b, `<w:tc><w:tcPr>%s</w:tcPr><w:p><w:r><w:t xml:space="preserve">%s</w:t></w:r></w:p></w:tc>`, sty, esc(text))
	}
	b.WriteString(`<w:tr>`)
	for _, h := range header {
		cell(h, "DEEAF6")
	}
	b.WriteString(`</w:tr>`)
	for ri, r := range rows {
		b.WriteString(`<w:tr>`)
		fill := ""
		if ri < len(fills) {
			fill = fills[ri]
		}
		for i := range header {
			v := ""
			if i < len(r) {
				v = r[i]
			}
			cell(v, fill)
		}
		b.WriteString(`</w:tr>`)
	}
	b.WriteString(`</w:tbl>`)
	// OOXML 要求表格后跟一个空段落
	b.WriteString(`<w:p/>`)
	d.buf.WriteString(b.String())
}

// PageBreak 分页。
func (d *Docx) PageBreak() {
	d.buf.WriteString(`<w:p><w:r><w:br w:type="page"/></w:r></w:p>`)
}

// CoverLine 封面行：居中、可指定半点字号（sz）与颜色。空文本输出空行（撑版面用）。
func (d *Docx) CoverLine(text string, sz int, bold bool) {
	pPr := `<w:pPr><w:jc w:val="center"/><w:spacing w:before="120" w:after="120"/></w:pPr>`
	if text == "" {
		fmt.Fprintf(&d.buf, `<w:p>%s</w:p>`, pPr)
		return
	}
	rpr := "<w:rPr>"
	if bold {
		rpr += "<w:b/>"
	}
	if sz > 0 {
		rpr += fmt.Sprintf(`<w:sz w:val="%d"/><w:szCs w:val="%d"/>`, sz, sz)
	}
	rpr += "</w:rPr>"
	fmt.Fprintf(&d.buf, `<w:p>%s<w:r>%s<w:t xml:space="preserve">%s</w:t></w:r></w:p>`, pPr, rpr, esc(text))
}

// TOC 目录字段：Word 打开时按 updateFields 设置自动刷新（settings.xml 已置
// updateFields，见 Encode）。\o "1-3" 收 Heading1-3——H4（漏洞明细小节）不进目录。
func (d *Docx) TOC() {
	d.buf.WriteString(`<w:p><w:fldSimple w:instr=" TOC \o &quot;1-3&quot; \h \z \u ">` +
		`<w:r><w:t>（在 Word 中按 F9 或打印预览可刷新目录）</w:t></w:r></w:fldSimple></w:p>`)
}

// Encode 打包成 .docx 字节流。
func (d *Docx) Encode(w io.Writer) error {
	// 页眉页脚引用走 r: 命名空间（headerReference/footerReference 的 r:id）。
	document := `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><w:body>` +
		d.buf.String() +
		`<w:sectPr>` +
		`<w:headerReference w:type="default" r:id="rIdHdr"/>` +
		`<w:footerReference w:type="default" r:id="rIdFtr"/>` +
		`<w:pgSz w:w="11906" w:h="16838"/><w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440"/>` +
		`</w:sectPr></w:body></w:document>`

	contentTypes := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>
<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
<Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>
<Override PartName="/word/numbering.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.numbering+xml"/>
<Override PartName="/word/settings.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.settings+xml"/>
<Override PartName="/word/header1.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.header+xml"/>
<Override PartName="/word/footer1.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.footer+xml"/>
</Types>`

	rels := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`

	// numbering.xml：Bullet 引用 numId=1——此前包里没有这个部件，Word 里项目符号
	// 静默不渲染（v0.3.93 A1，报告质感的显性 bug）。
	documentRels := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/numbering" Target="numbering.xml"/>
<Relationship Id="rIdHdr" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/header" Target="header1.xml"/>
<Relationship Id="rIdFtr" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/footer" Target="footer1.xml"/>
<Relationship Id="rIdSet" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/settings" Target="settings.xml"/>
</Relationships>`

	// settings.xml：updateFields 让 Word 打开文档时自动刷新 TOC/PAGE 等字段。
	settings := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:settings xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
<w:updateFields w:val="true"/>
</w:settings>`

	// 页眉：报告名（右侧小字灰色，底边框分隔线）；页脚：页码字段。
	hdr := d.headerText
	if hdr == "" {
		hdr = "渗透测试报告"
	}
	header := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:hdr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
<w:p><w:pPr><w:pBdr><w:bottom w:val="single" w:sz="4" w:space="1" w:color="999999"/></w:pBdr><w:jc w:val="right"/></w:pPr>
<w:r><w:rPr><w:color w:val="808080"/><w:sz w:val="16"/></w:rPr><w:t xml:space="preserve">%s</w:t></w:r></w:p>
</w:hdr>`, esc(hdr))

	footer := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:ftr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
<w:p><w:pPr><w:jc w:val="center"/></w:pPr>
<w:r><w:rPr><w:color w:val="808080"/><w:sz w:val="16"/></w:rPr><w:t xml:space="preserve">第 </w:t></w:r>
<w:r><w:fldChar w:fldCharType="begin"/></w:r>
<w:r><w:instrText xml:space="preserve"> PAGE </w:instrText></w:r>
<w:r><w:fldChar w:fldCharType="end"/></w:r>
<w:r><w:rPr><w:color w:val="808080"/><w:sz w:val="16"/></w:rPr><w:t xml:space="preserve"> 页</w:t></w:r>
</w:p>
</w:ftr>`

	zw := zip.NewWriter(w)
	files := []struct{ name, body string }{
		{"[Content_Types].xml", contentTypes},
		{"_rels/.rels", rels},
		{"word/document.xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + document},
		{"word/_rels/document.xml.rels", documentRels},
		{"word/styles.xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + docxStyles},
		{"word/numbering.xml", numbering()},
		{"word/settings.xml", settings},
		{"word/header1.xml", header},
		{"word/footer1.xml", footer},
	}
	for _, f := range files {
		fw, err := zw.Create(f.name)
		if err != nil {
			return err
		}
		if _, err := io.Copy(fw, strings.NewReader(f.body)); err != nil {
			return err
		}
	}
	return zw.Close()
}

// SetHeaderText 自定义页眉文字（客户名/报告名）。须在 Encode 前调用。
func (d *Docx) SetHeaderText(text string) { d.headerText = text }

// numbering numbering.xml 部件：numId=1 → bullet 项目符号（Bullet 方法引用它）。
func numbering() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:numbering xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
<w:abstractNum w:abstractNumId="0">
<w:multiLevelType w:val="hybridMultilevel"/>
<w:lvl w:ilvl="0"><w:start w:val="1"/><w:numFmt w:val="bullet"/><w:lvlText w:val="&#8226;"/><w:lvlJc w:val="left"/>
<w:pPr><w:ind w:left="720" w:hanging="360"/></w:pPr>
<w:rPr><w:rFonts w:ascii="Symbol" w:hAnsi="Symbol" w:hint="default"/></w:rPr></w:lvl>
<w:lvl w:ilvl="1"><w:start w:val="1"/><w:numFmt w:val="bullet"/><w:lvlText w:val="o"/><w:lvlJc w:val="left"/>
<w:pPr><w:ind w:left="1440" w:hanging="360"/></w:pPr></w:lvl>
<w:lvl w:ilvl="2"><w:start w:val="1"/><w:numFmt w:val="bullet"/><w:lvlText w:val="&#61623;"/><w:lvlJc w:val="left"/>
<w:pPr><w:ind w:left="2160" w:hanging="360"/></w:pPr></w:lvl>
</w:abstractNum>
<w:num w:numId="1"><w:abstractNumId w:val="0"/></w:num>
</w:numbering>`
}

// docxNum 部件按需打包；headerText 经由 Docx 字段透传。

// docxStyles 内置中文报告样式:标题黑体分级、正文宋体小四、代码 Consolas、
// Heading1 大纲级别 0(Word 导航窗格可见 → 天然目录结构)。
const docxStyles = `<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
<w:docDefaults><w:rPrDefault><w:rPr>
<w:rFonts w:ascii="Calibri" w:eastAsia="宋体" w:hAnsi="Calibri"/>
<w:sz w:val="24"/><w:szCs w:val="24"/>
</w:rPr></w:rPrDefault></w:docDefaults>
<w:style w:type="paragraph" w:styleId="Normal" w:default="1"><w:name w:val="Normal"/>
<w:pPr><w:spacing w:after="120"/></w:pPr></w:style>
<w:style w:type="paragraph" w:styleId="Heading1"><w:name w:val="heading 1"/><w:basedOn w:val="Normal"/>
<w:pPr><w:outlineLvl w:val="0"/><w:spacing w:before="360" w:after="180"/></w:pPr>
<w:rPr><w:rFonts w:ascii="Calibri" w:eastAsia="黑体"/><w:b/><w:sz w:val="40"/></w:rPr></w:style>
<w:style w:type="paragraph" w:styleId="Heading2"><w:name w:val="heading 2"/><w:basedOn w:val="Normal"/>
<w:pPr><w:outlineLvl w:val="1"/><w:spacing w:before="280" w:after="140"/></w:pPr>
<w:rPr><w:rFonts w:ascii="Calibri" w:eastAsia="黑体"/><w:b/><w:sz w:val="32"/></w:rPr></w:style>
<w:style w:type="paragraph" w:styleId="Heading3"><w:name w:val="heading 3"/><w:basedOn w:val="Normal"/>
<w:pPr><w:outlineLvl w:val="2"/><w:spacing w:before="200" w:after="120"/></w:pPr>
<w:rPr><w:rFonts w:ascii="Calibri" w:eastAsia="黑体"/><w:b/><w:sz w:val="28"/></w:rPr></w:style>
<w:style w:type="paragraph" w:styleId="Heading4"><w:name w:val="heading 4"/><w:basedOn w:val="Normal"/>
<w:pPr><w:outlineLvl w:val="3"/><w:spacing w:before="160" w:after="100"/></w:pPr>
<w:rPr><w:rFonts w:ascii="Calibri" w:eastAsia="黑体"/><w:b/><w:sz w:val="24"/></w:rPr></w:style>
<w:style w:type="paragraph" w:styleId="Code"><w:name w:val="Code"/><w:basedOn w:val="Normal"/>
<w:rPr><w:rFonts w:ascii="Consolas" w:eastAsia="宋体" w:hAnsi="Consolas"/><w:sz w:val="18"/></w:rPr></w:style>
<w:style w:type="table" w:styleId="TableGrid"><w:name w:val="Table Grid"/>
<w:tblPr><w:tblBorders>
<w:top w:val="single" w:sz="4" w:color="999999"/><w:left w:val="single" w:sz="4" w:color="999999"/>
<w:bottom w:val="single" w:sz="4" w:color="999999"/><w:right w:val="single" w:sz="4" w:color="999999"/>
<w:insideH w:val="single" w:sz="4" w:color="999999"/><w:insideV w:val="single" w:sz="4" w:color="999999"/>
</w:tblBorders></w:tblPr></w:style>
</w:styles>`

// SeverityHex 严重度主色（标题/表格底色用，A2）。
func SeverityHex(sev string) string {
	switch strings.ToLower(strings.TrimSpace(sev)) {
	case "critical":
		return "C00000" // 深红
	case "high":
		return "ED7D31" // 橙
	case "medium":
		return "BF9000" // 琥珀
	case "low":
		return "5B9BD5" // 蓝
	default:
		return ""
	}
}

// SeverityHexSoft 严重度柔和底色（表格行底色用，比主色浅）。
func SeverityHexSoft(sev string) string {
	switch strings.ToLower(strings.TrimSpace(sev)) {
	case "critical":
		return "F8CBCC"
	case "high":
		return "FBE5D5"
	case "medium":
		return "FFF2CC"
	case "low":
		return "DEEAF6"
	default:
		return ""
	}
}

// SeverityCN 严重度中文映射(报告用)。
func SeverityCN(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "critical":
		return "严重(critical)"
	case "high":
		return "高危(high)"
	case "medium":
		return "中危(medium)"
	case "low":
		return "低危(low)"
	default:
		return s
	}
}

// FormatTimeUTC 统一时间展示(UTC,报告里避免时区歧义)。
func FormatTimeUTC(t time.Time) string {
	return t.UTC().Format("2006-01-02 15:04 UTC")
}
