package handler

import (
	"math"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// CountTokensCompat returns a local Anthropic-compatible token estimate for
// platforms that do not expose a native /v1/messages/count_tokens endpoint.
func (h *GatewayHandler) CountTokensCompat(c *gin.Context) {
	body, err := c.GetRawData()
	if err != nil {
		h.errorResponse(c, 400, "invalid_request_error", "Failed to read request body")
		return
	}
	if len(body) == 0 {
		h.errorResponse(c, 400, "invalid_request_error", "Request body is empty")
		return
	}

	c.JSON(200, gin.H{
		"input_tokens": estimateAnthropicCountTokens(body),
	})
}

func estimateAnthropicCountTokens(reqBody []byte) int {
	total := 0

	addText := func(text string) {
		total += estimateCompatTokensForText(text)
	}

	system := gjson.GetBytes(reqBody, "system")
	switch {
	case system.IsArray():
		system.ForEach(func(_, item gjson.Result) bool {
			addText(item.Get("text").String())
			return true
		})
	case system.Type == gjson.String:
		addText(system.String())
	}

	gjson.GetBytes(reqBody, "messages").ForEach(func(_, message gjson.Result) bool {
		content := message.Get("content")
		switch {
		case content.IsArray():
			content.ForEach(func(_, block gjson.Result) bool {
				addText(block.Get("text").String())
				addText(block.Get("name").String())
				addText(block.Get("id").String())
				addText(block.Get("type").String())
				return true
			})
		case content.Type == gjson.String:
			addText(content.String())
		}
		return true
	})

	gjson.GetBytes(reqBody, "tools").ForEach(func(_, tool gjson.Result) bool {
		addText(tool.Get("name").String())
		addText(tool.Get("description").String())
		addText(tool.Get("custom.description").String())
		addText(tool.Get("input_schema").Raw)
		addText(tool.Get("custom.input_schema").Raw)
		return true
	})

	if total <= 0 {
		return 1
	}
	return total
}

func estimateCompatTokensForText(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	runes := []rune(s)
	if len(runes) == 0 {
		return 0
	}

	ascii := 0
	for _, r := range runes {
		if r <= 0x7f {
			ascii++
		}
	}

	asciiRatio := float64(ascii) / float64(len(runes))
	if asciiRatio >= 0.8 {
		return int(math.Ceil(float64(len(runes)) / 4.0))
	}
	return len(runes)
}
