package tasks

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

	"go.uber.org/zap"
)

const (
	defaultSegmentOverlapSec = 3
	maxSegmentOverlapSec     = 8
)

var continuationPrefixes = []string{
	"然后",
	"所以",
	"因为",
	"接下来",
	"也就是说",
	"我们继续",
	"继续",
}

// UniformStats 表示一组分段长度的均匀性统计结果。
type UniformStats struct {
	Count     int
	ModeBin   int
	ModeRatio float64
	MinLen    int
	MaxLen    int
}

// CalcUniformStats 统计分段长度的分布情况，用于判断 LLM 是否输出了过于规整的切分结果。
func CalcUniformStats(segs []llmSegment) UniformStats {
	const minValidLen = 5
	const binWidth = 10

	st := UniformStats{
		Count:  0,
		MinLen: math.MaxInt,
		MaxLen: 0,
	}
	if len(segs) == 0 {
		st.MinLen = 0
		return st
	}

	freq := make(map[int]int, len(segs))
	for _, s := range segs {
		l := s.EndTimeSec - s.StartTimeSec
		if l < minValidLen {
			continue
		}
		st.Count++
		if l < st.MinLen {
			st.MinLen = l
		}
		if l > st.MaxLen {
			st.MaxLen = l
		}
		bin := (l / binWidth) * binWidth
		freq[bin]++
	}
	if st.Count == 0 {
		st.MinLen = 0
		return st
	}

	modeBin := 0
	modeCount := 0
	for bin, c := range freq {
		if c > modeCount {
			modeCount = c
			modeBin = bin
		}
	}
	st.ModeBin = modeBin
	st.ModeRatio = float64(modeCount) / float64(st.Count)
	return st
}

// IsUniformSegments 判断分段是否存在明显的等距切分倾向。
func IsUniformSegments(segs []llmSegment) (bool, UniformStats) {
	st := CalcUniformStats(segs)
	if st.Count < 3 {
		return false, st
	}
	if st.ModeRatio >= 0.6 {
		return true, st
	}
	if st.MaxLen-st.MinLen <= 15 {
		return true, st
	}
	return false, st
}

// NormalizeLLMSegments 清洗并规范化 LLM 生成的细分段结果。
// 这里会负责裁剪越界时间、限制相邻重叠、合并过短分段，并重新生成连续的 segment_index。
func NormalizeLLMSegments(llmOut string, durationSec int, minSec int, maxSec int) ([]llmSegment, error) {
	if durationSec <= 0 {
		return nil, errors.New("durationSec must be > 0")
	}
	if minSec <= 0 {
		minSec = 20
	}
	if maxSec <= 0 {
		maxSec = 180
	}
	if maxSec < minSec {
		maxSec = minSec
	}
	overlapSec := calcAllowedSegmentOverlapSec(minSec)

	rawJSON, ok := ExtractFirstJSONObject(llmOut)
	if !ok {
		return nil, errors.New("llm output is not a json object")
	}
	var parsed struct {
		Segments []llmSegment `json:"segments"`
	}
	if err := json.Unmarshal([]byte(rawJSON), &parsed); err != nil {
		return nil, err
	}
	if len(parsed.Segments) == 0 {
		return nil, errors.New("llm output has no segments")
	}
	zap.L().Debug("vectorize_hierarchical_llm_segments_raw",
		zap.Int("raw_total", len(parsed.Segments)))

	tmp := make([]llmSegment, 0, len(parsed.Segments))
	for _, s := range parsed.Segments {
		if s.StartTimeSec < 0 {
			s.StartTimeSec = 0
		}
		if s.EndTimeSec > durationSec {
			s.EndTimeSec = durationSec
		}
		if s.EndTimeSec <= s.StartTimeSec {
			continue
		}
		s.ContentSummary = strings.TrimSpace(s.ContentSummary)
		s.KnowledgeTags = NormalizeTags(s.KnowledgeTags)
		s.BoundaryReason = strings.TrimSpace(s.BoundaryReason)
		s.StartAnchorText = strings.TrimSpace(s.StartAnchorText)
		s.EndAnchorText = strings.TrimSpace(s.EndAnchorText)
		s.BoundaryConfidence = NormalizeBoundaryConfidence(s.BoundaryConfidence)
		zap.L().Debug("vectorize_hierarchical_llm_segment_raw",
			zap.Int("idx", s.SegmentIndex),
			zap.Int("start_sec", s.StartTimeSec),
			zap.Int("end_sec", s.EndTimeSec),
			zap.String("boundary_reason", s.BoundaryReason),
			zap.String("start_anchor_text", s.StartAnchorText),
			zap.String("end_anchor_text", s.EndAnchorText),
			zap.String("boundary_confidence", s.BoundaryConfidence),
			zap.String("content_summary", s.ContentSummary))
		tmp = append(tmp, s)
	}
	if len(tmp) == 0 {
		return nil, errors.New("no valid segments after clamp")
	}

	sort.Slice(tmp, func(i, j int) bool {
		if tmp[i].StartTimeSec != tmp[j].StartTimeSec {
			return tmp[i].StartTimeSec < tmp[j].StartTimeSec
		}
		return tmp[i].EndTimeSec < tmp[j].EndTimeSec
	})

	normalized := make([]llmSegment, 0, len(tmp))
	for _, s := range tmp {
		if len(normalized) == 0 {
			normalized = append(normalized, s)
			continue
		}
		prev := &normalized[len(normalized)-1]
		minStart := prev.StartTimeSec + 1
		if minStart < 0 {
			minStart = 0
		}
		if s.StartTimeSec < minStart {
			s.StartTimeSec = minStart
		}
		allowedOverlapStart := prev.EndTimeSec - overlapSec
		if allowedOverlapStart < minStart {
			allowedOverlapStart = minStart
		}
		// 这里不主动制造重叠，只在 LLM 给出的重叠过大时把起点压回允许范围。
		if s.StartTimeSec < allowedOverlapStart {
			s.StartTimeSec = allowedOverlapStart
		}
		if s.EndTimeSec <= s.StartTimeSec {
			continue
		}
		decision := EvaluateSegmentBoundary(LLMSegment(*prev), LLMSegment(s))
		zap.L().Debug("segment_boundary_decision",
			zap.String("action", decision.Action),
			zap.Float64("score", decision.Score),
			zap.String("confidence", decision.Confidence),
			zap.Strings("reasons", decision.Reasons),
			zap.String("prev_summary", prev.ContentSummary),
			zap.String("curr_summary", s.ContentSummary),
			zap.String("prev_boundary_confidence", prev.BoundaryConfidence),
			zap.String("curr_boundary_confidence", s.BoundaryConfidence))
		if decision.Action == "merge" {
			if s.EndTimeSec > prev.EndTimeSec {
				prev.EndTimeSec = s.EndTimeSec
			}
			if s.ContentSummary != "" {
				if prev.ContentSummary == "" {
					prev.ContentSummary = s.ContentSummary
				} else {
					prev.ContentSummary = prev.ContentSummary + "\n" + s.ContentSummary
				}
			}
			prev.KnowledgeTags = MergeTags(prev.KnowledgeTags, s.KnowledgeTags)
			if prev.EndAnchorText == "" {
				prev.EndAnchorText = s.EndAnchorText
			}
			if prev.BoundaryReason == "" {
				prev.BoundaryReason = s.BoundaryReason
			}
			if prev.BoundaryConfidence == "" || prev.BoundaryConfidence == "low" {
				prev.BoundaryConfidence = s.BoundaryConfidence
			}
			zap.L().Debug("vectorize_hierarchical_llm_segments_merge_continuation",
				zap.Int("prev_start_sec", prev.StartTimeSec),
				zap.Int("prev_end_sec", prev.EndTimeSec),
				zap.Int("curr_start_sec", s.StartTimeSec),
				zap.Int("curr_end_sec", s.EndTimeSec),
				zap.String("curr_boundary_confidence", s.BoundaryConfidence),
				zap.String("curr_start_anchor_text", s.StartAnchorText),
				zap.String("curr_content_summary", s.ContentSummary))
			continue
		}
		if decision.Action == "recut" {
			newBoundary := recutBoundary(prev.EndTimeSec, s.StartTimeSec)
			if newBoundary > prev.StartTimeSec && newBoundary < s.EndTimeSec {
				prev.EndTimeSec = newBoundary
				s.StartTimeSec = newBoundary
				zap.L().Debug("vectorize_hierarchical_llm_segments_recut_boundary",
					zap.Int("prev_start_sec", prev.StartTimeSec),
					zap.Int("prev_end_sec_old", prev.EndTimeSec),
					zap.Int("curr_start_sec_old", s.StartTimeSec),
					zap.Int("new_boundary_sec", newBoundary),
					zap.Float64("decision_score", decision.Score),
					zap.Strings("decision_reasons", decision.Reasons))
			}
		}
		normalized = append(normalized, s)
	}

	merged := make([]llmSegment, 0, len(normalized))
	for _, s := range normalized {
		d := s.EndTimeSec - s.StartTimeSec
		if d >= minSec || len(merged) == 0 {
			merged = append(merged, s)
			continue
		}
		prev := &merged[len(merged)-1]
		if s.EndTimeSec > prev.EndTimeSec {
			prev.EndTimeSec = s.EndTimeSec
		}
		if s.ContentSummary != "" {
			if prev.ContentSummary == "" {
				prev.ContentSummary = s.ContentSummary
			} else {
				prev.ContentSummary = prev.ContentSummary + "\n" + s.ContentSummary
			}
		}
		prev.KnowledgeTags = MergeTags(prev.KnowledgeTags, s.KnowledgeTags)
	}

	if len(merged) >= 2 {
		last := merged[len(merged)-1]
		if last.EndTimeSec-last.StartTimeSec < minSec {
			prev := &merged[len(merged)-2]
			if last.EndTimeSec > prev.EndTimeSec {
				prev.EndTimeSec = last.EndTimeSec
			}
			if last.ContentSummary != "" {
				if prev.ContentSummary == "" {
					prev.ContentSummary = last.ContentSummary
				} else {
					prev.ContentSummary = prev.ContentSummary + "\n" + last.ContentSummary
				}
			}
			prev.KnowledgeTags = MergeTags(prev.KnowledgeTags, last.KnowledgeTags)
			merged = merged[:len(merged)-1]
		}
	}

	for i := range merged {
		merged[i].SegmentIndex = i
		zap.L().Debug("vectorize_hierarchical_llm_segment_normalized",
			zap.Int("idx", merged[i].SegmentIndex),
			zap.Int("start_sec", merged[i].StartTimeSec),
			zap.Int("end_sec", merged[i].EndTimeSec),
			zap.String("boundary_reason", merged[i].BoundaryReason),
			zap.String("start_anchor_text", merged[i].StartAnchorText),
			zap.String("end_anchor_text", merged[i].EndAnchorText),
			zap.String("boundary_confidence", merged[i].BoundaryConfidence),
			zap.String("content_summary", merged[i].ContentSummary))
	}
	lowConfidence := 0
	for _, seg := range merged {
		if seg.BoundaryConfidence == "low" {
			lowConfidence++
		}
	}
	zap.L().Debug("vectorize_hierarchical_llm_low_confidence_summary",
		zap.Int("total", len(merged)),
		zap.Int("low_confidence_count", lowConfidence))
	return merged, nil
}

func recutBoundary(prevEndSec int, currStartSec int) int {
	if currStartSec <= prevEndSec {
		return prevEndSec + (currStartSec-prevEndSec)/2
	}
	return prevEndSec + (currStartSec-prevEndSec)/2
}

// calcAllowedSegmentOverlapSec 根据最小分段时长推导允许的相邻片段最大重叠秒数。
func calcAllowedSegmentOverlapSec(minSec int) int {
	if minSec <= 0 {
		return defaultSegmentOverlapSec
	}
	overlap := minSec / 6
	if overlap < defaultSegmentOverlapSec {
		overlap = defaultSegmentOverlapSec
	}
	if overlap > maxSegmentOverlapSec {
		overlap = maxSegmentOverlapSec
	}
	return overlap
}

// BuildHierarchicalSegmentationRetryPrompt 构造二次纠偏 prompt，用于纠正 LLM 过于等距的分段结果。
func BuildHierarchicalSegmentationRetryPrompt(durationSec int, coarseSegmentSec int, refineMinSec int, refineMaxSec int, coarseItems []coarseItem) (string, error) {
	if durationSec <= 0 {
		return "", errors.New("durationSec must be > 0")
	}
	if coarseSegmentSec <= 0 {
		coarseSegmentSec = 60
	}
	if refineMinSec <= 0 {
		refineMinSec = 20
	}
	if refineMaxSec <= 0 {
		refineMaxSec = 180
	}
	overlapSec := calcAllowedSegmentOverlapSec(refineMinSec)

	var b strings.Builder
	b.WriteString("你上一次的分段结果存在内容边界不合理的问题，可能表现为过于等距/规整，也可能表现为把同一个知识点切碎，或把两个不同知识点混在一起。请重新分段，并严格只输出 JSON（不要输出任何解释性文字、不要用 Markdown）。\n")
	b.WriteString("要求：\n")
	b.WriteString(fmt.Sprintf("- 视频总时长（秒）：%d\n", durationSec))
	b.WriteString(fmt.Sprintf("- 粗分段步长（秒）：%d（不中断，0->duration）\n", coarseSegmentSec))
	b.WriteString(fmt.Sprintf("- 输出分段最小长度（秒）：%d\n", refineMinSec))
	b.WriteString(fmt.Sprintf("- 输出分段最大长度（秒）：%d\n", refineMaxSec))
	b.WriteString("- 重新分段前，请先自检上一版结果是否存在以下问题：同一个知识点被拆成多个过碎小段；定义和关键解释被切开；完整解题步骤被切成前后两半；上一段结论和下一段新主题混在一起；分段表面不等距但本质仍按时间切分\n")
	b.WriteString("- 分段边界必须依据内容主题/转折点（例如讲解对象变化、步骤切换、定义/举例/总结切换）\n")
	b.WriteString("- 一个分段应尽量对应一个完整的知识单元，例如：一个定义、一个定理、一组连续推导、一个完整解题步骤、一个完整例题阶段、一个总结结论\n")
	b.WriteString("- 分段结束位置优先落在 ASR 文本里一整句话说完的位置，不要把一句话截成前后两半\n")
	b.WriteString("- 如果主题切换点出现在一句话中间，当前分段应延续到这句话自然结束，再进入下一个分段\n")
	b.WriteString("- 如果发现某个分段只是上一段内容的延续，应优先合并或后移边界\n")
	b.WriteString("- 如果发现某个分段同时包含上一知识点收尾和下一知识点起始，应优先把边界移动到两者之间更自然的位置\n")
	b.WriteString("- 如果某个分段虽然时长合规，但内容上仍然是半个定义、半个步骤或半个例题阶段，也视为不合格分段\n")
	b.WriteString("- 如果下一段需要承接转折句，可让下一段从上一句末尾前少量时间开始，但不要为了重叠而重叠\n")
	b.WriteString("- 严禁等距切分，严禁固定每 N 秒一段（例如每 120 秒一段）\n")
	b.WriteString("- 不要只通过微调时间让长度看起来不一致；只有当知识点边界或步骤边界更清晰时，才算更好的重分段\n")
	b.WriteString("- 输出分段必须按 start_time 升序排列\n")
	b.WriteString("- start_time/end_time 必须在 [0, 视频总时长] 范围内，且 end_time > start_time\n")
	b.WriteString(fmt.Sprintf("- 相邻分段允许少量重叠（建议 1-%d 秒），用于保留转折处上下文，避免一句话被生硬切断\n", overlapSec))
	b.WriteString(fmt.Sprintf("- 不要让重叠过大；相邻分段重叠尽量不要超过 %d 秒\n", overlapSec))
	b.WriteString("- 如果主题切换点出现在一句话中间，先给出语义边界意图，最终句子收尾由后处理完成\n")
	b.WriteString("- 分段优先级从高到低：先保证知识单元完整，再保证句意完整，最后再考虑时长均衡\n")
	b.WriteString("- content_summary 字段存的是该视频段标题，不是内容简介\n")
	b.WriteString("- content_summary 必须写成短标题，便于展示、检索和列表阅读\n")
	b.WriteString("- content_summary 必须能被该分段内的实际讲解内容完整支撑，不能提前概括下一段内容\n")
	b.WriteString("- 如果某个知识点在本段没有讲完，不要把它总结成已经完整讲完\n")
	b.WriteString("- 优先保证知识单元完整，不要为了时长均匀硬切到一句话说一半的位置\n")
	b.WriteString("- 不要写成长段内容简介，不要复述整段讲解过程，不要输出多句解释\n")
	b.WriteString("- 每个分段必须给出 boundary_reason，说明为什么从这里开始/上一段为什么在这里结束，并引用 ASR 关键词或短句\n")
	b.WriteString("- 每个分段必须给出 start_anchor_text 和 end_anchor_text，要求短、具体、可在 ASR 中定位\n")
	b.WriteString("- 每个分段必须给出 boundary_confidence，取值为 high、medium、low\n")
	b.WriteString("- 只输出下面 schema 的 JSON\n\n")

	b.WriteString("输出 JSON schema：\n")
	b.WriteString("{\n")
	b.WriteString("  \"segments\": [\n")
	b.WriteString("    {\n")
	b.WriteString("      \"segment_index\": 0,\n")
	b.WriteString("      \"start_time\": 0,\n")
	b.WriteString("      \"end_time\": 75,\n")
	b.WriteString("      \"content_summary\": \"...\",\n")
	b.WriteString("      \"knowledge_tags\": [\"tag1\", \"tag2\"],\n")
	b.WriteString("      \"boundary_reason\": \"上一句在这里完整收束，后面开始进入新的知识点\",\n")
	b.WriteString("      \"start_anchor_text\": \"下面先看定义\",\n")
	b.WriteString("      \"end_anchor_text\": \"这就是它的定义\",\n")
	b.WriteString("      \"boundary_confidence\": \"high\"\n")
	b.WriteString("    }\n")
	b.WriteString("  ]\n")
	b.WriteString("}\n\n")

	b.WriteString("粗分段 ASR 输入（JSON Lines，每行一个粗分段）：\n")
	for _, it := range coarseItems {
		text := NormalizeText(it.Text)
		b.WriteString(fmt.Sprintf(`{"index":%d,"start":%d,"end":%d,"text":%q}\n`, it.Index, it.StartSec, it.EndSec, text))
	}
	return b.String(), nil
}

// BuildHierarchicalSegmentationPrompt 构造 hierarchical 模式下的细分段 prompt。
// prompt 会要求模型优先按完整句、完整步骤或完整定义收尾，而不是追求均匀时长。
func BuildHierarchicalSegmentationPrompt(durationSec int, coarseSegmentSec int, refineMinSec int, refineMaxSec int, coarseItems []coarseItem) (string, error) {
	if durationSec <= 0 {
		return "", errors.New("durationSec must be > 0")
	}
	if coarseSegmentSec <= 0 {
		coarseSegmentSec = 60
	}
	if refineMinSec <= 0 {
		refineMinSec = 20
	}
	if refineMaxSec <= 0 {
		refineMaxSec = 180
	}
	overlapSec := calcAllowedSegmentOverlapSec(refineMinSec)

	var b strings.Builder
	b.WriteString("请根据视频的粗粒度 ASR 文本，生成结构化的\"内容分段\"，并严格只输出 JSON（不要输出任何解释性文字、不要用 Markdown）。\n")
	b.WriteString("要求：\n")
	b.WriteString(fmt.Sprintf("- 视频总时长（秒）：%d\n", durationSec))
	b.WriteString(fmt.Sprintf("- 粗分段步长（秒）：%d（不中断，0->duration）\n", coarseSegmentSec))
	b.WriteString(fmt.Sprintf("- 输出分段最小长度（秒）：%d\n", refineMinSec))
	b.WriteString(fmt.Sprintf("- 输出分段最大长度（秒）：%d\n", refineMaxSec))
	b.WriteString("- 分段边界以内容主题/转折为主，不要为了\"看起来规整\"而等距切分（例如固定每 120 秒一段）\n")
	b.WriteString("- 一个分段应尽量对应一个完整的知识单元，例如：一个定义、一个定理、一组连续推导、一个完整解题步骤、一个完整例题阶段、一个总结结论\n")
	b.WriteString("- 如果同一段内容仍在围绕同一个知识点展开解释，不要仅因为时长接近上限就切开\n")
	b.WriteString("- 如果一个知识点已经讲完，并且开始进入新的定义、步骤、例子、结论或分析目标，应优先在这里切分\n")
	b.WriteString("- 分段结束位置优先落在 ASR 文本里一整句话说完的位置，不要把一句话截成前后两半\n")
	b.WriteString("- 如果主题切换点出现在一句话中间，当前分段应延续到这句话自然结束，再进入下一个分段\n")
	b.WriteString("- 不要把“定义”和它紧随其后的关键解释强行拆开\n")
	b.WriteString("- 不要把“解题步骤进行中”的内容切成前后两段\n")
	b.WriteString("- 不要把“上一段的结论/总结”和“下一段的新知识点引入”混在同一个分段里\n")
	b.WriteString("- 如果下一段需要承接转折句，可让下一段从上一句末尾前少量时间开始，但不要为了重叠而重叠\n")
	b.WriteString("- 输出分段必须按 start_time 升序排列\n")
	b.WriteString("- start_time/end_time 必须在 [0, 视频总时长] 范围内，且 end_time > start_time\n")
	b.WriteString(fmt.Sprintf("- 相邻分段允许少量重叠（建议 1-%d 秒），用于保留转折处上下文，避免一句话被生硬切断\n", overlapSec))
	b.WriteString(fmt.Sprintf("- 不要让重叠过大；相邻分段重叠尽量不要超过 %d 秒\n", overlapSec))
	b.WriteString("- 每个分段都应以完整句、完整步骤或完整定义收尾，优先保证句意完整，再考虑时间均衡\n")
	b.WriteString("- 分段优先级从高到低：先保证知识单元完整，再保证句意完整，最后再考虑时长均衡\n")
	b.WriteString("- content_summary 字段存的是该视频段标题，不是内容简介\n")
	b.WriteString("- content_summary 必须写成短标题，便于展示、检索和列表阅读\n")
	b.WriteString("- content_summary 必须能被该分段内的实际讲解内容完整支撑，不能提前概括下一段内容\n")
	b.WriteString("- 如果某个知识点在本段没有讲完，不要把它总结成已经完整讲完\n")
	b.WriteString("- 优先保证知识单元完整，不要为了时长均匀硬切到一句话说一半的位置\n")
	b.WriteString("- 不要写成长段内容简介，不要复述整段讲解过程，不要输出多句解释\n")
	b.WriteString("- 如果知识单元完整与时长均衡冲突，优先保证知识单元完整\n")
	b.WriteString("- 如果主题切换点出现在一句话中间，先给出语义边界意图，最终句子收尾由后处理完成\n")
	b.WriteString("- 错误切分示例：一个定义刚说出解释还没讲完就切开；一个解题步骤进行到一半就切到下一段；上一段总结和下一段新主题开头混在一起\n")
	b.WriteString("- 正确切分示例：定义及其必要解释保留在同一段；完整步骤结束后再切到下一步骤；总结收束后再进入新知识点\n")
	b.WriteString("- 每个分段必须给出 boundary_reason，说明为什么从这里开始/上一段为什么在这里结束，并引用 ASR 关键词或短句\n")
	b.WriteString("- 每个分段必须给出 start_anchor_text 和 end_anchor_text，要求短、具体、可在 ASR 中定位\n")
	b.WriteString("- 每个分段必须给出 boundary_confidence，取值为 high、medium、low\n")
	b.WriteString("- 只输出下面 schema 的 JSON\n\n")

	b.WriteString("输出 JSON schema：\n")
	b.WriteString("{\n")
	b.WriteString("  \"segments\": [\n")
	b.WriteString("    {\n")
	b.WriteString("      \"segment_index\": 0,\n")
	b.WriteString("      \"start_time\": 0,\n")
	b.WriteString("      \"end_time\": 75,\n")
	b.WriteString("      \"content_summary\": \"...\",\n")
	b.WriteString("      \"knowledge_tags\": [\"tag1\", \"tag2\"],\n")
	b.WriteString("      \"boundary_reason\": \"上一句在这里完整收束，后面开始进入新的知识点\",\n")
	b.WriteString("      \"start_anchor_text\": \"下面先看定义\",\n")
	b.WriteString("      \"end_anchor_text\": \"这就是它的定义\",\n")
	b.WriteString("      \"boundary_confidence\": \"high\"\n")
	b.WriteString("    }\n")
	b.WriteString("  ]\n")
	b.WriteString("}\n\n")

	b.WriteString("粗分段 ASR 输入（JSON Lines，每行一个粗分段）：\n")
	for _, it := range coarseItems {
		text := NormalizeText(it.Text)
		b.WriteString(fmt.Sprintf(`{"index":%d,"start":%d,"end":%d,"text":%q}\n`, it.Index, it.StartSec, it.EndSec, text))
	}
	return b.String(), nil
}

func BuildSummaryRewritePrompt(text string) string {
	text = NormalizeText(text)
	var b strings.Builder
	b.WriteString("请只根据提供的正文生成标题。\n")
	b.WriteString("要求：\n")
	b.WriteString("- 只根据提供的正文生成标题\n")
	b.WriteString("- 不要引入正文里没有出现的概念\n")
	b.WriteString("- 输出短标题\n")
	b.WriteString("- 不要输出解释、不要输出多句\n")
	b.WriteString("正文：\n")
	b.WriteString(text)
	return b.String()
}
