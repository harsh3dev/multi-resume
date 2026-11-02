package parser

import (
	"strings"

	"github.com/harsh3dev/multi-resume/backend/internal/fonts"
	"github.com/harsh3dev/multi-resume/backend/internal/latex"
	"github.com/harsh3dev/multi-resume/backend/internal/model"
)

// findStyles exposes only values backed by an actual preamble span.
func findStyles(src string, pre model.Span) []model.StyleSetting {
	styles := []model.StyleSetting{}
	tokens := latex.Tokenize(src[pre[0]:pre[1]])
	for i := range tokens {
		tokens[i].Start += pre[0]
		tokens[i].End += pre[0]
	}
	add := func(key string, span model.Span) {
		if span[0] < pre[0] || span[1] > pre[1] || span[0] >= span[1] {
			return
		}
		value := strings.TrimSpace(src[span[0]:span[1]])
		if value == "" {
			return
		}
		styles = append(styles, model.StyleSetting{ID: newID("st"), Key: key, Value: value, Span: &span})
	}
	for i, t := range tokens {
		if t.Kind != latex.TokCommand {
			continue
		}
		if t.Start >= pre[1] {
			continue
		}
		switch t.Raw {
		case `\documentclass`:
			options, _, ok := group(tokens, i+1, src, latex.TokOpenBracket, latex.TokCloseBracket)
			if !ok {
				continue
			}
			for _, part := range splitOptions(src, options) {
				value := src[part[0]:part[1]]
				if strings.HasSuffix(value, "pt") {
					add("fontSize", part)
				}
				if value == "letterpaper" || value == "a4paper" || value == "legalpaper" {
					add("pageSize", part)
				}
			}
		case `\geometry`:
			span, _, ok := group(tokens, i+1, src, latex.TokOpenBrace, latex.TokCloseBrace)
			if ok {
				for _, part := range splitOptions(src, span) {
					option := src[part[0]:part[1]]
					if strings.HasPrefix(option, "margin=") {
						add("margin", model.Span{part[0] + len("margin="), part[1]})
					}
				}
			}
		case `\setmainfont`:
			span, _, ok := group(tokens, i+1, src, latex.TokOpenBrace, latex.TokCloseBrace)
			if ok {
				add("fontFamily", span)
			}
		case `\usepackage`:
			j := i + 1
			if _, next, ok := group(tokens, j, src, latex.TokOpenBracket, latex.TokCloseBracket); ok {
				j = next
			}
			span, _, ok := group(tokens, j, src, latex.TokOpenBrace, latex.TokCloseBrace)
			if ok {
				if fonts.IsPackage(src[span[0]:span[1]]) {
					add("fontFamily", span)
				}
			}
		case `\titlespacing`:
			j := i + 1
			_, j, ok := group(tokens, j, src, latex.TokOpenBrace, latex.TokCloseBrace)
			if !ok {
				continue
			}
			_, j, ok = group(tokens, j, src, latex.TokOpenBrace, latex.TokCloseBrace)
			if !ok {
				continue
			}
			before, _, ok := group(tokens, j, src, latex.TokOpenBrace, latex.TokCloseBrace)
			if ok {
				add("sectionSpacing", before)
			}
		case `\titleformat`:
			_, j, ok := group(tokens, i+1, src, latex.TokOpenBrace, latex.TokCloseBrace)
			if !ok {
				continue
			}
			format, _, ok := group(tokens, j, src, latex.TokOpenBrace, latex.TokCloseBrace)
			if ok {
				add("headingFormat", format)
			}
		case `\definecolor`:
			name, next, ok := group(tokens, i+1, src, latex.TokOpenBrace, latex.TokCloseBrace)
			if !ok {
				continue
			}
			_, next, ok = group(tokens, next, src, latex.TokOpenBrace, latex.TokCloseBrace)
			if !ok {
				continue
			}
			value, _, ok := group(tokens, next, src, latex.TokOpenBrace, latex.TokCloseBrace)
			if ok {
				add("color:"+src[name[0]:name[1]], value)
			}
		case `\setlength`:
			name, next, ok := group(tokens, i+1, src, latex.TokOpenBrace, latex.TokCloseBrace)
			if !ok || strings.TrimSpace(src[name[0]:name[1]]) != `\itemsep` {
				continue
			}
			value, _, ok := group(tokens, next, src, latex.TokOpenBrace, latex.TokCloseBrace)
			if ok {
				add("bulletSpacing", value)
			}
		}
	}
	return styles
}

func splitOptions(src string, span model.Span) []model.Span {
	parts := []model.Span{}
	start := span[0]
	for i := start; i <= span[1]; i++ {
		if i == span[1] || src[i] == ',' {
			begin, end := start, i
			for begin < end && src[begin] == ' ' {
				begin++
			}
			for end > begin && src[end-1] == ' ' {
				end--
			}
			if begin < end {
				parts = append(parts, model.Span{begin, end})
			}
			start = i + 1
		}
	}
	return parts
}

func findAssets(src string) []model.AssetRef {
	assets := []model.AssetRef{}
	tokens := latex.Tokenize(src)
	for i, t := range tokens {
		// \photo is moderncv's picture command: \photo[height][frame]{file}.
		if t.Kind != latex.TokCommand || (t.Raw != `\includegraphics` && t.Raw != `\photo`) {
			continue
		}
		j := i + 1
		if _, next, ok := group(tokens, j, src, latex.TokOpenBracket, latex.TokCloseBracket); ok {
			j = next
		}
		path, _, ok := group(tokens, j, src, latex.TokOpenBrace, latex.TokCloseBrace)
		if !ok {
			continue
		}
		value := strings.TrimSpace(src[path[0]:path[1]])
		if value == "" {
			continue
		}
		assets = append(assets, model.AssetRef{ID: newID("a"), Path: value, Span: path, Uploaded: false})
	}
	return assets
}
