package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"hasucalc/cell"
	"hasucalc/coord"
	"hasucalc/formula"
	"hasucalc/sheet"
	"hasucalc/tui"
	"hasucalc/version"
)

type jsonrpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonrpcResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      any           `json:"id,omitempty"`
	Result  any           `json:"result,omitempty"`
	Error   *jsonrpcError `json:"error,omitempty"`
}

type jsonrpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type mcpTextContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type mcpToolCallResult struct {
	Content []mcpTextContent `json:"content"`
	IsError bool             `json:"isError,omitempty"`
}

type mcpToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// RunMCP launches the Model Context Protocol stdio server loop.
func RunMCP(args []string) int {
	if err := ServeMCP(os.Stdin, os.Stdout); err != nil && err != io.EOF {
		fmt.Fprintf(os.Stderr, "MCP server error: %v\n", err)
		return 1
	}
	return 0
}

// ServeMCP processes JSON-RPC messages from r and writes responses to w.
func ServeMCP(r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, 16*1024*1024) // up to 16MB message buffer

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}

		resp := processMessage(line)
		if resp != nil {
			respBytes, err := json.Marshal(resp)
			if err != nil {
				continue
			}
			respBytes = append(respBytes, '\n')
			if _, err := w.Write(respBytes); err != nil {
				return err
			}
		}
	}
	return scanner.Err()
}

func processMessage(msgBytes []byte) *jsonrpcResponse {
	var req jsonrpcRequest
	if err := json.Unmarshal(msgBytes, &req); err != nil {
		return &jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      nil,
			Error: &jsonrpcError{
				Code:    -32700,
				Message: fmt.Sprintf("Parse error: %v", err),
			},
		}
	}

	// Notifications (no ID) do not receive a response
	isNotification := (req.ID == nil)

	switch req.Method {
	case "initialize":
		result := map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]any{
				"tools": map[string]any{},
			},
			"serverInfo": map[string]any{
				"name":    "hasucalc",
				"version": version.Version,
			},
		}
		return &jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  result,
		}

	case "notifications/initialized":
		return nil

	case "ping":
		if isNotification {
			return nil
		}
		return &jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  map[string]any{},
		}

	case "tools/list":
		if isNotification {
			return nil
		}
		return &jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"tools": getMCPToolDefinitions(),
			},
		}

	case "tools/call":
		if isNotification {
			return nil
		}
		var callParams struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &callParams); err != nil {
			return &jsonrpcResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &jsonrpcError{
					Code:    -32602,
					Message: fmt.Sprintf("Invalid params: %v", err),
				},
			}
		}

		callRes := dispatchMCPTool(callParams.Name, callParams.Arguments)
		return &jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  callRes,
		}

	default:
		if isNotification {
			return nil
		}
		return &jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &jsonrpcError{
				Code:    -32601,
				Message: fmt.Sprintf("Method not found: %s", req.Method),
			},
		}
	}
}

func dispatchMCPTool(name string, argsRaw json.RawMessage) mcpToolCallResult {
	switch name {
	case "read_sheet":
		return mcpToolReadSheet(argsRaw)
	case "get_info":
		return mcpToolGetInfo(argsRaw)
	case "evaluate_formula":
		return mcpToolEvaluateFormula(argsRaw)
	case "edit_cell":
		return mcpToolEditCell(argsRaw)
	case "batch_edit":
		return mcpToolBatchEdit(argsRaw)
	case "render_chart":
		return mcpToolRenderChart(argsRaw)
	case "convert_file":
		return mcpToolConvertFile(argsRaw)
	default:
		return mcpToolCallResult{
			Content: []mcpTextContent{{Type: "text", Text: fmt.Sprintf("Error: unknown tool '%s'", name)}},
			IsError: true,
		}
	}
}

// ---------------- Tool Implementations ----------------

func mcpToolReadSheet(argsRaw json.RawMessage) mcpToolCallResult {
	var args struct {
		File   string `json:"file"`
		Sheet  string `json:"sheet"`
		Range  string `json:"range"`
		Format string `json:"format"`
		Recalc *bool  `json:"recalc"`
	}
	if err := json.Unmarshal(argsRaw, &args); err != nil {
		return mcpErrorResult(fmt.Sprintf("Invalid arguments: %v", err))
	}
	if args.File == "" {
		return mcpErrorResult("Missing required argument: 'file'")
	}

	wb, err := LoadWorkbookAuto(args.File)
	if err != nil {
		return mcpErrorResult(fmt.Sprintf("Failed to load workbook '%s': %v", args.File, err))
	}

	recalc := true
	if args.Recalc != nil {
		recalc = *args.Recalc
	}
	if recalc {
		wb.RecalculateAll()
	}

	sh, err := GetTargetSheet(wb, args.Sheet)
	if err != nil {
		return mcpErrorResult(err.Error())
	}

	format := strings.ToLower(strings.TrimSpace(args.Format))
	if format == "" {
		format = "json"
	}

	shMinC, shMinR, shMaxC, shMaxR := sh.BoundingBox()
	cellCount := 0
	for pt := range sh.GetPopulatedCoords() {
		c := sh.GetCell(pt.Col, pt.Row)
		if c != nil && c.Type != cell.TypeEmpty {
			cellCount++
		}
	}

	var usedRange string
	if cellCount > 0 {
		if shMinC == shMaxC && shMinR == shMaxR {
			usedRange = coord.CellRef{Col: shMinC, Row: shMinR}.String()
		} else {
			usedRange = fmt.Sprintf("%s:%s", coord.CellRef{Col: shMinC, Row: shMinR}.String(), coord.CellRef{Col: shMaxC, Row: shMaxR}.String())
		}
	} else {
		usedRange = "A1"
		shMinC, shMinR, shMaxC, shMaxR = 0, 0, 0, 0
	}

	var minC, minR, maxC, maxR int
	var queryRange string

	if args.Range != "" {
		rng, err := coord.ParseRangeRef(args.Range)
		if err != nil {
			return mcpErrorResult(fmt.Sprintf("Invalid range '%s': %v", args.Range, err))
		}
		minC, maxC = rng.MinCol(), rng.MaxCol()
		minR, maxR = rng.MinRow(), rng.MaxRow()
		if rng.IsWholeColumn() && cellCount > 0 {
			maxR = shMaxR
		}
		if rng.IsWholeRow() && cellCount > 0 {
			maxC = shMaxC
		}
		if minC == maxC && minR == maxR {
			queryRange = coord.CellRef{Col: minC, Row: minR}.String()
		} else {
			queryRange = fmt.Sprintf("%s:%s", coord.CellRef{Col: minC, Row: minR}.String(), coord.CellRef{Col: maxC, Row: maxR}.String())
		}
	} else {
		minC, minR, maxC, maxR = shMinC, shMinR, shMaxC, shMaxR
		queryRange = usedRange
	}

	switch format {
	case "markdown", "md":
		table := sh.RenderMarkdownTableRange(minC, minR, maxC, maxR)
		return mcpSuccessResult(table)
	case "csv":
		var buf bytes.Buffer
		_ = SaveWorkbookToWriter(wb, sh.Name(), &buf, "csv", false)
		return mcpSuccessResult(buf.String())
	case "values":
		var lines []string
		for r := minR; r <= maxR; r++ {
			var rowVals []string
			for c := minC; c <= maxC; c++ {
				cellVal := sh.GetCell(c, r)
				if cellVal != nil && cellVal.Value != nil && cellVal.Type != cell.TypeEmpty {
					rowVals = append(rowVals, cellVal.FormattedValue(sh.GlobalFormat()))
				} else {
					rowVals = append(rowVals, "")
				}
			}
			lines = append(lines, strings.Join(rowVals, "\t"))
		}
		return mcpSuccessResult(strings.Join(lines, "\n"))
	default: // json
		var cells []CellItem
		for r := minR; r <= maxR; r++ {
			for c := minC; c <= maxC; c++ {
				cellVal := sh.GetCell(c, r)
				if cellVal == nil || cellVal.Type == cell.TypeEmpty {
					continue
				}
				if cellVal.RawInput == "" && cellVal.Value == nil {
					continue
				}

				ref := coord.CellRef{Col: c, Row: r}.String()
				typeStr := string(cellVal.Type)
				val := cellVal.Value
				if errVal, ok := cellVal.Value.(cell.LotusError); ok {
					typeStr = "ERROR"
					val = errVal.Code
				}

				var fmtStr string
				if cellVal.FormatSpec != nil && cellVal.FormatSpec.Type != cell.FmtGeneral {
					fmtStr = cellVal.FormatSpec.String()
				}

				cells = append(cells, CellItem{
					Ref: ref,
					R:   r,
					C:   c,
					Typ: typeStr,
					Raw: cellVal.RawInput,
					Val: val,
					Fmt: fmtStr,
				})
			}
		}

		resp := Response{
			OK:      true,
			Command: "get",
			Meta: MetaInfo{
				File:       filepath.Base(args.File),
				Sheet:      sh.Name(),
				Sheets:     wb.SheetNames(),
				UsedRange:  usedRange,
				QueryRange: queryRange,
				RecalcMode: sh.RecalcMode(),
			},
			Cells: cells,
		}
		jsonBytes, _ := json.MarshalIndent(resp, "", "  ")
		return mcpSuccessResult(string(jsonBytes))
	}
}

func mcpToolGetInfo(argsRaw json.RawMessage) mcpToolCallResult {
	var args struct {
		File  string `json:"file"`
		Sheet string `json:"sheet"`
	}
	if err := json.Unmarshal(argsRaw, &args); err != nil {
		return mcpErrorResult(fmt.Sprintf("Invalid arguments: %v", err))
	}
	if args.File == "" {
		return mcpErrorResult("Missing required argument: 'file'")
	}

	wb, err := LoadWorkbookAuto(args.File)
	if err != nil {
		return mcpErrorResult(fmt.Sprintf("Failed to load '%s': %v", args.File, err))
	}

	if args.Sheet != "" {
		if _, err := GetTargetSheet(wb, args.Sheet); err != nil {
			return mcpErrorResult(err.Error())
		}
	}

	activeSheetName := wb.GetActiveSheet().Name()
	if args.Sheet != "" {
		activeSheetName = args.Sheet
	}

	var sheetInfos []SheetInfo
	for idx, s := range wb.Sheets {
		if args.Sheet != "" && !strings.EqualFold(s.Name(), args.Sheet) {
			continue
		}
		sheetInfos = append(sheetInfos, extractSheetInfo(s, idx))
	}

	resp := Response{
		OK:      true,
		Command: "info",
		Meta: MetaInfo{
			File:        filepath.Base(args.File),
			Format:      DetectFormat(args.File),
			ActiveSheet: activeSheetName,
			SheetCount:  len(wb.Sheets),
			Sheets:      sheetInfos,
			RecalcMode:  wb.GetActiveSheet().RecalcMode(),
		},
	}
	jsonBytes, _ := json.MarshalIndent(resp, "", "  ")
	return mcpSuccessResult(string(jsonBytes))
}

func mcpToolEvaluateFormula(argsRaw json.RawMessage) mcpToolCallResult {
	var args struct {
		Formula string `json:"formula"`
		File    string `json:"file"`
		Sheet   string `json:"sheet"`
	}
	if err := json.Unmarshal(argsRaw, &args); err != nil {
		return mcpErrorResult(fmt.Sprintf("Invalid arguments: %v", err))
	}
	if args.Formula == "" {
		return mcpErrorResult("Missing required argument: 'formula'")
	}

	var wb *sheet.Workbook
	var sh *sheet.Sheet
	var err error

	if args.File != "" {
		wb, err = LoadWorkbookAuto(args.File)
		if err != nil {
			return mcpErrorResult(fmt.Sprintf("Failed to load '%s': %v", args.File, err))
		}
		sh, err = GetTargetSheet(wb, args.Sheet)
		if err != nil {
			return mcpErrorResult(err.Error())
		}
	} else {
		sh = sheet.NewSheet()
		wb = sheet.NewWorkbook("Eval")
		wb.Sheets[0] = sh
		sh.SetWorkbook(wb)
	}

	ast, err := formula.ParseFormula(args.Formula)
	if err != nil {
		return mcpErrorResult(fmt.Sprintf("Formula syntax error: %v", err))
	}

	evaluator := formula.NewEvaluator(sh)
	val := evaluator.Evaluate(ast)
	val = unwrapGrid(val)

	var typeStr string
	var valPayload any
	var errPayload any

	if errVal, ok := val.(cell.LotusError); ok {
		typeStr = "ERROR"
		valPayload = errVal.Code
		errPayload = errVal.Code
	} else {
		switch v := val.(type) {
		case float64:
			typeStr = "NUMBER"
			valPayload = v
		case int:
			typeStr = "NUMBER"
			valPayload = v
		case bool:
			typeStr = "BOOLEAN"
			valPayload = v
		case string:
			typeStr = "LABEL"
			valPayload = v
		case nil:
			typeStr = "EMPTY"
			valPayload = nil
		default:
			typeStr = "LABEL"
			valPayload = fmt.Sprintf("%v", v)
		}
	}

	resp := Response{
		OK:      true,
		Command: "eval",
		Formula: args.Formula,
		Result: EvalResult{
			Val:   valPayload,
			Type:  typeStr,
			Error: errPayload,
		},
	}
	if args.File != "" {
		minC, minR, maxC, maxR := sh.BoundingBox()
		usedRange := "A1"
		if len(sh.GetPopulatedCoords()) > 0 {
			usedRange = fmt.Sprintf("%s:%s", coord.CellRef{Col: minC, Row: minR}.String(), coord.CellRef{Col: maxC, Row: maxR}.String())
		}
		resp.Meta = MetaInfo{
			File:      filepath.Base(args.File),
			Sheet:     sh.Name(),
			UsedRange: usedRange,
		}
	}

	jsonBytes, _ := json.MarshalIndent(resp, "", "  ")
	return mcpSuccessResult(string(jsonBytes))
}

func mcpToolEditCell(argsRaw json.RawMessage) mcpToolCallResult {
	var args struct {
		File   string `json:"file"`
		Target string `json:"target"`
		Value  string `json:"value"`
		Sheet  string `json:"sheet"`
		Format string `json:"format"`
		Recalc *bool  `json:"recalc"`
		DryRun bool   `json:"dry_run"`
	}
	if err := json.Unmarshal(argsRaw, &args); err != nil {
		return mcpErrorResult(fmt.Sprintf("Invalid arguments: %v", err))
	}
	if args.File == "" || args.Target == "" {
		return mcpErrorResult("Missing required arguments: 'file' and 'target'")
	}

	wb, err := LoadWorkbookAuto(args.File)
	if err != nil {
		return mcpErrorResult(fmt.Sprintf("Failed to load '%s': %v", args.File, err))
	}

	sh, err := GetTargetSheet(wb, args.Sheet)
	if err != nil {
		return mcpErrorResult(err.Error())
	}

	rng, err := coord.ParseRangeRef(args.Target)
	if err != nil {
		return mcpErrorResult(fmt.Sprintf("Invalid coordinate/range '%s': %v", args.Target, err))
	}

	var fmtSpec *cell.CellFormat
	if args.Format != "" {
		parsed := cell.ParseCellFormat(args.Format)
		fmtSpec = &parsed
	}

	for r := rng.MinRow(); r <= rng.MaxRow(); r++ {
		for c := rng.MinCol(); c <= rng.MaxCol(); c++ {
			sh.SetCellInput(c, r, args.Value, fmtSpec)
		}
	}

	recalc := true
	if args.Recalc != nil {
		recalc = *args.Recalc
	}
	if recalc {
		wb.RecalculateAll()
	}

	var affected []CellItem
	for r := rng.MinRow(); r <= rng.MaxRow(); r++ {
		for c := rng.MinCol(); c <= rng.MaxCol(); c++ {
			cellVal := sh.GetCell(c, r)
			if cellVal == nil {
				continue
			}
			ref := coord.CellRef{Col: c, Row: r}.String()
			typeStr := string(cellVal.Type)
			val := cellVal.Value
			if errVal, ok := cellVal.Value.(cell.LotusError); ok {
				typeStr = "ERROR"
				val = errVal.Code
			}
			var appliedFmt string
			if cellVal.FormatSpec != nil && cellVal.FormatSpec.Type != cell.FmtGeneral {
				appliedFmt = cellVal.FormatSpec.String()
			}
			affected = append(affected, CellItem{
				Ref: ref,
				R:   r,
				C:   c,
				Typ: typeStr,
				Raw: cellVal.RawInput,
				Val: val,
				Fmt: appliedFmt,
			})
		}
	}

	saved := false
	if !args.DryRun {
		if err := SaveWorkbookAtomic(wb, sh.Name(), args.File, "", recalc); err != nil {
			return mcpErrorResult(fmt.Sprintf("Failed to save changes: %v", err))
		}
		saved = true
	}

	resp := Response{
		OK:      true,
		Command: "set",
		Meta: MetaInfo{
			File:       filepath.Base(args.File),
			Sheet:      sh.Name(),
			Target:     args.Target,
			Saved:      saved,
			DryRun:     args.DryRun,
			RecalcMode: sh.RecalcMode(),
		},
		Affected: affected,
	}
	jsonBytes, _ := json.MarshalIndent(resp, "", "  ")
	return mcpSuccessResult(string(jsonBytes))
}

func mcpToolBatchEdit(argsRaw json.RawMessage) mcpToolCallResult {
	var args struct {
		File    string        `json:"file"`
		Actions []BatchAction `json:"actions"`
		Sheet   string        `json:"sheet"`
		Recalc  *bool         `json:"recalc"`
		DryRun  bool          `json:"dry_run"`
	}
	if err := json.Unmarshal(argsRaw, &args); err != nil {
		return mcpErrorResult(fmt.Sprintf("Invalid arguments: %v", err))
	}
	if args.File == "" || len(args.Actions) == 0 {
		return mcpErrorResult("Missing required arguments: 'file' and non-empty 'actions' array")
	}

	wb, err := LoadWorkbookAuto(args.File)
	if err != nil {
		return mcpErrorResult(fmt.Sprintf("Failed to load '%s': %v", args.File, err))
	}

	for stepIdx, act := range args.Actions {
		if err := executeBatchAction(wb, act, args.Sheet); err != nil {
			resp := Response{
				OK:             false,
				Command:        "batch",
				Error:          err.Error(),
				Code:           determineErrorCode(err),
				FailedStep:     stepIdx,
				CompletedSteps: stepIdx,
				TotalSteps:     len(args.Actions),
			}
			jsonBytes, _ := json.MarshalIndent(resp, "", "  ")
			return mcpToolCallResult{
				Content: []mcpTextContent{{Type: "text", Text: string(jsonBytes)}},
				IsError: true,
			}
		}
	}

	recalc := true
	if args.Recalc != nil {
		recalc = *args.Recalc
	}
	if recalc {
		wb.RecalculateAll()
	}

	saved := false
	if !args.DryRun {
		if err := SaveWorkbookAtomic(wb, args.Sheet, args.File, "", recalc); err != nil {
			return mcpErrorResult(fmt.Sprintf("Failed to save batch changes: %v", err))
		}
		saved = true
	}

	resp := Response{
		OK:      true,
		Command: "batch",
		Meta: MetaInfo{
			File:           filepath.Base(args.File),
			Saved:          saved,
			DryRun:         args.DryRun,
			ActionsApplied: len(args.Actions),
			RecalcMode:     wb.GetActiveSheet().RecalcMode(),
		},
	}
	jsonBytes, _ := json.MarshalIndent(resp, "", "  ")
	return mcpSuccessResult(string(jsonBytes))
}

func mcpToolRenderChart(argsRaw json.RawMessage) mcpToolCallResult {
	var args struct {
		File    string `json:"file"`
		Output  string `json:"output"`
		Sheet   string `json:"sheet"`
		Type    string `json:"type"`
		Title   string `json:"title"`
		RangeX  string `json:"range_x"`
		SeriesA string `json:"series_a"`
		SeriesB string `json:"series_b"`
		SeriesC string `json:"series_c"`
		Width   int    `json:"width"`
		Height  int    `json:"height"`
	}
	if err := json.Unmarshal(argsRaw, &args); err != nil {
		return mcpErrorResult(fmt.Sprintf("Invalid arguments: %v", err))
	}
	if args.File == "" || args.Output == "" {
		return mcpErrorResult("Missing required arguments: 'file' and 'output'")
	}

	wb, err := LoadWorkbookAuto(args.File)
	if err != nil {
		return mcpErrorResult(fmt.Sprintf("Failed to load '%s': %v", args.File, err))
	}

	sh, err := GetTargetSheet(wb, args.Sheet)
	if err != nil {
		return mcpErrorResult(err.Error())
	}

	sh.Recalculate()
	g := sh.Graph()
	if g.Series == nil {
		g.Series = make(map[string]*coord.RangeRef)
	}

	if args.Type != "" {
		ct := strings.ToUpper(strings.TrimSpace(args.Type))
		switch ct {
		case "LINE", "BAR", "STACKED", "PIE":
			g.Type = ct
		default:
			return mcpErrorResult(fmt.Sprintf("Unsupported chart type '%s'", args.Type))
		}
	} else if g.Type == "" {
		g.Type = "LINE"
	}

	if args.Title != "" {
		g.Title = args.Title
	}

	if args.RangeX != "" {
		rx, err := coord.ParseRangeRef(args.RangeX)
		if err != nil {
			return mcpErrorResult(fmt.Sprintf("Invalid range_x: %v", err))
		}
		g.RangeX = &rx
	}

	if args.SeriesA != "" {
		ref, err := coord.ParseRangeRef(args.SeriesA)
		if err != nil {
			return mcpErrorResult(fmt.Sprintf("Invalid series_a: %v", err))
		}
		g.Series["A"] = &ref
	}
	if args.SeriesB != "" {
		ref, err := coord.ParseRangeRef(args.SeriesB)
		if err != nil {
			return mcpErrorResult(fmt.Sprintf("Invalid series_b: %v", err))
		}
		g.Series["B"] = &ref
	}
	if args.SeriesC != "" {
		ref, err := coord.ParseRangeRef(args.SeriesC)
		if err != nil {
			return mcpErrorResult(fmt.Sprintf("Invalid series_c: %v", err))
		}
		g.Series["C"] = &ref
	}

	w := args.Width
	if w <= 0 {
		w = 1280
	}
	h := args.Height
	if h <= 0 {
		h = 720
	}

	if err := tui.ExportGraphPNG(sh, args.Output, w, h); err != nil {
		return mcpErrorResult(fmt.Sprintf("Failed to export chart: %v", err))
	}

	return mcpSuccessResult(fmt.Sprintf("Chart successfully rendered to '%s' (%dx%d)", args.Output, w, h))
}

func mcpToolConvertFile(argsRaw json.RawMessage) mcpToolCallResult {
	var args struct {
		Input  string `json:"input"`
		Output string `json:"output"`
		Sheet  string `json:"sheet"`
		Recalc bool   `json:"recalc"`
	}
	if err := json.Unmarshal(argsRaw, &args); err != nil {
		return mcpErrorResult(fmt.Sprintf("Invalid arguments: %v", err))
	}
	if args.Input == "" || args.Output == "" {
		return mcpErrorResult("Missing required arguments: 'input' and 'output'")
	}

	wb, err := LoadWorkbookAuto(args.Input)
	if err != nil {
		return mcpErrorResult(fmt.Sprintf("Failed to load '%s': %v", args.Input, err))
	}

	toFmt := DetectFormat(args.Output)
	if toFmt == "" {
		return mcpErrorResult(fmt.Sprintf("Cannot detect output format from '%s'", args.Output))
	}

	if err := SaveWorkbookAuto(wb, args.Sheet, args.Output, toFmt, args.Recalc); err != nil {
		return mcpErrorResult(fmt.Sprintf("Failed to convert to '%s': %v", args.Output, err))
	}

	return mcpSuccessResult(fmt.Sprintf("Successfully converted '%s' to '%s' (%s)", args.Input, args.Output, toFmt))
}

func mcpSuccessResult(text string) mcpToolCallResult {
	return mcpToolCallResult{
		Content: []mcpTextContent{{Type: "text", Text: text}},
		IsError: false,
	}
}

func mcpErrorResult(errMsg string) mcpToolCallResult {
	return mcpToolCallResult{
		Content: []mcpTextContent{{Type: "text", Text: "Error: " + errMsg}},
		IsError: true,
	}
}

func getMCPToolDefinitions() []mcpToolDefinition {
	return []mcpToolDefinition{
		{
			Name:        "read_sheet",
			Description: "Read cells and tabular data from a spreadsheet workbook (.hwk, .xlsx, .ods, .csv, .md) with optional range and format scoping.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"file":   map[string]any{"type": "string", "description": "Path to spreadsheet file"},
					"sheet":  map[string]any{"type": "string", "description": "Target sheet name (defaults to active sheet)"},
					"range":  map[string]any{"type": "string", "description": "Target coordinate or range (e.g. 'B2', 'A1:E10')"},
					"format": map[string]any{"type": "string", "description": "Output format: 'json' (default), 'markdown', 'csv', or 'values'", "enum": []string{"json", "markdown", "csv", "values"}},
					"recalc": map[string]any{"type": "boolean", "description": "Recalculate workbook prior to reading (default: true)"},
				},
				"required": []string{"file"},
			},
		},
		{
			Name:        "get_info",
			Description: "Inspect structural metadata of a spreadsheet workbook, including sheet names, used ranges, cell counts, freeze panes, and chart configurations.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"file":  map[string]any{"type": "string", "description": "Path to spreadsheet file"},
					"sheet": map[string]any{"type": "string", "description": "Inspect specific sheet name"},
				},
				"required": []string{"file"},
			},
		},
		{
			Name:        "evaluate_formula",
			Description: "Evaluate a spreadsheet formula or arithmetic expression immediately, optionally within the context of a workbook file.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"formula": map[string]any{"type": "string", "description": "Formula expression e.g. '=SUM(A1:A10)*1.1'"},
					"file":    map[string]any{"type": "string", "description": "Context workbook file path (optional)"},
					"sheet":   map[string]any{"type": "string", "description": "Context sheet name (optional)"},
				},
				"required": []string{"formula"},
			},
		},
		{
			Name:        "edit_cell",
			Description: "Update a single cell or rectangular range with a value, label, formula, or format, with automatic recalculation and atomic persistence.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"file":    map[string]any{"type": "string", "description": "Path to spreadsheet file"},
					"target":  map[string]any{"type": "string", "description": "Target cell coordinate (e.g. 'B2') or range (e.g. 'B2:B10')"},
					"value":   map[string]any{"type": "string", "description": "Value, string, or formula e.g. '150', '=SUM(B2:B4)'"},
					"sheet":   map[string]any{"type": "string", "description": "Target sheet name"},
					"format":  map[string]any{"type": "string", "description": "Cell format descriptor e.g. '(F2)', '(C2)'"},
					"recalc":  map[string]any{"type": "boolean", "description": "Recalculate workbook after mutation (default: true)"},
					"dry_run": map[string]any{"type": "boolean", "description": "Preview mutation in memory without saving to disk"},
				},
				"required": []string{"file", "target", "value"},
			},
		},
		{
			Name:        "batch_edit",
			Description: "Execute an atomic sequence of spreadsheet mutations (set_cell, set_range, clear, format, insert/delete rows/columns, add/rename/delete sheets) with transactional rollback guarantee.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"file":    map[string]any{"type": "string", "description": "Path to spreadsheet file"},
					"actions": map[string]any{"type": "array", "description": "Ordered list of batch action objects", "items": map[string]any{"type": "object"}},
					"sheet":   map[string]any{"type": "string", "description": "Default sheet name for actions omitting sheet"},
					"recalc":  map[string]any{"type": "boolean", "description": "Recalculate workbook after batch execution (default: true)"},
					"dry_run": map[string]any{"type": "boolean", "description": "Execute batch in memory without saving to disk"},
				},
				"required": []string{"file", "actions"},
			},
		},
		{
			Name:        "render_chart",
			Description: "Render an HD 1280x720 PNG chart from worksheet data without opening a GUI window.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"file":     map[string]any{"type": "string", "description": "Path to spreadsheet file"},
					"output":   map[string]any{"type": "string", "description": "Output PNG file path"},
					"sheet":    map[string]any{"type": "string", "description": "Target sheet name"},
					"type":     map[string]any{"type": "string", "description": "Chart type", "enum": []string{"LINE", "BAR", "STACKED", "PIE"}},
					"title":    map[string]any{"type": "string", "description": "Chart title"},
					"range_x":  map[string]any{"type": "string", "description": "X-axis category range e.g. 'A2:A10'"},
					"series_a": map[string]any{"type": "string", "description": "Series A data range e.g. 'B2:B10'"},
					"series_b": map[string]any{"type": "string", "description": "Series B data range"},
					"series_c": map[string]any{"type": "string", "description": "Series C data range"},
					"width":    map[string]any{"type": "integer", "description": "Image width in pixels (default: 1280)"},
					"height":   map[string]any{"type": "integer", "description": "Image height in pixels (default: 720)"},
				},
				"required": []string{"file", "output"},
			},
		},
		{
			Name:        "convert_file",
			Description: "Convert tabular data between supported formats (.xlsx, .ods, .csv, .tsv, .md, .html, .hwk, .hwkz).",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"input":  map[string]any{"type": "string", "description": "Input file path"},
					"output": map[string]any{"type": "string", "description": "Output destination path"},
					"sheet":  map[string]any{"type": "string", "description": "Target sheet name (for single-sheet formats)"},
					"recalc": map[string]any{"type": "boolean", "description": "Force full workbook recalculation before exporting"},
				},
				"required": []string{"input", "output"},
			},
		},
	}
}

// PrintMCPHelp displays usage instructions for the mcp command.
func PrintMCPHelp() {
	msg := `Usage:
  hasucalc mcp [flags]

Launches the Model Context Protocol (MCP) server over standard input/output (stdio).
Compatible with Claude Desktop, Cursor, Gemini CLI, Antigravity, and other MCP clients.

Exposed MCP Tools:
  - read_sheet: Extract tabular cell data (JSON, Markdown, CSV, values)
  - get_info: Inspect workbook and sheet structural metadata
  - evaluate_formula: Compute formula standalone or in workbook context
  - edit_cell: Update cell value/formula/format with atomic persistence
  - batch_edit: Execute transactional sequence of actions with rollback
  - render_chart: Render 1280x720 HD PNG chart from sheet data
  - convert_file: Convert files between supported formats
`
	fmt.Print(msg)
}
